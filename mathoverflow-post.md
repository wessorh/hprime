# Title

Is the eigenvalue perturbation bound |λ_k(2N) − λ_k(N)| = O(1/(N·log N)) valid for this tridiagonal operator?

# Body

I'm studying a discrete Schrödinger operator whose eigenvalues numerically match the imaginary parts of Riemann zeta zeros (correlation coefficient |r| = 1.000 at N=1024). I need to verify whether the eigenvalue perturbation bound I'm using is mathematically sound, as the entire proof of convergence depends on it.

## The Operator

For N = 2^n, define the N×N symmetric tridiagonal matrix H_N:

```
H_{z,z}   = γ_z                    (z = 0,...,N-1)
H_{z,z+1} = (1/N)·√(γ_z·γ_{z+1})   (z = 0,...,N-2)
```

where γ_z is the z-th zeta zero (exact for z < 64, Riemann–von Mangoldt approximation γ_z ≈ 2πz/log(z) thereafter).

This is H = diag(γ) + ε·A where ε = 1/N and A is the tridiagonal adjacency matrix with entries A_{z,z+1} = √(γ_z·γ_{z+1}).

## The Question

When we double the resolution (N → 2N), I need to bound the eigenvalue perturbation:

```
|λ_k^{(2N)} - λ_k^{(N)}| ≤ ?
```

Numerically this behaves as O(1/(N·log N)) — summable, giving Cauchy convergence for each fixed k. My analytic argument:

**Davis-Kahan step:** The first-order eigenvalue shift is ⟨ψ_k, D ψ_k⟩ where D = P* H_{2N} P - H_N (P is the coarse-graining projection) and ψ_k are the eigenvectors of H_N. For the tridiagonal Laplacian, ψ_k(z) ≈ √(2/N)·sin(kπz/(N+1)).

**Sin² averaging:** The diagonal of D is γ_{2z} − γ_z ≈ 2πz/log(z). The inner product is:

```
⟨ψ_k, D ψ_k⟩ = (2/N)·Σ_{z=0}^{N-1} sin²(kπz/(N+1))·(γ_{2z} − γ_z)
```

Because sin² has k complete oscillations over [0,N] and γ_{2z}−γ_z is slowly varying (its derivative is ~1/log(z)), the oscillation averages the diagonal difference. The residual after averaging over one oscillation period is O(z/(N·log z)) — smaller by a factor of 1/N than the naive bound.

Specifically: expanding γ_{2z} − γ_z around z = N/2 gives a leading term proportional to z/log(z). The sin² kernel is orthogonal to constants and linear functions over a full period, so the first non-vanishing moment is the second derivative of the diagonal difference, which is O(1/log(z)) — smaller again by 1/N.

This gives:

```
|⟨ψ_k, D ψ_k⟩| = O(1/(N·log N)) for each fixed k.
```

Which is summable over N = 2^n (Σ 1/(N·log N) converges).

**Off-diagonal contribution:** The coupling term difference is O(1/log²(N)), so it's subdominant.

## What I'm Asking

1. Is the sin² averaging step valid? Specifically, does the orthogonality of sin²(kπz/(N+1)) to linear functions over a full period imply that the leading O(z/log z) contribution cancels to O(1/(N·log N))? I can see numerically that it does, but I need to know if there's a rigorous formulation — perhaps via integration by parts on the Riemann–Stieltjes sum.

2. If this bound is valid, does Davis–Kahan give |λ_k^{(2N)} − λ_k^{(N)}| ≤ |⟨ψ_k, D ψ_k⟩| + O(‖D‖²)? Or is there a sharper form?

3. Are there known counterexamples where the sin² averaging fails to give the expected cancellation for a slowly-varying diagonal perturbation to a tridiagonal Laplacian?

## Context

The full construction and numerical validation are at: https://github.com/wessorh/hprime

If this bound holds, the eigenvalues converge individually → strong resolvent convergence → the limit H_∞ is self-adjoint → its spectral measure equals the zeta zero distribution (via Gershgorin split-sum) → the eigenvalues of H_∞ are the zeta zeros → RH.

The chain is three short papers (13 pages total) in the repository. Everything reproduces with `go build`. I'm looking for either validation that the sin² averaging step is rigorous, or a clear explanation of why it fails.
