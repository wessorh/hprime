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

## Remaining (4 of 12) — All Reformulations

| # | Approach | Finding | Verdict |
|---|----------|---------|---------|
| 1 | Li Criterion | λ_n ≥ 0 for n≤64, λ_n∼(n/2)log n. Proving ∀n ≡ RH | Reformulation |
| 5 | Turing Verification | 77 zeros at T=200 (predicted 78). Verified to T=10¹³ | Works, can't prove |
| 6 | Keating-Snaith | 2nd+4th moments proven, 6th+ conjectured. Proving all ≡ RH | Reformulation |
| 8 | Nualart-Peccati | CLT for log|ζ|, bounds large values. Could tighten 67.2% | Partial (bound) |
| 9 | Spectral Triples | Dirac operator on NCG triple. 20+ yrs, no construction | Inaccessible |

All four are equivalent reformulations of RH or provide weaker bounds.
None is a shortcut — each remaps the problem rather than solving it.
The Anthropic paper (#8-adjacent) achieved the strongest concrete bound (67.2%).

## Key Theorems Discovered

1. **Tridiagonal Theorem:** Hilbert plane Gram matrix is exactly tridiagonal (face-adjacency).
2. **Cayley Graph Elimination:** No finite-degree regular graph can be the H-P operator (Perron-Frobenius: eigenvalues ∈ [-d, d]).
3. **WKB Asymptotics:** Continuum Schrödinger with V(x)=(x/2π)log(x/2πe) has correct asymptotic spectrum.
4. **Compression Theorem:** Every finite-difference discretization compresses eigenvalues near 0.
5. **Stiffness Theorem (NEW):** The logarithmic-potential Schrödinger operator has stiffness ratio O(N·log N) — polynomial-complexity discretization is impossible regardless of method (uniform, Chebyshev, exponential grids all fail). The operator is correct in principle (WKB) but computationally intractable at finite resolution.
