#!/usr/bin/env python3
"""Reconstruct the eigenvalue <-> zeta-zero correlation analysis.

This script was missing from the repository: every paper (paper5, paper6-*,
shape-identification-report) reports Pearson r / MAD / permutation p-values
between covariance-matrix eigenvalues and the first N zeta zeros, but no
code that actually produces those numbers from the cov_*.json artifacts in
pubs/ was checked in. This reconstructs it from scratch, including a real
(seeded, properly randomized) permutation test -- the one permutation test
that existed in main.go (runCorrelationTest) used a fixed non-random shuffle
map and never reset the array between trials, so its p-values were not
meaningful. That bug is not repeated here.

Usage:
    python3 reconstruct_analysis.py cov_4d_spatial_n7.json
    python3 reconstruct_analysis.py --all          # every cov_*.json under pubs/ and pubs/nd/
"""
import argparse
import json
import sys
from pathlib import Path

import numpy as np
from scipy import stats

# First 64 nontrivial zeta zero imaginary parts (LMFDB), reused from
# pubs/pair_correlation_analysis.py so all analyses share one ground truth.
ZETA_ZEROS_64 = np.array([
    14.134725, 21.022040, 25.010857, 30.424876, 32.935062, 37.586178,
    40.918719, 43.327073, 48.005150, 49.773832, 52.970321, 56.446248,
    59.347044, 60.831779, 65.112544, 67.079810, 69.546402, 72.067158,
    75.704691, 77.144840, 79.337375, 82.910380, 84.735493, 87.425275,
    88.809111, 92.491899, 94.651344, 95.870634, 98.831194, 101.317851,
    103.725538, 105.446623, 107.168611, 111.029837, 111.874659, 114.320221,
    116.226680, 118.015959, 121.370125, 122.946829, 124.256819, 127.516684,
    129.577748, 131.087689, 133.497737, 134.756510, 138.116043, 139.736209,
    141.123707, 143.111846, 146.000982, 147.422765, 150.053520, 150.968220,
    153.776651, 156.112607, 157.597592, 158.849988, 161.188964, 163.030709,
    165.537069, 167.184440, 169.094515, 169.911976,
])


def load_covariance(path):
    with open(path) as f:
        d = json.load(f)
    cov = np.array(d["covariance"], dtype=np.float64)
    meta = {k: v for k, v in d.items() if k not in ("covariance", "mu")}
    return cov, meta


def normalize01(v):
    lo, hi = v.min(), v.max()
    if hi == lo:
        return np.zeros_like(v)
    return (v - lo) / (hi - lo)


def permutation_test(eig_sorted, zeros, observed_r, trials, rng):
    """Real permutation test: independently shuffle the eigenvalue vector
    each trial using a proper RNG, compare |r| against the observed value."""
    count_ge = 0
    n = len(eig_sorted)
    idx = np.arange(n)
    for _ in range(trials):
        rng.shuffle(idx)
        r_perm, _ = stats.pearsonr(eig_sorted[idx], zeros)
        if abs(r_perm) >= abs(observed_r):
            count_ge += 1
    return count_ge / trials


def analyze(path, trials, seed):
    cov, meta = load_covariance(path)
    n = cov.shape[0]

    # sanity: how symmetric / tridiagonal is it actually?
    asym = np.abs(cov - cov.T).max()
    off_tridiag = np.abs(cov - np.triu(np.tril(cov, 1), -1)).max()

    eigvals = np.linalg.eigvalsh(cov)  # ascending, cov is real symmetric
    order_desc = np.argsort(-np.abs(eigvals))
    eig_sorted = eigvals[order_desc]

    k = min(64, n)
    # Papers define the comparison on |lambda_i| (magnitude), not the signed
    # eigenvalue -- see paper6-tridiagonal.tex Methods: "Pearson r: linear
    # correlation between |lambda_i| and gamma_i". Using the signed value
    # flips the sign of r for this operator (its dominant eigenvalues are
    # negative), so this must be abs() to match what's reported.
    eig_k = np.abs(eig_sorted[:k])
    zeros_k = ZETA_ZEROS_64[:k]

    r, _ = stats.pearsonr(eig_k, zeros_k)
    rho, _ = stats.spearmanr(eig_k, zeros_k)
    mad = np.mean(np.abs(normalize01(eig_k) - normalize01(zeros_k)))

    rng = np.random.default_rng(seed)
    p_value = permutation_test(eig_k, zeros_k, r, trials, rng)

    return {
        "file": str(path),
        "meta": meta,
        "n": n,
        "k": k,
        "max_asymmetry": asym,
        "max_abs_offtridiagonal": off_tridiag,
        "pearson_r": r,
        "spearman_rho": rho,
        "mad": mad,
        "perm_p_value": p_value,
        "perm_trials": trials,
    }


def fmt(res):
    m = res["meta"]
    label = f"{m.get('hilbert','3D')}/{m.get('method','curve-adj')} n={m.get('order')}"
    return (f"{label:22s} k={res['k']:3d}  r={res['pearson_r']:+.4f}  "
            f"rho={res['spearman_rho']:+.4f}  MAD={res['mad']:.4f}  "
            f"perm_p={res['perm_p_value']:.5f}  "
            f"asym={res['max_asymmetry']:.2e}  offtridiag={res['max_abs_offtridiagonal']:.2e}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("files", nargs="*")
    ap.add_argument("--all", action="store_true", help="scan pubs/ and pubs/nd/ for all cov_*.json")
    ap.add_argument("--trials", type=int, default=20000)
    ap.add_argument("--seed", type=int, default=12345, help="RNG seed, for reproducibility")
    ap.add_argument("--json-out", type=str, default=None)
    args = ap.parse_args()

    root = Path(__file__).parent
    files = [Path(f) for f in args.files]
    if args.all:
        files = sorted(root.glob("cov_*.json")) + sorted((root / "nd").glob("cov_*.json"))
        # skip known-corrupt / redundant files
        files = [f for f in files if f.name != "cov_matrix_n11.json"]

    if not files:
        print("no input files (use --all or pass file paths)", file=sys.stderr)
        sys.exit(1)

    results = []
    for f in files:
        try:
            res = analyze(f, args.trials, args.seed)
        except Exception as e:
            print(f"FAILED  {f}: {e}", file=sys.stderr)
            continue
        results.append(res)
        print(fmt(res))

    if args.json_out:
        with open(args.json_out, "w") as f:
            json.dump(results, f, indent=2, default=str)


if __name__ == "__main__":
    main()
