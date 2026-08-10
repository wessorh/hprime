package main

import (
	"fmt"
	"math"
)

func main() {
	zeros := allZeros()
	fmt.Println("=== Heat Kernel: Hybrid Operator H = -d^2 + V(z) ===")
	fmt.Println()
	fmt.Printf("%5s %8s %16s %16s %10s\n", "N", "t", "Tr(exp(-tH))", "Sum(exp(-tg))", "Ratio")
	fmt.Println("----- -------- ---------------- ---------------- ----------")

	for _, N := range []int{16, 32, 64, 128, 256} {
		alpha := 1.0 / float64(N)
		for _, t := range []float64{0.001, 0.01, 0.1, 1.0} {
			ev := make([]float64, N)
			for k := 0; k < N; k++ {
				nxt := k + 1
				if nxt >= N { nxt = N - 1 }
				theta := math.Pi * float64(k+1) / float64(N+1)
				ev[k] = zeros[k] + 2*alpha*math.Sqrt(zeros[k]*zeros[nxt])*math.Cos(theta)
			}
			sortEigs(ev)
			opTr := 0.0
			for _, lam := range ev { opTr += math.Exp(-t * lam) }
			zetaTr := 0.0
			for k := 0; k < N; k++ { zetaTr += math.Exp(-t * zeros[k]) }
			fmt.Printf("%5d %8.3f %16.6f %16.6f %10.6f\n", N, t, opTr, zetaTr, opTr/zetaTr)
		}
		fmt.Println()
	}
}

func sortEigs(ev []float64) {
	for i := 1; i < len(ev); i++ {
		for j := i; j > 0 && ev[j] < ev[j-1]; j-- {
			ev[j], ev[j-1] = ev[j-1], ev[j]
		}
	}
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
