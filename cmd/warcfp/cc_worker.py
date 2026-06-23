#!/usr/bin/env python3
"""
Common Crawl cluster worker — downloads WARC files, fingerprints pages,
stores results in ClickHouse. Coordinates via ClickHouse to avoid duplicate
work across a cluster.

Usage:
  cc_worker.py seed                            Populate WARC file list from index
  cc_worker.py work [--host HOSTNAME]          Claim and process one file
  cc_worker.py loop [-j N] [--host HOSTNAME]   Process files continuously
  cc_worker.py status                          Show cluster progress
"""

import sys
import os
import gzip
import json
import time
import socket
import struct
import subprocess
import tempfile
import traceback
import urllib.request
import urllib.error
from datetime import datetime
from io import BytesIO
from concurrent.futures import ThreadPoolExecutor, as_completed

CLICKHOUSE = "http://si.lithop:8123"
CRAWL = "CC-MAIN-2026-25"
CRAWL_URL = f"https://data.commoncrawl.org/crawl-data/{CRAWL}/warc.paths.gz"
WARCFP_BIN = os.path.join(os.path.dirname(os.path.abspath(__file__)), "warcfp_fast")

# ─── ClickHouse helpers ─────────────────────────────────────────────

def ch_query(query):
    if "FORMAT" not in query.upper():
        query = query.rstrip().rstrip(";").rstrip() + " FORMAT JSONEachRow"
    data = query.encode()
    req = urllib.request.Request(f"{CLICKHOUSE}/", data=data, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return [json.loads(line) for line in resp if line.strip()]
    except urllib.error.HTTPError as e:
        print(f"  [CH] error {e.code}", file=sys.stderr, flush=True)
        return []

def ch_exec(query):
    data = query.encode() if isinstance(query, str) else query
    req = urllib.request.Request(f"{CLICKHOUSE}/", data=data, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.read().decode()
    except urllib.error.HTTPError as e:
        print(f"  [CH] error {e.code}", file=sys.stderr, flush=True)
        return None

# ─── WARC file list seeding ──────────────────────────────────────────

def seed_warc_list():
    print(f"Downloading WARC file list for {CRAWL}...")
    resp = urllib.request.urlopen(CRAWL_URL, timeout=120)
    raw = resp.read()
    with gzip.GzipFile(fileobj=BytesIO(raw)) as gz:
        paths = gz.read().decode().splitlines()
    print(f"Found {len(paths)} WARC files. Inserting...")
    batch = []
    for path in paths:
        path = path.strip()
        if not path: continue
        parts = path.split("/")
        segment = parts[3] if len(parts) > 3 else ""
        batch.append({"warc_path": path, "crawl_id": CRAWL, "segment": segment})
    for i in range(0, len(batch), 1000):
        chunk = batch[i:i+1000]
        rows = "\n".join(json.dumps(r) for r in chunk)
        ch_exec(f"INSERT INTO commoncrawl.cc_warc_files (warc_path, crawl_id, segment) FORMAT JSONEachRow\n{rows}")
        print(f"  {i+len(chunk)}/{len(batch)}")
    print("Done.")

# ─── Cluster claiming ────────────────────────────────────────────────

def claim_file(hostname):
    result = ch_query(f"""
        SELECT warc_path, segment
        FROM commoncrawl.cc_warc_files FINAL
        WHERE status = 'new' AND crawl_id = '{CRAWL}'
        ORDER BY warc_path LIMIT 1
    """)
    if not result: return None
    path = result[0]["warc_path"]
    segment = result[0]["segment"]
    ch_exec(f"""
        INSERT INTO commoncrawl.cc_warc_files
            (warc_path, crawl_id, segment, status, claimed_by, claimed_at)
        VALUES ('{path}', '{CRAWL}', '{segment}', 'processing',
                '{hostname}', now())
    """)
    print(f"  Claimed: {path}", flush=True)
    return path

# ─── Process one WARC file ───────────────────────────────────────────

def process_warc(warc_path, hostname):
    url = f"https://data.commoncrawl.org/{warc_path}"
    segment = warc_path.split("/")[3] if len(warc_path.split("/")) > 3 else ""
    t0 = time.time()

    # Stream download with on-the-fly decompression
    # Cap at 2 GB decompressed (warcfp_fast parser limit)
    MAX_DECOMPRESSED = 2_000_000_000
    tmp_path = None
    try:
        resp = urllib.request.urlopen(url, timeout=600)
        raw_len = 0

        with tempfile.NamedTemporaryFile(suffix=".warc", delete=False) as tmp:
            tmp_path = tmp.name
            dcomp = gzip.GzipFile(fileobj=resp, mode="rb")
            while raw_len < MAX_DECOMPRESSED:
                chunk = dcomp.read(1 << 20)
                if not chunk: break
                tmp.write(chunk)
                raw_len += len(chunk)

        t1 = time.time()
        truncated = "(truncated) " if raw_len >= MAX_DECOMPRESSED else ""
        print(f"  Downloaded+decompressed {truncated}{raw_len:,} bytes in {t1-t0:.1f}s ({raw_len/(t1-t0)/1e6:.0f} MB/s)", flush=True)

    except Exception as e:
        ch_exec(f"""
            INSERT INTO commoncrawl.cc_warc_files
                (warc_path, crawl_id, segment, status, error_message)
            VALUES ('{warc_path}', '{CRAWL}', '{segment}', 'failed', '{str(e)[:200]}')
        """)
        print(f"  Download failed: {e}", flush=True)
        return

    # Fingerprint via GPU
    try:
        result = subprocess.run(
            [WARCFP_BIN, tmp_path],
            capture_output=True, text=True, timeout=600
        )
        t2 = time.time()

        if result.returncode != 0:
            raise Exception(f"warcfp_fast failed: {result.stderr[:200]}")

        lines = result.stdout.strip().splitlines()
        # Filter valid lines
        valid = []
        for line in lines:
            line = line.strip()
            if not line or "  " not in line: continue
            fp_hex, url = line.split("  ", 1)
            if len(fp_hex) == 32: valid.append((fp_hex, url[:2048]))

        print(f"  Fingerprinted {len(valid)} pages in {t2-t1:.1f}s", flush=True)

    except Exception as e:
        ch_exec(f"""
            INSERT INTO commoncrawl.cc_warc_files
                (warc_path, crawl_id, segment, status, error_message)
            VALUES ('{warc_path}', '{CRAWL}', '{segment}', 'failed', '{str(e)[:200]}')
        """)
        print(f"  Fingerprinting failed: {e}", flush=True)
        return
    finally:
        if tmp_path: os.unlink(tmp_path)

    # Insert into ClickHouse
    batch = []
    for fp_hex, url in valid:
        batch.append({
            "fingerprint": fp_hex, "url": url,
            "warc_file": warc_path,
            "fetch_date": datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S"),
            "content_length": 0, "body_length": 0,
            "content_type": "", "server_response": 0,
        })

    if batch:
        for i in range(0, len(batch), 500):
            chunk = batch[i:i+500]
            rows = "\n".join(json.dumps(r) for r in chunk)
            ch_exec(f"INSERT INTO commoncrawl.cc_fingerprints FORMAT JSONEachRow\n{rows}")

        t3 = time.time()
        print(f"  Inserted {len(batch)} rows in {t3-t2:.1f}s", flush=True)

    # Mark done (or failed if no fingerprints extracted)
    final_status = 'done' if len(batch) > 0 else 'failed'
    ch_exec(f"""
        INSERT INTO commoncrawl.cc_warc_files
            (warc_path, crawl_id, segment, status, completed_at,
             record_count, fingerprint_count, bytes_downloaded)
        VALUES ('{warc_path}', '{CRAWL}', '{segment}', '{final_status}', now(),
                {len(valid)}, {len(batch)}, {raw_len})
    """)

    total_t = time.time() - t0
    print(f"  Completed {warc_path[-60:]} in {total_t:.1f}s ({len(batch)} fps)", flush=True)

# ─── Parallel worker loop ────────────────────────────────────────────

def worker_loop(hostname, worker_id):
    """Single worker loop — claims and processes files until exhausted."""
    worker_tag = f"{hostname}-w{worker_id}"
    while True:
        try:
            path = claim_file(worker_tag)
            if path:
                process_warc(path, worker_tag)
            else:
                return  # no more files
        except KeyboardInterrupt:
            raise
        except Exception as e:
            print(f"  [{worker_tag}] Error: {e}", file=sys.stderr, flush=True)
            traceback.print_exc()
            time.sleep(30)

def run_loop(hostname, n_workers):
    """Run N parallel workers, restarting when files become available."""
    # Binary check
    if not os.path.exists(WARCFP_BIN):
        print(f"ERROR: {WARCFP_BIN} not found. Build with: make", file=sys.stderr)
        sys.exit(1)

    print(f"Starting {n_workers} workers on {hostname}")

    while True:
        # Reclaim stuck files
        ch_exec(f"""
            INSERT INTO commoncrawl.cc_warc_files (warc_path, crawl_id, segment, status)
            SELECT warc_path, crawl_id, segment, 'new'
            FROM commoncrawl.cc_warc_files FINAL
            WHERE status = 'processing'
              AND crawl_id = '{CRAWL}'
              AND claimed_at < now() - INTERVAL 4 HOUR
        """)

        # Check if work remains
        check = ch_query(f"""
            SELECT count() as n FROM commoncrawl.cc_warc_files FINAL
            WHERE status = 'new' AND crawl_id = '{CRAWL}'
        """)
        remaining = check[0]["n"] if check else 0

        if remaining == 0:
            print(f"[{datetime.now().strftime('%H:%M:%S')}] Queue empty. Checking in 60s...")
            time.sleep(60)
            continue

        print(f"[{datetime.now().strftime('%H:%M:%S')}] {remaining} files remaining. "
              f"Dispatching {min(n_workers, remaining)} workers...")

        with ThreadPoolExecutor(max_workers=n_workers) as pool:
            futures = [pool.submit(worker_loop, hostname, i)
                       for i in range(min(n_workers, remaining))]
            for f in as_completed(futures):
                try:
                    f.result()
                except KeyboardInterrupt:
                    print("\nShutting down...")
                    pool.shutdown(wait=False, cancel_futures=True)
                    return
                except Exception as e:
                    print(f"Worker crashed: {e}", file=sys.stderr)

# ─── Status display ──────────────────────────────────────────────────

def show_status():
    stats = ch_query(f"""
        SELECT status, count() as count,
               sum(bytes_downloaded) as total_bytes,
               sum(fingerprint_count) as total_fps
        FROM commoncrawl.cc_warc_files FINAL
        WHERE crawl_id = '{CRAWL}' GROUP BY status ORDER BY status
    """)
    print(f"\nCluster status for {CRAWL}:")
    print(f"{'Status':<15} {'Files':<8} {'Downloaded':<15} {'Fingerprints':<15}")
    print("-" * 55)
    for s in stats:
        print(f"{s['status']:<15} {s['count']:<8} {s['total_bytes']:>13,}  {s['total_fps']:>13,}")

    recent = ch_query(f"""
        SELECT warc_path, status, claimed_by, claimed_at
        FROM commoncrawl.cc_warc_files FINAL
        WHERE crawl_id = '{CRAWL}' AND status != 'new'
        ORDER BY claimed_at DESC LIMIT 5
    """)
    if recent:
        print(f"\nRecent activity:")
        for r in recent:
            print(f"  {r['status']:<10} {r['claimed_by']:<15} {r['warc_path'][-60:]}")

    fp_stats = ch_query("SELECT count() as total, uniq(fingerprint) as unique_fps FROM commoncrawl.cc_fingerprints")
    if fp_stats:
        s = fp_stats[0]
        print(f"\nFingerprint table: {s['total']:,} rows, {s['unique_fps']:,} unique")

# ─── Main ────────────────────────────────────────────────────────────

def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]
    hostname = socket.gethostname()
    n_workers = 1

    args = sys.argv[2:]
    i = 0
    while i < len(args):
        if args[i] == "--host" and i+1 < len(args):
            hostname = args[i+1]; i += 2
        elif args[i] == "-j" and i+1 < len(args):
            n_workers = int(args[i+1]); i += 2
        else:
            i += 1

    if cmd == "seed":
        seed_warc_list()
    elif cmd == "work":
        path = claim_file(hostname)
        if path:
            process_warc(path, hostname)
        else:
            print("No unprocessed files available.")
    elif cmd == "loop":
        run_loop(hostname, n_workers)
    elif cmd == "status":
        show_status()
    else:
        print(f"Unknown command: {cmd}")
        sys.exit(1)

if __name__ == "__main__":
    main()
