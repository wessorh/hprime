# Twelve Approaches to RH — Systematic Investigation Plan

Strategy: test each approach, verify failure mode, document what was learned, move on.

## Active

### #10 — Weighted Hilbert Curves with Growing Degree [IN PROGRESS]
**Hypothesis:** A sequence of d_n-regular graphs where d_n grows with n can produce an
unbounded spectrum, escaping the Perron-Frobenius bound that killed PSL(2,Z/NZ).
**Failure criterion:** Spectrum still bounded relative to degree, or correlation degrades.

## Queue (test each, document failure, move on)

### #1 — Li Criterion (Lagarias, 1997)
**Hypothesis:** RH ⇔ λ_n ≥ 0 for all n. Compute first N Li coefficients numerically
and check positivity.
**Failure criterion:** Computation infeasible, or positivity unverifiable.

### #2 — De Branges' Hilbert Space
**Hypothesis:** Canonical system of differential equations produces zeta zero spectrum.
**Failure criterion:** Gap between claimed proof and verifiable computation.

### #3 — Trace Formula on Adèle Spaces (Connes, 1998)
**Hypothesis:** RH ⇔ positivity of global trace formula.
**Failure criterion:** Too abstract to verify computationally.

### #4 — Quantum Graphs
**Hypothesis:** Graph whose length spectrum matches prime powers → trace formula → RH.
**Failure criterion:** No explicit construction of the specific graph.

### #5 — Turing Verification
**Hypothesis:** Verify RH computationally up to height T.
**Failure criterion:** Can't prove anything, only verify.

### #6 — Keating–Snaith Moments
**Hypothesis:** Prove moment conjecture → RH.
**Failure criterion:** Moment conjecture is itself unproven.

### #7 — Supersymmetric QM
**Hypothesis:** Superpotential W(x) such that partner Hamiltonians have zeta zero spectra.
**Failure criterion:** No known construction of W(x).

### #8 — Fourth Moment Theorem (Nualart–Peccati)
**Hypothesis:** Gaussian fluctuations → bounds on zeros.
**Failure criterion:** Only proves distribution, not location.

### #9 — Spectral Triples (Connes–Moscovici)
**Hypothesis:** Dirac operator on spectral triple → RH.
**Failure criterion:** Too abstract to construct explicitly.

### #11 — Exact WKB / Resurgence
**Hypothesis:** Voros-Écalle resurgent WKB → exact spectrum from semiclassical.
**Failure criterion:** Resurgence coefficients computable but convergence unproven at finite N.

### #12 — Selberg Trace on Computable Hyperbolic Surface
**Hypothesis:** Modular curve X_0(N) with Hecke operators → growing degree → unbounded spectrum.
**Failure criterion:** Construction of exact 1-skeleton computationally infeasible.
