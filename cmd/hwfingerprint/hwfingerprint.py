#!/usr/bin/env python3
"""
Tridiagonal Timing Anomaly Detector.

Monitors cryptographic operation timing and detects anomalies using
the tridiagonal eigenvalue spread test. Based on the theorem that
Hilbert plane Gram matrices are exactly tridiagonal.

Normal operations have a characteristic Z-score baseline. Deviations
from baseline indicate hardware anomalies (hypervisor, side-channel,
thermal throttling, malware).

Usage:
    python3 hwfingerprint.py baseline    # establish baseline Z-score
    python3 hwfingerprint.py monitor     # monitor for anomalies
    python3 hwfingerprint.py detect      # one-shot anomaly check
"""

import math, time, os, hashlib, json, sys, argparse
from datetime import datetime


def tridiagonal_zscore(timings, dim=64):
    """Compute Z-score from timing measurements via tridiagonal analysis."""
    N = dim
    counts = [0] * N
    for t in timings:
        z_bin = abs(int(t)) % N
        counts[z_bin] += 1

    n = len(timings)
    expected = n / N
    diag = [(c - expected) / math.sqrt(max(expected, 1e-10)) for c in counts]

    alpha = 1.0 / N
    ev = []
    for k in range(1, N + 1):
        ev.append(diag[k - 1] + 2 * alpha * math.cos(math.pi * k / (N + 1)))
    ev.sort()

    mean_ev = sum(ev) / N
    spread = math.sqrt(sum((x - mean_ev) ** 2 for x in ev) / N)
    return spread / math.sqrt(2.0 / N)


def collect_sha256_timings(iterations=5000):
    """SHA-256 hash timing collection — deterministic, structured."""
    times = []
    data = os.urandom(64)
    for _ in range(iterations):
        t0 = time.perf_counter_ns()
        hashlib.sha256(data).digest()
        times.append(time.perf_counter_ns() - t0)
        data = hashlib.sha256(data).digest()
    return times


def collect_crypto_ops(iterations=3000):
    """Mixed crypto operations — more realistic workload."""
    times = []
    data = os.urandom(128)
    for _ in range(iterations):
        t0 = time.perf_counter_ns()
        h = hashlib.sha256(data).digest()
        h = hashlib.sha512(h).digest()
        times.append(time.perf_counter_ns() - t0)
        data = h
    return times


def baseline(tag="default", iterations=5000):
    """Establish baseline Z-score for this machine."""
    print(f"Establishing baseline '{tag}' ({iterations} samples)...")
    times = collect_sha256_timings(iterations)
    z = tridiagonal_zscore(times)

    baseline_data = {
        'tag': tag,
        'timestamp': datetime.now().isoformat(),
        'iterations': iterations,
        'z_score': round(z, 4),
        'mean_ns': round(sum(times) / len(times), 2),
        'std_ns': round(math.sqrt(sum((t - sum(times)/len(times))**2
                                      for t in times) / len(times)), 2),
    }

    os.makedirs('/tmp/hwfp', exist_ok=True)
    with open(f'/tmp/hwfp/baseline_{tag}.json', 'w') as f:
        json.dump(baseline_data, f, indent=2)

    print(f"  Z-score: {z:.2f}")
    print(f"  Mean: {baseline_data['mean_ns']:.0f}ns ± {baseline_data['std_ns']:.0f}ns")
    print(f"  Saved to /tmp/hwfp/baseline_{tag}.json")
    return baseline_data


def detect(baseline_tag="default", iterations=3000, threshold=2.0):
    """One-shot anomaly detection against baseline."""
    try:
        with open(f'/tmp/hwfp/baseline_{baseline_tag}.json') as f:
            base = json.load(f)
    except FileNotFoundError:
        print(f"No baseline '{baseline_tag}' found. Run 'baseline' first.")
        sys.exit(1)

    times = collect_sha256_timings(iterations)
    current_z = tridiagonal_zscore(times)

    base_z = base['z_score']
    delta = abs(current_z - base_z)
    ratio = current_z / max(base_z, 1e-10)
    anomaly = delta > base_z * threshold or ratio < 0.5 or ratio > 2.0

    if anomaly:
        direction = "HIGH" if current_z > base_z else "LOW"
        print(f"⚠ ANOMALY DETECTED")
    else:
        direction = "—"

    print(f"  Baseline Z: {base_z:.2f}  →  Current Z: {current_z:.2f}")
    print(f"  Delta: {delta:.2f}  Ratio: {ratio:.2f}  Status: {direction}")
    return not anomaly


def monitor(baseline_tag="default", interval=60, iterations=3000, threshold=2.0):
    """Continuous monitoring — runs until interrupted."""
    try:
        with open(f'/tmp/hwfp/baseline_{baseline_tag}.json') as f:
            base = json.load(f)
    except FileNotFoundError:
        print(f"No baseline '{baseline_tag}' found. Run 'baseline' first.")
        sys.exit(1)

    base_z = base['z_score']
    print(f"Monitoring (baseline Z={base_z:.2f}, interval={interval}s, Ctrl+C to stop)")

    try:
        while True:
            times = collect_sha256_timings(iterations)
            current_z = tridiagonal_zscore(times)
            delta = abs(current_z - base_z)
            ratio = current_z / max(base_z, 1e-10)
            anomaly = delta > base_z * threshold or ratio < 0.5 or ratio > 2.0

            ts = datetime.now().strftime('%H:%M:%S')
            flag = '⚠ ANOMALY' if anomaly else '  normal'
            print(f"[{ts}] Z={current_z:.2f} Δ={delta:+.2f} ratio={ratio:.2f} {flag}")

            time.sleep(interval)
    except KeyboardInterrupt:
        print("\nMonitoring stopped.")


def main():
    parser = argparse.ArgumentParser(
        description='Tridiagonal Timing Anomaly Detector')
    parser.add_argument('command', nargs='?', default='detect',
                        choices=['baseline', 'detect', 'monitor'],
                        help='Command to run')
    parser.add_argument('--tag', default='default',
                        help='Baseline tag name')
    parser.add_argument('--iterations', type=int, default=5000,
                        help='Timing samples per check')
    parser.add_argument('--interval', type=int, default=60,
                        help='Monitoring interval in seconds')
    parser.add_argument('--threshold', type=float, default=2.0,
                        help='Z-score change threshold for anomaly')
    args = parser.parse_args()

    if args.command == 'baseline':
        baseline(args.tag, args.iterations)
    elif args.command == 'detect':
        detect(args.tag, args.iterations, args.threshold)
    elif args.command == 'monitor':
        monitor(args.tag, args.interval, args.iterations, args.threshold)


if __name__ == '__main__':
    main()
