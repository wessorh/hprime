# Show HN: A Discrete Hilbert-Pólya Operator (Candidate RH Proof)

**Title:** Show HN: A Discrete Hilbert-Pólya Operator — eigenvalues converge to Riemann zeta zeros

**URL:** https://github.com/wessorh/hprime

**Body:**

In 1859, Riemann conjectured that all non-trivial zeros of the zeta function lie on a single vertical line. In the 1980s, Hilbert and Pólya suggested a shortcut: find a self-adjoint operator whose eigenvalues are exactly the imaginary parts of the zeros. Nobody has found one that works.

This repo contains what we believe is the first explicitly constructible candidate. It's a discrete Schrödinger operator built from two ingredients:

- **The kinetic term** comes from the face-adjacency geometry of 3D Hilbert curves. When you map integers onto a Hilbert curve, count primes on each horizontal slice, and measure how adjacent slices couple, the resulting matrix is exactly tridiagonal — a one-paragraph proof.

- **The potential term** is V(z) = (z/2π)·log(z/2πe), the asymptotic inverse of the zeta zero counting function. A Schrödinger operator with this potential has the correct eigenvalue density by the Weyl law.

Neither ingredient works alone. The Hilbert geometry gives correct eigenvalue ordering (|r| = 0.994) but wrong magnitudes (eigenvalues are bounded cosines). The logarithmic potential gives correct magnitudes (|r| = 0.999) but approximate ordering. Combined, the hybrid operator H = −Δ + V(z) achieves |r| = 1.0000 correlation with known zeta zeros across orders n=4 through n=10 (16 to 1024 planes).

The proof that this isn't just curve-fitting has three parts (13 pages total in the repo):

1. Individual eigenvalue convergence at rate O(1/(N·log N)) via Davis-Kahan and sin² averaging
2. Strong resolvent convergence to a self-adjoint limit H_∞ via the Rellich criterion
3. Spectral measure equality μ_∞ = μ_ζ via Gershgorin circles with a split-sum estimate

If these hold, the eigenvalues of H_∞ are the zeta zeros. Since H_∞ is self-adjoint, its eigenvalues are real. All zeros lie on the critical line. RH follows.

Every numerical result is reproducible: `go build ./cmd/hybrid && ./hybrid`

I'm posting this here because HN has the right combination of mathematical depth and intellectual honesty to stress-test this before any formal submission. What breaks first?
