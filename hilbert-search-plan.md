# Optimizing 3D Hilbert Curve Construction for Zeta Zero Spectral Correspondence

## Goal

Find a 3D Hilbert curve construction that maximizes r (Pearson correlation between
eigenvalue spectrum and zeta zeros), ideally converging toward r → −1.0 as
order n increases.

## Search Space

A 3D Hilbert curve at order n is defined by:

1. **Base pattern** (1 of 6): the octant traversal order of the 2×2×2 cube
   - A: {0,1,3,2,6,7,5,4} (standard)
   - B: {0,2,6,4,5,7,3,1}
   - C: {0,2,3,1,5,7,6,4}
   - D: {0,4,6,2,3,7,5,1}
   - E: {0,4,5,1,3,7,6,2}
   - F: {0,1,5,4,6,7,3,2}

2. **Per-level rotation** (1 of 24): applied at each recursion level 0..n-1
   - The standard construction uses the same rotation at every level
   - Different levels could use different rotations: 24^(n-1) possibilities

3. **Per-level reflection** (yes/no per level): whether to reflect coordinates
   globally at each recursion level — 2^(n-1) possibilities

**Total naive search space at order n:** 6 × 48^(n-1)
- Order 6: 6 × 48^5 ≈ 1.5 × 10^9 (intractable)
- Order 7: 6 × 48^6 ≈ 7.4 × 10^10 (impossible)

## Key Insight: Levels Are Not Independent

Computational evidence from order 6 shows:
- Varying the base pattern changes r by up to 4% (0.868 to 0.906)
- Per-level rotations that differ from the pattern default produce identical
  results to some other base pattern (rotations just relabel axes)
- The correlation is determined primarily by the BASE PATTERN, not the
  per-level rotation sequence

**Therefore:** The effective search space is much smaller — approximately the
6 base patterns, with per-level rotations providing at most fine-tuning.

## Phase 1: Exhaustive Pattern Search (Orders 5–8)

### Step 1.1: Rank all 6 base patterns at orders 5–8

For each pattern P ∈ {A,B,C,D,E,F}, compute r at orders 5, 6, 7, 8.
Identify which pattern maximizes r at each order and whether the
best pattern is consistent across orders.

**Effort:** 6 patterns × 4 orders = 24 runs. Already have 2 of 4 orders.

**Success criterion:** Find a pattern that improves on standard r = −0.904
at order 6 AND r = −0.783 at order 7.

### Step 1.2: Test pattern mixing

For the best 2 patterns from Step 1.1, test whether mixing them across
recursion levels helps. E.g., use pattern A at level 0, pattern E at
level 1, etc. This adds 2^(n-1) combinations per pair.

**Effort:** ~16 runs per order for 2-pattern mixing.

## Phase 2: Per-Level Rotation Optimization (Orders 6–7)

### Step 2.1: Single-level perturbation

Starting from the best base pattern, try all 48 symmetries at a single
recursion level (keeping other levels default). Measure the impact on r.
This identifies which recursion levels are most sensitive.

**Effort:** 48 × (n-1) ≈ 240 runs at order 6.

### Step 2.2: Greedy optimization

Starting from the best pattern:
1. For level 0, try all 48 symmetries — keep the best
2. For level 1, try all 48 — keep the best
3. Repeat for all levels
4. Iterate until convergence

**Effort:** ~48 × (n-1) × k iterations. At order 6 with k=3: ~720 runs.

## Phase 3: Gradient-Free Optimization (Order 6)

If Phase 2 shows that per-level choices matter, use a proper optimizer:

### Step 3.1: Define the objective

Each curve is a vector c = (base_pattern, s_0, s_1, ..., s_{n-1}) where
each s_i ∈ {0..47} selects one of 48 symmetries at recursion level i.

Objective: f(c) = |r| (absolute Pearson correlation)

### Step 3.2: Simulated annealing

Given the discrete search space:
1. Start from best-known curve c_0
2. At each step, randomly mutate one level's symmetry
3. Accept if f(c') > f(c); accept with probability exp(-Δ/T) otherwise
4. Cool T from T_0 to 0 over 1000 iterations

**Effort:** 1000 function evaluations ≈ 1000 × (order 6 runtime) ≈ 1000 ×
30s ≈ 8 hours. Feasible.

### Step 3.3: Genetic algorithm (Phase 2-3 overlap)

Population of 20 curves. Each generation:
1. Evaluate fitness f(c) for all curves
2. Select top 5
3. Crossover: swap symmetry choices between parents
4. Mutate: randomly change one level's symmetry
5. Repeat for 50 generations

**Effort:** 1000 evaluations ≈ same as simulated annealing.

## Phase 4: Scale the Best Curve (Orders 7–10)

Take the best curve found at order 6 and evaluate it at orders 7, 8, 9, 10.
Compare to the standard curve (pattern A at all levels).

**Key question:** Does the optimized curve show IMPROVING r with order
(converging toward −1.0) while the standard curve remains stable at −0.80?

If yes: **the optimized curve is a better approximation to the true
Hilbert-Pólya operator, and the optimization is converging on Ĥ.**

## Phase 5: Analytic Investigation

If an optimized curve is found with r significantly better than the standard:

1. **Characterize the curve**: What mathematical property distinguishes
   the optimized curve from the standard one? Look at the symmetry group,
   the octant visitation pattern at each level, the bit-manipulation rules.

2. **Prove optimality**: Can we prove that this particular construction
   maximizes the alignment between the Hilbert recursion and prime
   residue classes? The modular constraint (primes mod 8) may dictate
   a unique optimal traversal.

3. **Extrapolate to infinite order**: If the optimized r(n) follows a
   trend that approaches −1.0, extrapolate the limit and characterize
   the limiting operator.

## Resource Estimate

| Phase | Runs | Time (order 6) | Time (total) |
|-------|------|----------------|--------------|
| 1.1 | 24 | 30s each | ~12 min |
| 1.2 | 16 | 30s each | ~8 min |
| 2.1 | 240 | 30s each | ~2 hours |
| 2.2 | 720 | 30s each | ~6 hours |
| 3.2 | 1000 | 30s each | ~8 hours |
| 4 | 5 | varies | ~2 hours |

**Total (conservative):** ~18 hours on EPYC 7H12 (96 cores).

## Immediate Next Step

Run Phase 1.1: exhaustive pattern search at orders 6, 7, and 8.
This requires completing the data for all 6 patterns at orders 7 and 8
(only have variants 24/30 so far).
