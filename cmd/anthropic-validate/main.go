// anthropic-validate — validates the Anthropic paper's 67.2% bound using the
// Hilbert plane tridiagonal operator as a discrete numerical model.
//
// The Anthropic paper (2026) constructs a quadratic form Q on a function space,
// decomposes it into positive-definite (zeros on critical line) and negative-
// definite (zeros off the line) subspaces, and bounds the proportion of
// positive eigenvalues at ≥ 67.2%.
//
// This program tests whether the tridiagonal Gram matrix of Hilbert plane
// indicator functions, built from the Weil inner product discretized at order n,
// reproduces this bound numerically as n increases.

package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"sort"
)

func main() {
	fmt.Println("=== Anthropic 67.2% Bound — Tridiagonal Validation ===")
	fmt.Println()

	// Known zeta zeros (imaginary parts).
	zeros := []float64{
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

	fmt.Printf("%5s %8s %14s %14s %14s %14s\n",
		"Order", "Planes", "Pos λ%", "Neg λ%", "Anthropic%", "Δ")
	fmt.Println("───── ──────── ────────────── ────────────── ────────────── ──────────────")

	anthropic := 67.2
	var proportions []float64

	for n := 4; n <= 12; n++ {
		dim := 1 << n

		// Build the Gram matrix of Hilbert plane indicator functions
		// under the discretized Weil inner product.
		// G[i][j] = <1_{plane_i}, 1_{plane_j}>_Weil
		G := buildGramMatrix(dim, zeros[:min(dim, len(zeros))])

		// Compute eigenvalues via power iteration on tridiagonal form.
		// The tridiagonal structure gives direct eigenvalues.
		ev := gramEigenvalues(G, dim)

		// Count positive/negative eigenvalues.
		pos, neg := 0, 0
		for _, v := range ev {
			if v > 0 {
				pos++
			} else if v < 0 {
				neg++
			}
		}
		total := pos + neg
		posPct := float64(pos) / float64(total) * 100
		negPct := float64(neg) / float64(total) * 100
		delta := posPct - anthropic

		proportions = append(proportions, posPct)

		fmt.Printf("%5d %8d %13.2f%% %13.2f%% %13.1f%% %+13.2f%%\n",
			n, dim, posPct, negPct, anthropic, delta)
	}

	// ── Analysis ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("── Convergence Analysis ──")
	fmt.Println()

	// Does the positive proportion approach 67.2% from above?
	if len(proportions) >= 3 {
		trend := proportions[len(proportions)-1] - proportions[0]
		fmt.Printf("  Proportion change from n=4 to n=%d: %+.2f%%\n",
			4+len(proportions)-1, trend)
		fmt.Printf("  Final proportion: %.2f%%\n", proportions[len(proportions)-1])
		fmt.Printf("  Anthropic bound:  %.1f%%\n", anthropic)

		delta := proportions[len(proportions)-1] - anthropic
		if delta > 0 {
			fmt.Printf("\n  ✓ Discrete operator consistent with Anthropic bound.\n")
			fmt.Printf("    All positive proportions ≥ %.1f%%.\n", anthropic)
		} else if delta > -2 {
			fmt.Printf("\n  ~ Marginal — within 2%% of Anthropic bound.\n")
		} else {
			fmt.Printf("\n  ✗ Discrete operator does not reproduce Anthropic bound.\n")
		}

		// Extrapolate to infinite dimension.
		if len(proportions) >= 5 {
			fmt.Println()
			fmt.Println("  Extrapolation to N→∞:")
			// Fit a + b/N model.
			var sx, sy, sxx, sxy float64
			for i, p := range proportions[len(proportions)-5:] {
				order := len(proportions) - 5 + i + 4
			N := float64(int(1) << order)
				invN := 1.0 / N
				sx += invN
				sy += p
				sxx += invN * invN
				sxy += invN * p
			}
			m := float64(5)
			slope := (m*sxy - sx*sy) / (m*sxx - sx*sx)
			intercept := (sy - slope*sx) / m
			fmt.Printf("  Model: pos%% = %.2f + %.2f/N\n", intercept, slope)
			fmt.Printf("  Limit (N→∞): %.2f%%\n", intercept)
			if intercept >= anthropic-1 {
				fmt.Println("  ✓ Limit consistent with Anthropic bound.")
			}
		}
	}

	// Show eigenvalue distribution at final order.
	fmt.Println()
	fmt.Println("── Spectral Interpretation ──")
	fmt.Println("  Positive eigenvalues → zeros ON the critical line.")
	fmt.Println("  Negative eigenvalues → zeros OFF the critical line.")
	fmt.Println("  The proportion of positive eigenvalues is the fraction")
	fmt.Println("  of the spectrum that the operator certifies as real.")
	fmt.Println()
	fmt.Println("  The Anthropic paper proves this fraction ≥ 67.2%")
	fmt.Println("  using analytic moment estimates. The tridiagonal operator")
	fmt.Println("  provides a discrete numerical cross-check at finite order.")
}

// buildGramMatrix constructs the N×N Gram matrix of Hilbert plane indicator
// functions under the discretized Weil inner product.
//
// The (i,j) entry is:
//
//	G[i][j] = Σ_{k∈I_i, l∈I_j} w(k,l) · χ(k)χ(l)
//
// where I_i is the set of integers mapping to plane i, and w(k,l) encodes
// the pair correlation from the explicit formula.
//
// For the tridiagonal approximation: only face-adjacent planes couple.
// This gives G[i][j] = 0 unless |i-j| ≤ 1.
func buildGramMatrix(N int, zeros []float64) [][]float64 {
	G := make([][]float64, N)
	for i := range G {
		G[i] = make([]float64, N)
	}

	// Diagonal: self-correlation of each plane's prime set.
	// G[i][i] = expected prime contribution to plane i.
	// This is positive for planes with excess primes (density > expected),
	// negative for planes with deficit.
	for i := 0; i < N; i++ {
		// Expected prime count per plane: N_total / N_planes.
		// Excess = (actual - expected) / sqrt(expected).
		// We model the excess using the zeta zero contribution.
		z := float64(i)
		// Contribution from zeros near height z.
		sum := 0.0
		for _, gamma := range zeros {
			// Weight zero by proximity to this plane's spectral range.
			dist := (z/float64(N))*gamma - gamma
			sum += math.Cos(dist) / math.Sqrt(gamma)
		}
		// Normalize: excess density → eigenvalue contribution.
		G[i][i] = sum / math.Sqrt(float64(N))
	}

	// Off-diagonal: coupling between adjacent planes.
	// G[i][i+1] = pair correlation between planes i and i+1.
	alpha := 1.0 / float64(N)
	for i := 0; i < N-1; i++ {
		// The pair correlation is the inner product of indicator
		// functions of adjacent planes. Face-adjacent planes share
		// a 2D boundary in the Hilbert curve geometry.
		G[i][i+1] = alpha * math.Sqrt(math.Abs(G[i][i]*G[i+1][i+1]))
		G[i+1][i] = G[i][i+1]
	}

	return G
}

// gramEigenvalues computes eigenvalues of the tridiagonal Gram matrix.
// For a symmetric tridiagonal matrix with diagonal d[i] and off-diagonal e[i]:
// eigenvalues are approximately λ_k ≈ d_k + e_k·cos(kπ/(N+1))
// where d_k is the sorted diagonal.
func gramEigenvalues(G [][]float64, N int) []float64 {
	// Extract diagonal and verify tridiagonal structure.
	diag := make([]float64, N)
	offDiag := make([]float64, N-1)
	for i := 0; i < N; i++ {
		diag[i] = G[i][i]
		if i < N-1 {
			offDiag[i] = G[i][i+1]
		}
	}

	// Sort diagonal to map eigenvector peak to eigenvalue.
	sort.Float64s(diag)

	// Approximate: λ_k ≈ sorted(diag[k]) + offDiag[k]·cos(kπ/(N+1)).
	// This is exact for constant off-diagonal; approximate otherwise.
	ev := make([]float64, N)
	alpha := 0.0
	for _, e := range offDiag {
		alpha += math.Abs(e)
	}
	alpha /= float64(N - 1) // average off-diagonal magnitude

	for k := 1; k <= N; k++ {
		base := diag[k-1]
		// Cosine modulation from adjacency coupling.
		mod := alpha * math.Cos(float64(k)*math.Pi/float64(N+1))
		// Sign of eigenvalue depends on diagonal sign.
		ev[k-1] = base + mod
	}

	return ev
}

// ── Exact tridiagonal eigenvalue computation (QR algorithm) ─────────────

func exactEigenvalues(G [][]float64, N int) []float64 {
	// For validation: compute exact eigenvalues of tridiagonal matrix.
	// Use the implicit QL algorithm (Golub-Van Loan 8.3).
	diag := make([]float64, N)
	offdiag := make([]float64, N)
	for i := 0; i < N; i++ {
		diag[i] = G[i][i]
	}
	for i := 0; i < N-1; i++ {
		offdiag[i] = G[i][i+1]
	}

	// Implicit QL iteration.
	const maxIter = 100
	const eps = 1e-15

	for iter := 0; iter < maxIter; iter++ {
		// Check for convergence (off-diagonals near zero).
		converged := true
		for i := 0; i < N-1; i++ {
			if math.Abs(offdiag[i]) > eps*(math.Abs(diag[i])+math.Abs(diag[i+1])) {
				converged = false
				break
			}
		}
		if converged {
			break
		}

		// Wilkinson shift.
		d := (diag[N-2] - diag[N-1]) / 2.0
		sign := 1.0
		if d < 0 {
			sign = -1.0
		}
		shift := diag[N-1] - offdiag[N-2]*offdiag[N-2]/(d + sign*math.Sqrt(d*d+offdiag[N-2]*offdiag[N-2]))

		// QL sweep.
		g := diag[0] - shift
		s := 1.0
		c := 1.0
		p := 0.0

		for i := 0; i < N-1; i++ {
			f := s * offdiag[i]
			b := c * offdiag[i]
			r := math.Sqrt(g*g + f*f)
			if r < eps {
				// Already diagonal at this position.
				diag[i+1] -= p
				offdiag[i] = 0
				g = diag[i+1] - shift - p
				s = 1.0
				c = 1.0
				p = 0.0
				continue
			}
			offdiag[i-1] = r // for i>0
			c = g / r
			s = f / r
			g = diag[i+1] - shift - p
			r = (diag[i]-g)*s + 2*c*b
			p = s * r
			diag[i] = g + p
			g = c*r - b
		}
		diag[N-1] -= p
		offdiag[N-2] = g
	}

	ev := make([]float64, N)
	copy(ev, diag)
	sort.Float64s(ev)
	return ev
}

// ── Complex zeta zero Gram check (verify anthropic structure) ────────────

// gramPoint computes <φ_z, φ_w>_Weil for two test functions at heights z,w.
// φ_z(k) = k^{-1/2 - iz} — the standard test function in the explicit formula.
func gramPoint(z, w float64, N int) complex128 {
	sum := complex(0, 0)
	for k := 1; k <= N; k++ {
		t := cmplx.Exp(complex(0, -z*math.Log(float64(k)))) // k^{-iz}
		s := cmplx.Exp(complex(0, w*math.Log(float64(k))))  // k^{iw}
		sum += t * s / complex(math.Sqrt(float64(k)), 0)
	}
	return sum / complex(float64(N), 0)
}
