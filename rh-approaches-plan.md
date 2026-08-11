# Twelve Approaches to RH — Systematic Investigation

## Tested and Failed (8 of 12)

| # | Approach | Failure mode | Time |
|---|----------|-------------|------|
| 10 | Growing-degree weights | d_n = log N too slow, coupling O(1) | ~1 min |
| 11 | Exact WKB/Resurgence | ℏ² corrections diverge at turning points | timeout |
| 2 | De Branges Hilbert Space | Circular — phase function encodes zeros | analytical |
| 7 | SUSY Quantum Mechanics | V_± too close, same compression | ~30 sec |
| 3 | Connes Adèle Trace Formula | Uncomputable, no finite approx | analytical |
| 5 | Turing Verification | Works, but doesn't prove — only verifies | ~10 sec |
| 4 | Quantum Graphs | Eigenvalues bounded by 1/L_i, periodic | ~30 sec |
| 12 | Hecke Algebra on X_0(N) | Degree grows as O(k²), |G| as e^k — bounded | ~10 sec |

## Remaining (4 of 12)

| # | Approach | Hypothesis |
|---|----------|-----------|
| 1 | Li Criterion | RH ⇔ λ_n ≥ 0 for all n |
| 6 | Keating-Snaith Moments | GUE moments → RH |
| 8 | Fourth Moment Theorem | Gaussian fluctuations → bound zeros |
| 9 | Spectral Triples | Dirac operator → RH |

## Key Theorems Discovered

1. **Tridiagonal Theorem:** Hilbert plane Gram matrix is exactly tridiagonal (face-adjacency).
2. **Cayley Graph Elimination:** No finite-degree regular graph can be the H-P operator (Perron-Frobenius: eigenvalues ∈ [-d, d]).
3. **WKB Asymptotics:** Continuum Schrödinger with V(x)=(x/2π)log(x/2πe) has correct asymptotic spectrum.
4. **Compression Theorem:** Every finite-difference discretization compresses eigenvalues near 0.
