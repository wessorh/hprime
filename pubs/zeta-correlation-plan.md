# Investigation Plan: Hilbert-Curve Prime Density vs Zeta Zero Spacing

## Goal

Test whether the structurally-biased prime density on specific 3D Hilbert
curve planes correlates with the local spacing of non-trivial zeta zeros.
If confirmed, this provides a geometric realization of the spectral
interpretation at the heart of the Hilbert-Pólya conjecture.

## Background

We have established at orders 5–8 that:

1. Primes map non-uniformly onto axis-aligned planes of the 3D Hilbert curve
2. |z_max| grows as √(2^n), reaching 18.7 at order 8 (p < 10⁻⁷⁷)
3. Specific planes (Z=31, Z=63, Z=127) are persistently "hot" across orders
4. Adjacent planes show 2–5× residue-class polarization (mod 3, mod 7)
5. The hot-plane sequence follows Z_hot(n) = 2^(n-2) − 1 for n ≥ 5

The Riemann explicit formula connects prime distribution to zeta zeros:

$$\psi(x) = x - \sum_{\rho} \frac{x^{\rho}}{\rho} - \log 2\pi - \frac{1}{2}\log(1-x^{-2})$$

If hot Hilbert planes select integer ranges where the oscillatory sum over
zeta zeros is systematically constructive, the zero-spacing in the
corresponding spectral range should be anomalous.

## Phase 1: Precompute Zeta Zeros (Week 1)

### 1.1 Obtain zero data

Use Andrew Odlyzko's published tables or compute zeros via `arb`/`mpmath`:

- **Source**: LMFDB or Odlyzko's 2 billion+ zero tables
- **Range needed**: zeros up to height T ≈ 10⁷ (covering integer range up to 16.7M at order 8)
- **Actually needed**: zeros up to T ≈ 10⁹ if investigating order 9–10
- **Start with**: First 100,000 zeros (available from LMFDB; covers T up to ~75,000)

### 1.2 Normalize zero spacings

Compute Montgomery's normalized spacings:

$$\delta_i = \frac{\gamma_{i+1} - \gamma_i}{2\pi / \log(\gamma_i/2\pi)}$$

These should follow the GUE (Gaussian Unitary Ensemble) distribution if
Montgomery's pair correlation conjecture holds. Deviations from GUE in
specific spectral ranges are the signal we're looking for.

### 1.3 Store format

Zero file: `zeros_100k.json` — one JSON object per zero:
```json
{"n": 1, "gamma": 14.134725, "spacing": 0.872, "gue_deviation": 0.03}
```

## Phase 2: Map Hilbert Planes to Integer Ranges (Week 1–2)

### 2.1 For each Z-plane, collect the set of integers mapping to it

At order n, plane Z=z contains exactly 4^n integers:

$$I_z = \{k \in [0, 8^n) : H_3(k).z = z\}$$

These are NOT contiguous — they're interleaved across the full range.
But the Hilbert locality property means they form structured clusters.

### 2.2 Compute the prime density function per plane

For each Z-plane z, compute:

$$D(z) = \frac{|\{p \in I_z : p \text{ prime}\}|}{|I_z|}$$

Compare to the expected density $1/\log(M)$ where M = 2^{n-1} (midpoint of range).

### 2.3 Compute the zeta error integral per plane

For each plane, approximate the explicit formula's oscillatory contribution
summed over the integers in that plane:

$$S(z) = \sum_{k \in I_z} \left(\psi(k) - k\right) \approx -\sum_{k \in I_z} \sum_{\gamma} \frac{k^{i\gamma}}{\rho}$$

This is computationally heavy — approximate by sampling k at intervals and
using the first 100K zeros.

### 2.4 Produce the correlation vector

For n=6 (64 planes, manageable):
- Vector P of length 64: P[z] = excess prime density on plane z
- Vector S of length 64: S[z] = mean oscillatory error on plane z

## Phase 3: Correlation Test (Week 2–3)

### 3.1 Primary test: Pearson correlation

Compute $\rho = \text{Corr}(P, S)$ — the Pearson correlation between excess
prime density and mean oscillatory error across planes.

**Null hypothesis:** $\rho = 0$ (no correlation).
**Alternative:** $\rho > 0$ (hot planes correspond to positive S(x) regions).

### 3.2 Permutation test

Shuffle the plane labels 10,000 times and recompute the correlation.
If the observed $\rho$ exceeds 95% of permuted correlations, reject null.

### 3.3 Spectral-range test

Divide the zeta zeros into 64 equal-width bands by height γ. For each band,
compute the mean GUE deviation. Correlate with the plane-density vector P.

If hot planes correlate with high-GUE-deviation spectral bands, that
suggests the Hilbert curve is "reading" the zeta zero structure.

### 3.4 Adjacent-plane oscillation test

The most dramatic signal is adjacent-plane polarization (Z=30 vs Z=31).
Test whether the integer ranges I_30 and I_31 sample regions where S(x)
has opposite sign — i.e., whether the oscillatory error flips polarity
between adjacent planes.

Compute:
$$A(z) = \text{sign}(\text{mean } S(k) \text{ for } k \in I_z)$$
and test whether $A(z) \neq A(z+1)$ for planes with large |z-score|.

## Phase 4: Scale to Higher Orders (Week 3–4)

### 4.1 Extend to order 9–10

At order 9 (512³ = 134M cells, ~7.6M primes):
- 512 Z-planes, each containing 262,144 integers
- Compute P and S vectors of length 512
- Correlation test as in Phase 3

At order 10 (1024³ = 1.07B cells, ~54M primes):
- 1024 Z-planes, each containing ~1M integers
- Requires distributed computation or GPU acceleration

### 4.2 GPU acceleration for order 10+

The Hilbert curve mapping for 1B+ points is embarrassingly parallel.
A CUDA kernel computing d2xyz3D for 1B integers would take ~1 second on
an A100. The prime sieve for 1B+ range needs ~100 GB — distributed across
multiple nodes.

## Phase 5: Machine Learning Validation (Week 4)

### 5.1 Train a classifier

Train a simple model (logistic regression or random forest) to predict
whether an integer is prime using only its Hilbert coordinates (x, y, z)
as features. If the model achieves AUC > 0.5, the Hilbert coordinates
encode prime-relevant information beyond random.

### 5.2 Feature importance

Extract feature importance: which coordinate (X, Y, or Z) is most
predictive of primality? This should match our plane-analysis results
(Z is most predictive at orders 5–8).

### 5.3 Compare to other space-filling curves

Repeat the entire analysis with the Moore curve, the Sierpiński curve,
and a random permutation (control). If the Hilbert curve shows
significantly stronger correlation than alternatives, the effect is
specific to its recursive structure — not a universal property of
space-filling curves.

## Milestones and Checkpoints

| Week | Deliverable | Success Criterion |
|------|-------------|-------------------|
| 1 | Zero data + Hilbert mapping at n=6 | Pipeline runs end-to-end |
| 2 | P and S vectors computed | 64-element correlation vectors |
| 3 | ρ > 0.3 with p < 0.05 | Statistically significant correlation |
| 3 | Adjacent-plane sign flip confirmed | A(z) ≠ A(z+1) for top 10% of planes |
| 4 | Order 9 results | |z_max| ≈ 22.6, correlation holds |
| 4 | ML validation | AUC > 0.52 on holdout set |

## Resources Required

| Resource | Quantity | Purpose |
|----------|----------|---------|
| Zeta zero tables | 100K–1M zeros | LMFDB download or Odlyzko tables |
| Compute (CPU) | 64 cores, 256 GB RAM | Order 9–10 Hilbert mapping + sieve |
| Compute (GPU) | 1× A100 or 4× V100 | Order 10+ Hilbert mapping |
| Storage | 500 GB | Zero tables, Hilbert mappings, intermediate results |
| Software | `arb` or `mpmath` | Zeta zero computation/verification |

## If Correlation Is Confirmed

1. **Hilbert-Pólya operator approximation**: The hot Z-planes are approximate
   eigenvectors of a discrete operator whose eigenvalues approximate zeta zeros.
   The plane index z would correspond to the eigenvector index.

2. **New primality heuristic**: Computation of Hilbert coordinates could serve
   as a fast pre-filter for Miller-Rabin — numbers on cold planes are ~30%
   less likely to be prime, reducing the expected number of expensive tests.

3. **Physical interpretation**: If the zeta zeros are eigenvalues of a quantum
   Hamiltonian (the Hilbert-Pólya conjecture), the Hilbert curve's plane
   structure would correspond to the spatial organization of wavefunctions in
   the underlying quantum system — hot planes are regions of high probability
   amplitude.

4. **Publication**: Results at this significance level (p < 10⁻⁷⁷ for plane
   bias, plus correlation p < 0.05) would merit submission to *Experimental
   Mathematics*, *Mathematics of Computation*, or *Physical Review Letters*
   (for the quantum interpretation).

## If Correlation Is NOT Confirmed

1. The plane bias is a purely combinatorial artifact of the Hilbert curve's
   3-bit encoding interacting with prime residue constraints — interesting
   but not zeta-related.

2. The tools (parallel sieve, Hilbert mapping, plane analysis) remain useful
   for exploring other structural properties of primes (gap distributions,
   twin prime localization, Goldbach partitions).

3. The negative result itself is publishable: it rules out a specific
   mechanism for Hilbert-Pólya realization.
