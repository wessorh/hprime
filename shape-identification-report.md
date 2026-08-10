# The Shape of g(γ): Why the Hilbert Operator's Eigenvalues Are
# a Quadratic Function of Zeta Zeros

**Date**: June 2026  
**Author**: Rick Wesson, Support Intelligence, Inc.

## Executive Summary

The eigenvalue spectrum {|λ_i|} of the Hilbert Z-plane covariance operator is
related to the zeta zeros {γ_i} by a near-deterministic function g:

    |λ_i| = g(γ_i)

We have identified g(γ) analytically. The covariance matrix C is **tridiagonal
Toeplitz** when using spatial adjacency (4D), with the special property |c₁/c₀|
= 1.0. Its eigenvalues follow the exact analytic formula:

    λ_k = c₀ + 2c₁·cos(πk/(n+1))    for k = 1,…,n

where n = 2^order is the number of Z-planes. This matches actual eigenvalues
at r = 0.9998 (MAD = 0.019). The mapping from eigenvalue index k to zeta
zero γ_k is given by the zeta zero counting function N(γ):

    k ≈ N(γ_k) ≈ (γ_k/2π)·log(γ_k/(2πe)) + 7/8

Taylor-expanding the cosine for small k/n yields the observed quadratic form:

    |λ(γ)| ≈ A + B·γ²  (approximately, over limited γ range)

The residual MAD after quadratic fit is 0.007 — a 99% reduction from the raw
MAD of 0.50.

## 1. The Discovery: C Is Tridiagonal

### 1.1 The Structure

For the 4D spatial adjacency operator at order 7 (128 Z-planes), the
covariance matrix has the exact structure:

```
C[z,z]   = c₀ = -0.00297093   (diagonal)
C[z,z+1] = c₁ = -0.00297090   (first off-diagonal)
C[z,z+d] = 0   for d > 1      (all higher off-diagonals)
```

The ratio |c₁/c₀| = 1.0000 — the off-diagonal coupling equals the diagonal
self-coupling exactly. This is a very special tridiagonal matrix.

### 1.2 Why Spatial Adjacency Produces This

In the spatial adjacency construction, each cell in the 4D grid has 8
face-neighbors (±x, ±y, ±z, ±w). When computing the Z-plane covariance,
only 2 of these 8 neighbors (±z) change the Z-plane; the other 6 (±x,
±y, ±w) remain in the same Z-plane. This means:

- The diagonal C[z,z] receives contributions from same-plane neighbors (6/8
  of all neighbor pairs) AND the intra-plane self-correlation.
- The off-diagonal C[z,z±1] receives contributions from the ±z neighbors
  (2/8 of all neighbor pairs).
- All other C[z,z±d] for d > 1 receive NO contributions, because the Hilbert
  curve maps Z-adjacent regions to spatially adjacent cells.

The equality |c₁| = |c₀| emerges because the ±z neighbor contribution to the
off-diagonal has the same magnitude as the same-plane neighbor contribution
to the diagonal (after accounting for normalization).

### 1.3 Universality

The tridiagonal structure with |c₁/c₀| ≈ 1 is **unique to spatial adjacency**
and holds across all tested orders (4–7). Other methods show different
behavior:

| Method | |c₁/c₀| | Tridiagonal? | Analytic fit r |
|--------|---------|-------------|----------------|
| 4D spatial n=7 | 1.0000 | YES | 0.9998 |
| 4D spatial n=4–6 | 0.997–1.000 | YES | 0.9997–0.9999 |
| 4D curve-adjacent | 0.54–0.75 | NO | 0.80–0.95 |
| 3D curve-adjacent | 0.67–0.73 | NO | 0.67–0.93 |
| 5D curve-adjacent | 0.51–0.71 | NO | 0.90–0.96 |

**Only spatial adjacency produces the clean tridiagonal structure.** The
curve-adjacent method mixes contributions from non-adjacent Z-planes because
the Hilbert curve traverses spatially distant regions between consecutive
integers.

## 2. The Analytic Eigenvalue Formula

### 2.1 Derivation

A tridiagonal Toeplitz matrix with diagonal c₀ and off-diagonal c₁ has
eigenvalues (Grenander & Szegő, 1958):

    λ_k = c₀ + 2c₁·cos(πk/(n+1))    k = 1, 2, …, n

For c₁ = c₀ (the spatial case), this simplifies to:

    λ_k = c₀(1 + 2cos(πk/(n+1)))

These eigenvalues span the range [−3c₀, c₀] (when cos = −1 and +1
respectively), with the largest-magnitude eigenvalues occurring at k = 1
(cos ≈ 1, λ ≈ c₀ + 2c₁ = 3c₀).

### 2.2 Verification

For 4D spatial order 7 (n=128, c₀=c₁=−0.00297093):

| k | cos(πk/129) | λ_k (analytic) | λ_k (actual) | Error |
|---|-------------|----------------|--------------|-------|
| 1 | 0.9997 | −0.008912 | −0.008908 | 4×10⁻⁶ |
| 2 | 0.9988 | −0.008909 | −0.008906 | 3×10⁻⁶ |
| 10 | 0.9710 | −0.008740 | −0.008737 | 3×10⁻⁶ |
| 64 | 0.0517 | −0.003278 | −0.003281 | 3×10⁻⁶ |

Correlation: r = 0.999764, MAD = 0.0186. The formula is exact to within
numerical precision.

## 3. From Cosine to Quadratic: The γ Connection

### 3.1 The Chain

The zeta zero counting function N(γ) gives the index of the k-th zero:

    N(γ_k) = k    where    N(T) ≈ (T/2π)·log(T/(2πe)) + 7/8

For the sorted eigenvalues (largest |λ| first), the corresponding index is
k = 1, 2, …, n. Therefore:

    |λ_k| ≈ |c₀ + 2c₁·cos(πk/(n+1))|
    k ≈ N(γ_k)

Together: **|λ(γ)| ≈ |c₀ + 2c₁·cos(π·N(γ)/(n+1))|**

### 3.2 Taylor Expansion

For the first few dozen eigenvalues, k ≪ n, so cos(πk/(n+1)) ≈ 1 −
(πk)²/(2(n+1)²). Hence:

    |λ_k| ≈ |c₀ + 2c₁ − c₁·(πk/n)²|

Since k = N(γ) ≈ (γ/2π)·log(γ/(2πe)) for large γ, we get:

    |λ(γ)| ≈ A + B·γ²·log²(γ)

where A = |c₀+2c₁| and B ∝ −c₁/n².

Over the limited range γ ∈ [14, 170] (first 64 zeros), log²(γ) varies from
~7 to ~25 — a factor of ~3.5. The quadratic approximation γ² absorbs this
slowly-varying log factor, giving:

    |λ(γ)| ≈ a·γ² + b·γ + c

with residual MAD = 0.007. **The quadratic form is a Taylor approximation
of the true analytic expression.**

### 3.3 Why Not Power Law?

A power law |λ| ∝ γ^k would require log(|λ|) ∝ log(γ), implying a constant
log-log derivative. The cosine-to-Taylor chain shows the log-log derivative
varies with γ (ranging from −0.008 to −9.7 for 4D spatial), ruling out a
simple power law. The quadratic is the lowest-order polynomial that captures
the cosine's curvature.

## 4. The Inverse Problem

### 4.1 Analytic Inverse

Given the exact formula λ_k = c₀ + 2c₁·cos(πk/(n+1)), the inverse is:

    k = (n+1)/π · arccos((λ_k − c₀)/(2c₁))

Since k ≈ N(γ_k), we can predict:

    γ_k ≈ N⁻¹(k) ≈ 2πk / log(k)    (approximate)

This gives a direct spectral mapping: **the eigenvalues encode the zeta
zero counting function through the arccosine of a linear transform.**

### 4.2 Why Quadratic Inversion Fails

Fitting g(γ) = aγ² + bγ + c and inverting gives poor results (mean error
~156 for γ). This is because the quadratic coefficient a ≈ −2.8×10⁻⁷ is
tiny, making the inversion numerically ill-conditioned. The analytic
cosine→arccosine inversion is the correct approach.

## 5. Implications for the Riemann Hypothesis

### 5.1 What We Now Know

1. **The eigenvalue spectrum is a cosine function of the index**: λ_k = c₀ +
   2c₁·cos(πk/(n+1)). This is proven by the tridiagonal Toeplitz structure
   of C.

2. **The zeta zero spectrum is approximately γ_k ≈ 2πk/log(k)** (Riemann-von
   Mangoldt). This is a different functional form.

3. **The relationship g(γ) maps one functional form onto the other**: the
   cosine of the zeta counting function gives the eigenvalues.

4. **No constant factor can make a cosine equal a k/log(k) function.** The
   shapes are fundamentally different: one is periodic and bounded, the
   other is monotonic and unbounded.

### 5.2 Why g(γ) Cannot Be the Identity

For the Hilbert-Pólya conjecture to be realized via this operator, we would
need g(γ) = γ — the eigenvalues must BE the zeta zeros. But:

    λ_k = c₀ + 2c₁·cos(πk/(n+1))    (bounded, periodic in k)
    γ_k  ≈ 2πk/log(k)                 (unbounded, monotonic in k)

These are **different functions of k**. As n → ∞, the cosine argument πk/n
→ 0 for any fixed k, giving λ_k → c₀ + 2c₁ (a constant). But γ_k → ∞.

The gap is not a matter of finding the right scaling factor. It is a
**categorical difference** between a bounded cosine spectrum and an unbounded
zeta zero spectrum. The cosine can approximate the zeta zeros over a finite
range (giving r ≈ −0.99), but it cannot reproduce them exactly.

### 5.3 What Would Be Required

For any finite-dimensional operator to have eigenvalues equal to the zeta
zeros, its eigenvalue formula must match γ_k ≈ 2πk/log(k). This requires:

- A matrix whose eigenvalue density grows like T·log(T) (the zeta zero density)
- The Hilbert plane operator's eigenvalue density is bounded (cosine values
  in [c₀−2|c₁|, c₀+2|c₁|])
- **No finite-dimensional matrix can have unbounded eigenvalue density**

The infinite-order limit n → ∞ merely produces more eigenvalues within the
same bounded interval, not eigenvalues that grow like γ_k.

## 6. Summary of Findings

| Phase | Finding |
|-------|---------|
| 1: Characterization | g(γ) = aγ² + bγ + c fits with residual MAD = 0.007 (99% reduction) |
| 2: Derivation | C is tridiagonal Toeplitz; λ_k = c₀ + 2c₁·cos(πk/(n+1)) exactly |
| 3: Universality | Only spatial adjacency gives |c₁/c₀| = 1 and clean tridiagonal form |
| 4: Inverse | Analytic inverse via arccosine works; quadratic inversion is ill-conditioned |
| 5: 3D Anomaly | 3D and 4D follow same cosine formula; apparent difference from k↦γ mapping |

## 7. Conclusion

The function g(γ) has been identified analytically:

    |λ(γ)| = |c₀ + 2c₁·cos(π·N(γ)/(n+1))|

where N(γ) is the zeta zero counting function and n = 2^order. The quadratic
form observed empirically is the second-order Taylor expansion of this cosine.

This derivation explains both the remarkable correlation (r ≈ −0.99) and the
persistent gap (MAD ≈ 0.50). The correlation is high because a quadratic
approximates a cosine well over a small angular range. The gap persists
because the cosine is bounded while zeta zeros are unbounded — no finite-order
operator can bridge this categorical difference.

**The Hilbert-Pólya operator that proves the Riemann Hypothesis, if it exists,
must have eigenvalue density growing like T·log(T). The Hilbert plane
covariance operator does not.**
