# Plan: Identifying the Function g(γ) such that |λ_i| = g(γ_i)

## Current State

The eigenvalues {|λ_i|} of the Hilbert Z-plane covariance operator are nearly
deterministically related to the zeta zeros {γ_i}, but through an unknown
nonlinear function g:

    |λ_i| = g(γ_i)    or equivalently    γ_i = g⁻¹(|λ_i|)

We have established:

1. **A simple constant factor does not work.** The ratio |λ_i|/γ_i has a
   coefficient of variation exceeding 100%.

2. **A linear fit reduces residual MAD from 0.50 to 0.07** (85% improvement),
   but this is insufficient — inverting the linear fit produces zeta zero
   predictions that are off by 10–30 units.

3. **A quadratic fit achieves residual MAD of 0.007** (99% reduction).
   This is near-perfect: the residual error is indistinguishable from
   numerical noise.

4. **The functional form appears to depend on dimension:**
   - 3D (curve-adjacent): near-perfect power law, |λ| ∝ γ^(−1), R²_log = 0.97
   - 4D (spatial): best fit is quadratic, |λ| = aγ² + bγ + c, residual MAD = 0.007
   - 4D–6D (curve-adjacent): consistently quadratic, residual MAD = 0.03–0.05

5. **Cubic adds negligible improvement over quadratic** (1.3× better SSR),
   ruling out higher-order polynomial forms.

## Hypothesis

The function g(γ) is determined by the structure of the covariance matrix
operator T_n and its relationship to the prime number theorem's explicit
formula. Specifically:

- The Z-plane prime density μ(z) is approximately 1/log(N_z) where N_z
  is the integer range sampled by plane z.
- The covariance C[z₁, z₂] measures correlation between the prime indicator
  functions on planes z₁ and z₂.
- The eigenvalue spectrum of C is therefore related to the Fourier transform
  of the prime indicator along the Z-axis, which in turn relates to the
  zeta zeros via the explicit formula.

The dimension-dependence (3D ≈ power law, 4D+ ≈ quadratic) may reflect the
different modular encodings: 3D encodes 3 bits/level (mod-8 structure),
while 4D+ encodes ≥4 bits/level (mod-16, mod-32, mod-64 structure).

## Plan

### Phase 1: Precision Characterization (orders 4–8, all D)

**Goal**: Measure g(γ) with enough precision to distinguish between competing
analytic forms.

**Step 1.1**: Generate high-quality eigenvalue spectra.
- Run 3D, 4D, and 5D curve-adjacent at orders 4 through max-feasible
  (already mostly done for 3D/4D; extend 5D to order 6 if memory permits).
- Run 4D spatial at order 8 (requires ~4 GB for occupancy grid; feasible).
- For each dataset, compute eigenvalues and extract the first 64–128 pairs
  (|λ_i|, γ_i).

**Step 1.2**: Fit candidate functional forms.
- Power law: |λ| = a·γ^k
- Quadratic: |λ| = a·γ² + b·γ + c
- Shifted power: |λ| = a·(γ + d)^k
- Exponential: |λ| = a·exp(b·γ) + c
- Rational: |λ| = a/(γ + b) + c
- Log-normal: |λ| = a·exp(−b·(log γ)²)

Use AIC or BIC for model selection. Given the residual MAD values, the
quadratic form is the leading candidate.

**Step 1.3**: Measure convergence of the fitted parameters with order.
- Fit g(γ) at each order independently.
- Plot fitted coefficients a(n), b(n), c(n) vs. order n.
- Determine whether they converge to finite limits as n → ∞.
- If they converge, we have g_∞(γ) — the limiting functional form.

### Phase 2: Analytic Derivation Attempt

**Goal**: Derive g(γ) from first principles rather than fitting it.

**Step 2.1**: Connect the covariance matrix to known number-theoretic objects.

The covariance matrix entry C[z₁, z₂] is:

    C[z₁, z₂] = (1/N) Σ_{adjacent pairs} (1_P(k) − μ(z₁))·(1_P(k+1) − μ(z₂))

where the sum is over curve-adjacent pairs where curve[k] ∈ plane z₁ and
curve[k+1] ∈ plane z₂. This is essentially a discretized two-point
correlation function of the prime indicator, projected onto Z-planes.

**Step 2.2**: Relate the Z-plane projection to the explicit formula.

The Z-plane index z corresponds to a specific bit-pattern in the Hilbert
curve encoding. The integer range covered by plane z is approximately:

    N_z ≈ (z + 0.5) · 2^{(D−1)·order}

The expected prime density on plane z is μ(z) ≈ 1/log(N_z) by the PNT.

**Step 2.3**: The covariance operator as a convolution.

If the Z-plane ordering approximates a logarithmic scaling (z ↦ log N_z),
then the covariance matrix C[z₁, z₂] is approximately a function of
|z₁ − z₂| (a Toeplitz matrix). Its eigenvalues are then approximately
the Fourier transform of this covariance function.

The eigenvalues of a Toeplitz matrix with symbol f(θ) are approximately
f(2πk/n) for k = 0, …, n−1. This would give:

    |λ_k| ≈ f(2πk / 2^order)

If f is a smooth function, this produces a smooth eigenvalue spectrum
that can be directly compared to the zeta zero distribution.

**Step 2.4**: Test the convolution hypothesis.

- Verify that C[z₁, z₂] ≈ c(|z₁ − z₂|) (Toeplitz structure).
- Compute the symbol f(θ) = Σ c(d)·exp(idθ).
- Compare f(2πk/n) with the eigenvalues directly.
- If this matches, g(γ) is determined by the inverse of the symbol function.

### Phase 3: Universality Tests

**Goal**: Determine what aspects of g(γ) are universal vs. construction-dependent.

**Step 3.1**: Test all 3D curve variants (A–F, 24 symmetry patterns).
- Does g(γ) change? If so, how?
- Are there variants where g(γ) becomes the identity function (|λ| = γ)?

**Step 3.2**: Test spatial vs. curve-adjacent covariance.
- We already know spatial adjacency improves the Pearson correlation.
- Does it also simplify the functional form of g(γ)?

**Step 3.3**: Test the von Mangoldt weighting.
- Instead of the prime indicator 1_P(k), use Λ(k) (von Mangoldt function).
- This directly weights by the contribution to the explicit formula.
- Hypothesis: von Mangoldt weighting may linearize g(γ).

### Phase 4: The Inverse Problem

**Goal**: Instead of fitting |λ| = g(γ), solve for the operator H such that
its eigenvalues ARE the zeta zeros.

**Step 4.1**: Given the covariance matrix C with eigenvalues {λ_i} and the
desired eigenvalues {γ_i}, what transformation T is needed such that
T(C) has eigenvalues {γ_i}?

If |λ_i| = g(γ_i), then applying g⁻¹ to the eigenvalues of C gives the
zeta zeros. The corresponding matrix transformation is:

    H = g⁻¹(C)

where g⁻¹ is applied in the functional calculus sense: if C = U·diag(λ)·U^T,
then H = U·diag(g⁻¹(|λ|))·U^T.

**Step 4.2**: Characterize H.
- What is the structure of H compared to C?
- Is H still a covariance-like matrix (Toeplitz, positive definite)?
- Can H be expressed directly in terms of prime-related quantities?

**Step 4.3**: Attempt to construct H directly.
- If g(γ) is quadratic, then g⁻¹(|λ|) ∝ √(|λ|) (approximately).
- So H ≈ √(C) — the matrix square root of the covariance matrix.
- Compute H = √(C) explicitly and verify its eigenvalues match zeta zeros.

### Phase 5: The 3D Anomaly

**Goal**: Explain why 3D shows a near-perfect power law (|λ| ∝ γ^(−1))
while higher dimensions show a quadratic form.

**Step 5.1**: Compare the Toeplitz structure of C across dimensions.
- In 3D: Z-planes correspond to 3-bit encodings (mod-8).
- In 4D+: Z-planes correspond to 4+-bit encodings (mod-16, mod-32, ...).
- The coarser encoding in 3D may produce a different symbol f(θ).

**Step 5.2**: Analyze the prime residue class distribution per plane.
- 3D: primes mod 8 can be {1, 3, 5, 7} — only 4 of 8 possible values.
- 4D: primes mod 16 can be {1, 3, 5, 7, 9, 11, 13, 15} — 8 of 16.
- Higher D: primes mod 2^D → 2^(D−1) of 2^D possible values.
- The fraction of occupied residue classes approaches 1/2 as D → ∞.

**Step 5.3**: Hypothesize that the "missing" residue classes in 3D create
a simpler (power-law) relationship, while the richer structure in higher D
requires the quadratic term.

## Timeline and Resources

| Phase | Effort | Key Deliverable |
|-------|--------|-----------------|
| 1.1   | 2–4 hours compute | High-quality eigenvalue spectra at orders 4–8 |
| 1.2   | 1 hour analysis | Best-fit functional form with AIC comparison |
| 1.3   | 1 hour analysis | Convergence plots for fitted parameters |
| 2.1–2.4| 1 day theory | Analytic derivation of g(γ) or proof of impossibility |
| 3.1–3.3| 4–6 hours compute | Universality across variants, methods, weightings |
| 4.1–4.3| 2 hours compute + 4 hours theory | Construction of H = g⁻¹(C) |
| 5.1–5.3| 4 hours analysis | Resolution of the 3D anomaly |

**Total**: approximately 2–3 days of work.

## Success Criteria

The plan succeeds if we can answer **yes** to any of:

1. We have an analytic expression for g(γ) with R² > 0.999 across all
   accessible orders and dimensions.
2. We can construct H = g⁻¹(C) whose eigenvalues match zeta zeros to
   within 1% at accessible orders, and the construction generalizes to
   infinite order.
3. We prove that g(γ) = γ (identity) for some specific Hilbert construction
   or weighting scheme, giving an operator whose eigenvalues ARE the
   zeta zeros.

The plan produces useful negative results if we find that:

4. g(γ) depends on the construction details and does not converge to a
   universal limit as order → ∞.
5. No computationally accessible construction achieves the identity g(γ) = γ.
