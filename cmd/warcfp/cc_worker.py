#!/usr/bin/env python3
"""
Common Crawl cluster worker — downloads WARC files, fingerprints pages,
stores results in ClickHouse. Coordinates via ClickHouse to avoid duplicate
work across a cluster.

Usage:
  cc_worker.py seed                            Populate WARC file list from index
  cc_worker.py work [--host HOSTNAME]          Claim and process one file
  cc_worker.py loop [--host HOSTNAME]          Process files continuously
  cc_worker.py status                          Show cluster progress
"""

import sys
import os
import gzip
import json
import time
import hashlib
import socket
import struct
import subprocess
import tempfile
import traceback
import urllib.request
import urllib.error
from datetime import datetime
from io import BytesIO

CLICKHOUSE = "http://si.lithop:8123"
CRAWL = "CC-MAIN-2026-25"
CRAWL_URL = f"https://data.commoncrawl.org/crawl-data/{CRAWL}/warc.paths.gz"
WARCFP_BIN = os.path.join(os.path.dirname(os.path.abspath(__file__)), "warcfp_fast")

def ch_query(query):
    """Execute ClickHouse query via HTTP, return list of dicts."""
    if "FORMAT" not in query.upper():
        query = query.rstrip().rstrip(";").rstrip() + " FORMAT JSONEachRow"
    data = query.encode()
    req = urllib.request.Request(f"{CLICKHOUSE}/", data=data, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return [json.loads(line) for line in resp if line.strip()]
    except urllib.error.HTTPError as e:
        body = e.read().decode()[:500]
        print(f"ClickHouse error: {e.code} — {body}", file=sys.stderr)
        return []

def ch_exec(query):
    """Execute ClickHouse statement (no result expected)."""
    data = query.encode() if isinstance(query, str) else query
    req = urllib.request.Request(f"{CLICKHOUSE}/", data=data, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.read().decode()
    except urllib.error.HTTPError as e:
        body = e.read().decode()[:500]
        print(f"ClickHouse error: {e.code} — {body}", file=sys.stderr)
        return None

def seed_warc_list():
    """Download the WARC file list from Common Crawl and populate cc_warc_files."""
    print(f"Downloading WARC file list for {CRAWL}...")
    resp = urllib.request.urlopen(CRAWL_URL, timeout=120)
    raw = resp.read()

    # Decompress
    with gzip.GzipFile(fileobj=BytesIO(raw)) as gz:
        paths = gz.read().decode().splitlines()

    print(f"Found {len(paths)} WARC files. Inserting into ClickHouse...")

    # Batch insert
    batch = []
    for path in paths:
        path = path.strip()
        if not path:
            continue
        parts = path.split("/")
        # path format: crawl-data/CRAWL/segments/SEGMENT/warc/FILE.warc.gz
        segment = parts[3] if len(parts) > 3 else ""
        batch.append({
            "warc_path": path,
            "crawl_id": CRAWL,
            "segment": segment,
        })

    # Insert in chunks of 1000
    for i in range(0, len(batch), 1000):
        chunk = batch[i:i+1000]
        rows = "\n".join(json.dumps(r) for r in chunk)
        ch_exec(f"INSERT INTO commoncrawl.cc_warc_files (warc_path, crawl_id, segment) FORMAT JSONEachRow\n{rows}")
        print(f"  Inserted {i+len(chunk)}/{len(batch)}")

    print("Done.")

def claim_file(hostname):
    """Atomically claim an unprocessed WARC file. Returns path or None."""
    # Try to claim a 'new' file by updating it to 'processing'
    result = ch_query(f"""
        SELECT warc_path, segment
        FROM commoncrawl.cc_warc_files
        WHERE status = 'new' AND crawl_id = '{CRAWL}'
        ORDER BY warc_path
        LIMIT 1
    """)
    if not result:
        return None

    path = result[0]["warc_path"]
    segment = result[0]["segment"]

    # Claim via synchronous INSERT. ClickHouse INSERTs are atomic
    # and immediately visible. The ReplacingMergeTree deduplicates
    # by warc_path, keeping the latest insert.
    ch_exec(f"""
        INSERT INTO commoncrawl.cc_warc_files
            (warc_path, crawl_id, segment, status, claimed_by, claimed_at)
        VALUES ('{path}', '{CRAWL}', '{segment}', 'processing',
                '{hostname}', now())
    """)
    print(f"Claimed: {path}")
    return path

def process_warc(warc_path, hostname):
    """Download, decompress, fingerprint, and store a WARC file."""
    url = f"https://data.commoncrawl.org/{warc_path}"
    segment = warc_path.split("/")[3] if len(warc_path.split("/")) > 3 else ""
    print(f"Downloading: {url}")

    start_time = time.time()

    try:
        # Stream download with decompression
        resp = urllib.request.urlopen(url, timeout=600)
        raw = resp.read()
        download_time = time.time() - start_time
        print(f"  Downloaded {len(raw):,} bytes compressed in {download_time:.1f}s")

        # Decompress
        decompress_start = time.time()
        decompressed = gzip.decompress(raw)
        decompress_time = time.time() - decompress_start
        print(f"  Decompressed to {len(decompressed):,} bytes in {decompress_time:.1f}s")

    except Exception as e:
        ch_exec(f"""
            INSERT INTO commoncrawl.cc_warc_files
                (warc_path, crawl_id, segment, status, error_message)
            VALUES ('{warc_path}', '{CRAWL}', '{segment}', 'failed',
                    '{str(e)[:200]}')
        """)
        print(f"  Download failed: {e}")
        return

    # Write decompressed WARC to temp file and run warcfp_fast
    fp_start = time.time()
    with tempfile.NamedTemporaryFile(suffix=".warc", delete=False) as tmp:
        tmp.write(decompressed)
        tmp_path = tmp.name

    try:
        result = subprocess.run(
            [WARCFP_BIN, tmp_path],
            capture_output=True, text=True, timeout=600
        )
        fp_time = time.time() - fp_start

        if result.returncode != 0:
            raise Exception(f"warcfp_fast failed: {result.stderr[:200]}")

        lines = result.stdout.strip().splitlines()
        print(f"  Fingerprinted {len(lines)} pages in {fp_time:.1f}s")

    except Exception as e:
        ch_exec(f"""
            INSERT INTO commoncrawl.cc_warc_files
                (warc_path, crawl_id, segment, status, error_message)
            VALUES ('{warc_path}', '{CRAWL}', '{segment}', 'failed',
                    '{str(e)[:200]}')
        """)
        print(f"  Fingerprinting failed: {e}")
        os.unlink(tmp_path)
        return
    finally:
        os.unlink(tmp_path)

    # Parse output and insert into ClickHouse
    insert_start = time.time()
    batch = []
    for line in lines:
        line = line.strip()
        if not line or "  " not in line:
            continue
        fp_hex, url = line.split("  ", 1)
        if len(fp_hex) != 32:
            continue
        try:
            fp_bytes = bytes.fromhex(fp_hex)
        except ValueError:
            continue
        batch.append({
            "fingerprint": fp_hex,
            "url": url[:2048],
            "warc_file": warc_path,
            "fetch_date": datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S"),
            "content_length": 0,
            "body_length": 0,
            "content_type": "",
            "server_response": 0,
        })

    if batch:
        # Insert in chunks
        for i in range(0, len(batch), 500):
            chunk = batch[i:i+500]
            rows = "\n".join(json.dumps(r) for r in chunk)
            ch_exec(f"INSERT INTO commoncrawl.cc_fingerprints FORMAT JSONEachRow\n{rows}")

        insert_time = time.time() - insert_start
        print(f"  Inserted {len(batch)} fingerprints in {insert_time:.1f}s")

    total_time = time.time() - start_time

    # Mark as done
    ch_exec(f"""
        INSERT INTO commoncrawl.cc_warc_files
            (warc_path, crawl_id, segment, status, completed_at,
             record_count, fingerprint_count, bytes_downloaded)
        VALUES ('{warc_path}', '{CRAWL}', '{segment}', 'done', now(),
                {len(lines)}, {len(batch)}, {len(raw)})
    """)

    print(f"  Completed in {total_time:.1f}s ({len(batch)} fingerprints)")

def show_status():
    """Display cluster progress."""
    stats = ch_query("""
        SELECT
            status,
            count() as count,
            sum(bytes_downloaded) as total_bytes,
            sum(fingerprint_count) as total_fps
        FROM commoncrawl.cc_warc_files FINAL
        WHERE crawl_id = 'CC-MAIN-2026-25'
        GROUP BY status
        ORDER BY status
    """)

    print(f"\nCluster status for {CRAWL}:")
    print(f"{'Status':<15} {'Files':<8} {'Downloaded':<15} {'Fingerprints':<15}")
    print("-" * 55)
    total_bytes = 0
    total_fps = 0
    total_files = 0
    for s in stats:
        st = s["status"]
        count = s["count"]
        tbytes = s["total_bytes"]
        tfps = s["total_fps"]
        total_files += count
        total_bytes += tbytes
        total_fps += tfps
        print(f"{st:<15} {count:<8} {tbytes:>13,}  {tfps:>13,}")
    print("-" * 55)
    print(f"{'TOTAL':<15} {total_files:<8} {total_bytes:>13,}  {total_fps:>13,}")

    # Also show recent activity
    recent = ch_query("""
        SELECT warc_path, status, claimed_by, claimed_at
        FROM commoncrawl.cc_warc_files FINAL
        WHERE crawl_id = 'CC-MAIN-2026-25' AND status != 'new'
        ORDER BY claimed_at DESC
        LIMIT 5
    """)
    if recent:
        print(f"\nRecent activity:")
        for r in recent:
            print(f"  {r['status']:<10} {r['claimed_by']:<12} {r['warc_path'][-60:]}")

    # Fingerprint stats
    fp_stats = ch_query("""
        SELECT count() as total, uniq(fingerprint) as unique_fps
        FROM commoncrawl.cc_fingerprints
    """)
    if fp_stats:
        s = fp_stats[0]
        print(f"\nFingerprint table: {s['total']:,} rows, {s['unique_fps']:,} unique")

def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]
    hostname = None
    for i, arg in enumerate(sys.argv[2:], start=2):
        if arg == "--host" and i + 1 < len(sys.argv):
            hostname = sys.argv[i + 1]

    if not hostname:
        hostname = socket.gethostname()

    if cmd == "seed":
        seed_warc_list()
    elif cmd == "work":
        path = claim_file(hostname)
        if path:
            process_warc(path, hostname)
        else:
            print("No unprocessed files available.")
    elif cmd == "loop":
        # Verify binary exists before starting loop
        if not os.path.exists(WARCFP_BIN):
            print(f"ERROR: {WARCFP_BIN} not found. Build with: make", file=sys.stderr)
            sys.exit(1)
        print(f"Worker {hostname} starting loop. Binary: {WARCFP_BIN}")

        # Reclaim stuck files (processing for > 4 hours)
        ch_exec(f"""
            INSERT INTO commoncrawl.cc_warc_files (warc_path, crawl_id, segment, status)
            SELECT warc_path, crawl_id, segment, 'new'
            FROM commoncrawl.cc_warc_files FINAL
            WHERE status = 'processing'
              AND crawl_id = '{CRAWL}'
              AND claimed_at < now() - INTERVAL 4 HOUR
        """)

        while True:
            try:
                path = claim_file(hostname)
                if path:
                    process_warc(path, hostname)
                else:
                    print(f"[{datetime.now().strftime('%H:%M:%S')}] No unprocessed files. Waiting 60s...")
                    time.sleep(60)
            except KeyboardInterrupt:
                print("\nShutting down...")
                break
            except Exception as e:
                print(f"[{datetime.now().strftime('%H:%M:%S')}] Error: {e}", file=sys.stderr)
                import traceback
                traceback.print_exc()
                print(f"Waiting 30s before retry...")
                time.sleep(30)
    elif cmd == "status":
        show_status()
    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)

if __name__ == "__main__":
    main()
