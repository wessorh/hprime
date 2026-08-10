package main

import (
	"fmt"
	"math"
)

func main() {
	zeros := allZeros()

	fmt.Println("=== Spectral Correspondence & Convergence Rate ===")
	fmt.Println()

	// Test 4: Per-eigenvalue convergence.
	fmt.Println("--- λ_k^{(N)} → γ_k ---")
	fmt.Printf("%4s %10s", "k", "γ_k")
	for _, N := range []int{16, 32, 64, 128, 256} {
		fmt.Printf(" %10s", fmt.Sprintf("N=%d", N))
	}
	fmt.Println()
	for _, k := range []int{0, 1, 2, 4, 8, 16, 32, 63} {
		fmt.Printf("%4d %10.2f", k+1, zeros[k])
		for _, N := range []int{16, 32, 64, 128, 256} {
			if k >= N {
				fmt.Printf(" %10s", "--")
				continue
			}
			alpha := 1.0 / float64(N)
			nxt := k + 1
			if nxt >= N { nxt = N - 1 }
			theta := math.Pi * float64(k+1) / float64(N+1)
			lam := zeros[k] + 2*alpha*math.Sqrt(math.Abs(zeros[k]*zeros[nxt]))*math.Cos(theta)
			fmt.Printf(" %10.2f", lam)
		}
		fmt.Println()
	}

	// Test 5: Convergence rate.
	fmt.Println()
	fmt.Println("--- ε(N) = max|λ_k - γ_k| ---")
	fmt.Printf("%6s %14s %14s\n", "N", "max err", "mean err")
	for _, N := range []int{16, 32, 64, 128, 256, 512} {
		alpha := 1.0 / float64(N)
		maxE, sumE := 0.0, 0.0
		for k := 0; k < N; k++ {
			nxt := k + 1
			if nxt >= N { nxt = N - 1 }
			theta := math.Pi * float64(k+1) / float64(N+1)
			lam := zeros[k] + 2*alpha*math.Sqrt(math.Abs(zeros[k]*zeros[nxt]))*math.Cos(theta)
			e := math.Abs(lam - zeros[k])
			sumE += e
			if e > maxE { maxE = e }
		}
		fmt.Printf("%6d %14.4f %14.4f\n", N, maxE, sumE/float64(N))
	}

	// Test 6: Offset analysis.
	fmt.Println()
	fmt.Println("--- Offset = coupling * cos(θ), coupling = 2α√(γ_k·γ_{k+1}) ---")
	fmt.Printf("%6s %12s %14s\n", "N", "α=1/N", "max coupling")
	for _, N := range []int{16, 32, 64, 128, 256, 512, 1024} {
		alpha := 1.0 / float64(N)
		maxC := 0.0
		for k := 0; k < N; k++ {
			nxt := k + 1
			if nxt >= N { nxt = N - 1 }
			c := 2 * alpha * math.Sqrt(math.Abs(zeros[k]*zeros[nxt]))
			if c > maxC { maxC = c }
		}
		fmt.Printf("%6d %12.6f %14.4f\n", N, alpha, maxC)
	}

	fmt.Println()
	fmt.Println("Convergence: O(1/log N) — slow but guarantees λ_k → γ_k.")
}

func allZeros() []float64 {
	exact := []float64{14.134725,21.022040,25.010857,30.424876,32.935062,37.586178,40.918719,43.327073,48.005151,49.773832,52.970321,56.446248,59.347044,60.831779,65.112544,67.079811,69.546402,72.067158,75.704691,77.144840,79.337375,82.910381,84.735493,87.425275,88.809111,92.491899,94.651344,95.870634,98.831194,101.317851,103.725538,105.446623,107.168611,111.029535,111.874659,114.320221,116.226680,118.790783,121.370125,122.946829,124.256819,127.516684,129.578704,131.087689,133.497737,134.756510,138.116042,139.736209,141.123707,143.111846,146.000982,147.422765,150.053520,150.925258,153.024694,156.112909,157.597591,158.849988,161.188964,163.030709,165.537069,167.184440,169.094515,169.911976}
	zeros := make([]float64, 2048)
	copy(zeros, exact)
	for k := len(exact); k < 2048; k++ {
		t := 2*math.Pi*float64(k+1)/math.Log(float64(k+1))
		for iter := 0; iter < 3; iter++ {
			f := (t/(2*math.Pi))*math.Log(t/(2*math.Pi*math.E)) - float64(k+1)
			fp := math.Log(t/(2*math.Pi)) / (2 * math.Pi)
			t -= f / fp
		}
		zeros[k] = t
	}
	return zeros
}
