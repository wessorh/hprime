// heat-kernel — tests whether the Hilbert plane operator's heat kernel
// trace matches the explicit formula's prediction:
//
//   Tr(exp(−t·H)) = Σ_k exp(−t·λ_k)  [operator side]
//   Σ_γ exp(−t·γ)                     [zeta zero side]
//
// If these converge as order n → ∞, the eigenvalues λ_k are the zeta zeros γ.
//
// The heat kernel trace can be computed WITHOUT diagonalizing H:
//   Tr(exp(−t·H)) ≈ Σ_{k=0}^{M} (−t)^k/k! · Tr(H^k)
//
// where Tr(H^k) is computable from matrix entries via cycle enumeration.
// For a tridiagonal matrix, Tr(H^k) counts closed walks of length k on
// the plane adjacency graph.

package main

import (
	"fmt"
	"math"
)

func main() {
	fmt.Println("=== Heat Kernel Trace vs. Explicit Formula ===")
	fmt.Println()

	zeros := allZeros()

	// Test at one order where we have enough zeros for comparison.
	n := 6
	N := 1 << n // 64 planes

	// Build the Hilbert tridiagonal operator.
	alpha := 1.0 / float64(N)
	diag := make([]float64, N)
	offdiag := make([]float64, N-1)
	for i := 0; i < N; i++ {
		diag[i] = 1.0
	}
	for i := 0; i < N-1; i++ {
		offdiag[i] = alpha
	}

	// ── Method 1: Exact heat kernel from eigenvalues ──────────────
	ev := tridiagEigenvalues(diag, offdiag)

	// ── Method 2: Heat kernel from trace of powers ───────────────
	// Tr(H^k) for tridiagonal matrix: sum of products along closed walks.
	// For a uniform tridiagonal: Tr(H^k) ≈ N · (1 + correction).
	// The correction comes from off-diagonal contributions.

	// ── Method 3: Zeta zero prediction ──────────────────────────
	// Σ_{k=1}^{N} exp(−t·γ_k) from known zeros.

	fmt.Printf("%8s %16s %16s %16s\n",
		"t", "Tr(exp(-tH))", "Σexp(-tγ)", "Δ")
	fmt.Println("──────── ──────────────── ──────────────── ────────────────")

	for _, t := range []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0} {
		// Operator side: exact from eigenvalues.
		opTrace := 0.0
		for k := 0; k < N; k++ {
			opTrace += math.Exp(-t * ev[k])
		}

		// Zeta zero side.
		zetaTrace := 0.0
		for k := 0; k < N; k++ {
			zetaTrace += math.Exp(-t * zeros[k])
		}

		delta := opTrace - zetaTrace
		fmt.Printf("%8.3f %16.6f %16.6f %+16.6f\n", t, opTrace, zetaTrace, delta)
	}

	// ── Sweep over orders ────────────────────────────────────────
	fmt.Println()
	fmt.Println("── Convergence with Order (t = 0.1) ──")
	fmt.Printf("%8s %16s %16s %16s\n",
		"Order", "Tr(exp(-tH))", "Σexp(-tγ)", "Ratio")
	fmt.Println("──────── ──────────────── ──────────────── ────────────────")

	t := 0.1
	for n := 4; n <= 10; n++ {
		N := 1 << n
		alpha := 1.0 / float64(N)
		diag := make([]float64, N)
		offdiag := make([]float64, N-1)
		for i := 0; i < N; i++ {
			diag[i] = 1.0
		}
		for i := 0; i < N-1; i++ {
			offdiag[i] = alpha
		}
		ev := tridiagEigenvalues(diag, offdiag)

		opTrace := 0.0
		for k := 0; k < N; k++ {
			opTrace += math.Exp(-t * ev[k])
		}

		zetaTrace := 0.0
		for k := 0; k < N; k++ {
			zetaTrace += math.Exp(-t * zeros[k])
		}

		ratio := opTrace / zetaTrace
		fmt.Printf("%8d %16.6f %16.6f %16.6f\n", n, opTrace, zetaTrace, ratio)
	}

	// ── Trace of powers: explicit computation for small k ──────────
	fmt.Println()
	fmt.Println("── Trace of Powers Tr(H^k) vs Matrix Prediction ──")
	fmt.Printf("%8s %16s %16s\n", "k", "Exact Tr(H^k)", "Tridiag formula")
	fmt.Println("──────── ──────────────── ────────────────")

	// Tr(H^1) = Σ diag[i] = N
	// Tr(H^2) = Σ diag[i]² + 2·Σ offdiag[i]²
	// Tr(H^3) = Σ diag[i]³ + 3·Σ diag[i]·offdiag[i]² + 3·Σ diag[i+1]·offdiag[i]² + ...
	// For uniform tridiagonal: closed form available.

	for k := 1; k <= 6; k++ {
		exact := tracePower(diag, offdiag, k)
		// Formula for uniform tridiagonal: Tr(H^k) ≈ N · Σ_{j} (k choose j,j,k-2j) · α^{2j}
		// where j counts off-diagonal steps (must come in pairs for closed walk).
		// For large N: Tr(H^k) → N · I_k(2α) where I_k is modified Bessel.
		approx := float64(N) * tracePowerApprox(k, 1.0/float64(N))
		fmt.Printf("%8d %16.6f %16.6f\n", k, exact, approx)
	}

	// ── Assessment ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("── Assessment ──")
	fmt.Println("  The heat kernel trace connects the operator spectrum to the")
	fmt.Println("  explicit formula. For the Hilbert tridiagonal operator at")
	fmt.Println("  order n, the trace Tr(exp(−tH)) should approach the zeta")
	fmt.Println("  zero sum Σexp(−tγ) as n → ∞ IF the eigenvalues converge to")
	fmt.Println("  the zeta zeros.")
	fmt.Println()
	fmt.Println("  Since the eigenvalues → 1 (identity), the operator trace")
	fmt.Println("  → N·exp(−t). The zeta zero trace → Σexp(−t·γ_k) which is")
	fmt.Println("  a completely different function of t. The traces do NOT")
	fmt.Println("  match because the limit is the identity operator.")
}

// tracePower computes Tr(H^k) exactly by summing over the matrix.
func tracePower(diag, offdiag []float64, k int) float64 {
	alpha := offdiag[0]
	N := len(diag)
	// Dynamic programming: dp[i][j][s] = sum of products for walks from i to j of length s.
	// Tr = Σ_i dp[i][i][k].
	// Memory: O(N²) for current and previous step.

	// Simplify: use the fact that H is tridiagonal and uniform.
	// H = I + α·A where A is the adjacency matrix (1 on off-diagonals).
	// H^k = Σ_{j=0}^k (k choose j) · α^j · A^j.
	// Tr(H^k) = Σ_{j even} (k choose j) · α^j · Tr(A^j).
	// For a path graph of length N: Tr(A^j) counts closed walks of length j.
	// Tr(A^0) = N, Tr(A^2) = 2(N-1), Tr(A^4) = ...

	sum := 0.0
	for j := 0; j <= k; j += 2 { // only even j contribute (closed walks on bipartite graph)
		binomial := choose(k, j)
		aTrace := traceAdjPower(N, j)
		sum += float64(binomial) * math.Pow(alpha, float64(j)) * float64(aTrace)
	}
	return sum
}

// traceAdjPower returns Tr(A^j) for the N-path adjacency matrix.
// A is tridiagonal with 1 on off-diagonals, 0 on diagonal.
func traceAdjPower(N, j int) int {
	if j == 0 {
		return N
	}
	if j%2 == 1 {
		return 0 // bipartite graph: no odd closed walks
	}
	// For a path graph: Tr(A^{2m}) = Σ_i (number of closed walks of length 2m from i).
	// Asymptotically for large N: ≈ N · Catalan(m).
	// Exact for path of length N:
	//   Tr(A^{2m}) = Σ_{k=1}^N λ_k^{2m} where λ_k = 2cos(kπ/(N+1)).
	eigenvalues := make([]float64, N)
	for k := 1; k <= N; k++ {
		eigenvalues[k-1] = 2 * math.Cos(float64(k)*math.Pi/float64(N+1))
	}
	sum := 0.0
	for _, lam := range eigenvalues {
		sum += math.Pow(lam, float64(j))
	}
	return int(sum + 0.5)
}

func tracePowerApprox(k int, a float64) float64 {
	// For large N, Tr(H^k)/N ≈ Σ_{j even} choose(k,j) · α^j · Catalan(j/2) · 2^j
	sum := 0.0
	for j := 0; j <= k; j += 2 {
		binomial := choose(k, j)
		cat := catalan(j / 2)
		sum += float64(binomial) * math.Pow(a, float64(j)) * float64(cat) * math.Pow(2, float64(j))
	}
	return sum
}

func choose(n, k int) int {
	if k < 0 || k > n {
		return 0
	}
	r := 1
	for i := 1; i <= k; i++ {
		r = r * (n - k + i) / i
	}
	return r
}

func catalan(n int) int {
	return choose(2*n, n) / (n + 1)
}

func tridiagEigenvalues(diag, offdiag []float64) []float64 {
	N := len(diag)
	sorted := make([]float64, N)
	copy(sorted, diag)
	eAvg := 0.0
	for _, v := range offdiag {
		eAvg += math.Abs(v)
	}
	if N > 1 {
		eAvg /= float64(N - 1)
	}
	ev := make([]float64, N)
	for k := 0; k < N; k++ {
		theta := math.Pi * float64(k+1) / float64(N+1)
		ev[k] = sorted[k] + 2*eAvg*math.Cos(theta)
	}
	// Sort.
	for i := 1; i < N; i++ {
		for j := i; j > 0 && ev[j] < ev[j-1]; j-- {
			ev[j], ev[j-1] = ev[j-1], ev[j]
		}
	}
	return ev
}

func allZeros() []float64 {
	exact := []float64{
		14.134725, 21.022040, 25.010857, 30.424876, 32.935062, 37.586178,
		40.918719, 43.327073, 48.005151, 49.773832, 52.970321, 56.446248,
		59.347044, 60.831779, 65.112544, 67.079811, 69.546402, 72.067158,
		75.704691, 77.144840, 79.337375, 82.910381, 84.735493, 87.425275,
		88.809111, 92.491899, 94.651344, 95.870634, 98.831194, 101.317851,
		103.725538, 105.446623, 107.168611, 111.029535, 111.874659, 114.320221,
		116.226680, 118.790783, 121.370125, 122.946829, 124.256819, 127.516684,
		129.578704, 131.087689, 133.497737, 134.756510, 138.116042, 139.736209,
		141.123707, 143.111846, 146.000982, 147.422765, 150.053520, 150.925258,
		153.024694, 156.112909, 157.597591, 158.849988, 161.188964, 163.030709,
		165.537069, 167.184440, 169.094515, 169.911976,
	}
	zeros := make([]float64, 2048)
	copy(zeros, exact)
	for k := len(exact); k < 2048; k++ {
		t := 2 * math.Pi * float64(k+1) / math.Log(float64(k+1))
		for iter := 0; iter < 3; iter++ {
			f := (t/(2*math.Pi))*math.Log(t/(2*math.Pi*math.E)) - float64(k+1)
			fp := math.Log(t/(2*math.Pi)) / (2 * math.Pi)
			t -= f / fp
		}
		zeros[k] = t
	}
	return zeros
}
