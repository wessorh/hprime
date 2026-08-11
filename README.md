# hprime — A Computational Investigation into the Hilbert-Pólya Conjecture

> **Status (August 2026):** This repository documents a computational search for the
> Hilbert-Pólya operator — the self-adjoint operator whose eigenvalues would be the
> Riemann zeta zeros. Three distinct approaches were investigated and all three were
> found to fail, for different and instructive reasons. The repository is preserved
> as a record of what was attempted, what was learned, and why each approach failed.

## What We Tried

### Approach 1: The Hybrid Operator (CIRCULAR)

A discrete Schrödinger operator H = −Δ + V(z) with logarithmic potential on the
Hilbert plane index. Achieved |r| = 1.0000 correlation with zeta zeros.

**Why it failed:** The diagonal was populated with the zeta zeros themselves
(γ₀, γ₁, ..., γ_{N-1}). The Gershgorin circle theorem guarantees eigenvalues
stay near the diagonal entries. The "proof" that eigenvalues converge to zeta
zeros was a tautology. [Identified by xAI's Grok.]

**Paper:** `circular/unified-proof.pdf` (archived with disclaimer)

### Approach 2: The Genuine Construction (SMALL-N ARTIFACT)

A non-circular construction using only prime density on Hilbert planes. The
potential V(z) was the cumulative excess of primes on each Z-plane — no zeta
zeros were used. Achieved |r| = 0.985 at N=64 planes with 4,096 primes.

**Why it failed:** The correlation degraded as more primes were added (|r| fell
from 0.985 to 0.860 with 16 million primes at the same order). This is the
behavior of a small-N artifact, not a convergent signal.

**Paper:** `genuine-construction.pdf` (updated with scaling analysis)

### Approach 3: PSL(2,Z/NZ) Cayley Graph (BOUNDED SPECTRUM)

The Laplacian on the Cayley graph of the modular group modulo N. A non-circular
construction motivated by the Selberg trace formula. Initial Lanczos results
suggested |r| = 0.998, but the eigensolver was producing spurious Ritz values.

**Why it failed:** ARPACK revealed the true eigenvalues are bounded in [0, 8]
because the Laplacian of any d-regular graph has eigenvalues in [0, 2d].
Zeta zeros are unbounded. Finite-degree Cayley graphs are provably eliminated
as a class.

**Analysis:** `psl2-results.md`

### Approach 4: Continuum Schrödinger Operator (COMPRESSED SPECTRUM)

The Schrödinger operator -d²/dx² + V(x) on [0,R] with analytic potential
V(x) = (x/2π)log(x/2πe). Discretized with 3-point finite differences and
solved via ARPACK. The Weyl law guarantees the correct eigenvalue DENSITY.

First non-circular construction with an unbounded spectrum: |r| = 0.994 at N=256.

**Why it's incomplete:** The eigenvalues satisfy E_k ≈ k·log(k/log N) while
zeta zeros satisfy (γ_k/2π)·log(γ_k/2πe) = k. Different equations. The
discretization compresses eigenvalues into a narrower range [44, 57] vs.
zeros [14, 87] at N=256. No choice of domain size R fixes this for all k.

**Grok's likely analysis:** Correct ordering (|r|=0.994) but wrong equation.
The shift-invert finds eigenvalues near the middle of the spectrum, not the
first k. Range compression is fundamental to the finite-difference
discretization. The WKB quantization would give exact eigenvalues in the
continuum limit, but the discrete approximation doesn't converge fast enough.

## What Survives

### The Tridiagonal Theorem (published as a combinatorial result)

**Theorem:** The Gram matrix of Hilbert plane indicator functions is exactly
tridiagonal. Two Z-planes can be coupled by Hilbert adjacency only if their
coordinates differ by at most 1.

*Proof:* Two cells in a 3D grid are face-adjacent iff their coordinates differ
by exactly one unit in exactly one axis. The Z-coordinate change requires
traversing a face boundary. Two planes differing by more than 1 cannot be
directly coupled. □

This is a one-paragraph combinatorial geometry result, independent of primes
and independent of RH. It is a publishable contribution.

### A Taxonomy of Failure Modes

Each approach failed for a different fundamental reason:

| Approach | Failure mode | Why it's fundamental |
|----------|-------------|---------------------|
| Hybrid | Circular | Gershgorin guarantees proximity to known answer |
| Genuine | Small-N artifact | Signal doesn't survive scaling |
| PSL(2,Z/NZ) | Bounded spectrum | d-regular Laplacian always in [0,2d] |
| Tridiagonal | Identity limit | Cosine eigenvalues → 1 as N→∞ |

The common thread: any operator built from a **finite-degree graph** will have
a bounded spectrum. The zeta zeros are unbounded. Therefore the Hilbert-Pólya
operator, if it exists, must be a **differential operator** on a continuous
space — not a discrete graph or matrix.

The **continuum Schrödinger operator** -d²/dx² + V(x) with
V(x) = (x/2π)log(x/2πe) has the correct WKB spectrum asymptotically
(the k-th eigenvalue satisfies (E_k/2π)log(E_k/2πe) ≈ k, the defining
equation of the k-th zeta zero). The operator is correct in principle.
Every finite-dimensional approximation of it has failed in a distinct way,
but the continuum limit remains the strongest candidate.

### The Selberg Connection (open question)

The scattering matrix of the Laplacian on PSL(2,Z)\H contains ζ(2s−1)/ζ(2s).
The zeta zeros ARE resonances of this operator — a theorem, not a conjecture.
The challenge is constructing a computable discrete approximation whose
spectrum converges to the continuous spectrum. Our Cayley graph attempt failed
because finite-degree graphs have bounded spectra. An **expander graph family**
with **growing degree** (so the spectral range grows) might succeed.

## Repository Structure

```
hprime/
├── cmd/                        # 11 test programs (all reproducible)
│   ├── anthropic-validate/     # 62.97% vs 67.2% Anthropic bound
│   ├── convergence/            # Cauchy criterion (exact ratio 0.500/octave)
│   ├── genuine/                # Genuine prime-density construction
│   ├── heat-kernel/            # Pure tridiagonal trace (diverges)
│   ├── hybrid/                 # Hybrid operator (circular)
│   ├── hybrid-heat/            # Heat kernel convergence
│   ├── hybrid-2048/            # Large-N scaling
│   ├── nnn-coupling/           # Bandwidth independence
│   ├── schrodinger/            # WKB Schrödinger operator
│   ├── selfadjoint/            # Self-adjointness verification
│   └── spectral/               # Spectral correspondence + rates
├── circular/                   # Archived circular proofs (with disclaimer)
├── genuine-construction.pdf    # Non-circular paper (documents small-N artifact)
├── paper.pdf                   # Original main paper
├── selberg-connection.md       # Selberg trace formula analysis
├── psl2-results.md             # Cayley graph investigation (with ARPACK correction)
├── response-plan.md            # Response to Grok's circularity analysis
├── grok-round2-plan.md         # Anticipating second-round critiques
├── grok-round3-prediction.md   # Anticipating third-round critiques
├── PROGRESS.md                 # Full investigation progress report
└── README.md                   # This file
```

## Quick Start

```bash
cd hprime
go build -o bin/hybrid ./cmd/hybrid && ./bin/hybrid          # circular hybrid (|r|=1.000)
go build -o bin/genuine ./cmd/genuine && ./bin/genuine       # non-circular (small-N artifact)
go build -o bin/convergence ./cmd/convergence && ./bin/convergence  # Cauchy proof
go build -o bin/selfadjoint ./cmd/selfadjoint && ./bin/selfadjoint  # Self-adjointness
```

## Acknowledgments

- **xAI's Grok** identified the circularity in the hybrid operator construction
  and prompted the scaling analysis that revealed the small-N artifact
- **Claude Code** assisted with the computational infrastructure and mathematical proofs
- The **Anthropic research team's** July 2026 Riemann zeta bound motivated the
  initial investigation
- The **Hilbert-Pólya conjecture** (c. 1980) has been the guiding framework

## Author

**Rick Wesson** — CEO, Support Intelligence, Inc.
