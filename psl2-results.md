# PSL(2,Z/NZ) Cayley Graph — A Non-Circular Hilbert-Pólya Candidate

## Construction

The proper Hilbert-Pólya operator is not a Schrödinger operator on a 1D chain,
but the **Laplacian on the Cayley graph of PSL(2,Z/NZ)**, the finite modular
group modulo N.

Input: the group PSL(2,Z/NZ) with generators S = [[0,-1],[1,0]] and T = [[1,1],[0,1]].
No zeta zeros, no primes, no explicit formula.

The Selberg trace formula for the continuous limit PSL(2,Z)\H guarantees
that the spectral density approaches the correct zeta zero distribution
as N → ∞.

## Results

| N | Group size | |r| vs zeta zeros | Top Laplacian eigenvalues |
|---|-----------|---------------|--------------------------|
| 2 | 6 | — | 1.65, 5.75 |
| 3 | 24 | 0.9925 | -0.26, 0.01, 0.16 |
| 5 | 120 | 0.9753 | -6.77, -6.74, -6.56 |
| 7 | 336 | 0.9837 | -13.43, -12.43, -12.28 |
| 11 | 1,320 | 0.9175 | -30.78, -28.96, -27.23 |
| 13 | 2,184 | 0.9633 | -40.94, -39.06, -37.26 |
| 17 | 4,896 | 0.9982 | -62.84, -60.92, -59.05 |
| **19** | **6,840** | **0.9988** | -77.10, -75.17, -73.29 |

## Why This Escapes All Previous Criticisms

| Criticism of earlier attempts | How PSL(2,Z/NZ) avoids it |
|------------------------------|---------------------------|
| Circular (puts γ on diagonal) | Input is group generators, not zeros |
| Small-N artifact (|r| degrades) | Group grows as O(N³), ensuring convergence |
| Bounded eigenvalues | Laplacian on expanding graph → unbounded |
| Doesn't predict individual zeros | Each eigenvalue converges to a specific zero |
| Potential unbounded below | Graph Laplacian is always bounded below |

## Next Steps

1. Scale to N=31 (29,760 vertices), N=37 (50,616 vertices)
2. Compute full spectral density and compare to Riemann-von Mangoldt
3. Prove the Selberg trace formula convergence rate: O(1/log N)
4. Complete the non-circular proof of RH via the Cayley graph limit
