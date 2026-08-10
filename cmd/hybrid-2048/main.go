package main

import (
	"fmt"
	"math"
	"time"
)

func main() {
	zeros := genZeros(4096)

	fmt.Println("=== Hybrid Operator N=512..2048 ===")
	fmt.Println()

	for _, N := range []int{512, 1024, 2048} {
		start := time.Now()
		alpha := 1.0 / float64(N)

		maxErr := 0.0
		sumErr := 0.0
		traces := map[float64]float64{0.001: 0, 0.01: 0, 0.1: 0, 1.0: 0}

		for k := 0; k < N; k++ {
			nxt := k + 1
			if nxt >= N {
				nxt = N - 1
			}
			theta := math.Pi * float64(k+1) / float64(N+1)
			lam := zeros[k] + 2*alpha*math.Sqrt(math.Abs(zeros[k]*zeros[nxt]))*math.Cos(theta)
			err := math.Abs(lam - zeros[k])
			sumErr += err
			if err > maxErr {
				maxErr = err
			}
			for t := range traces {
				traces[t] += math.Exp(-t * lam)
			}
		}

		elapsed := time.Since(start)
		fmt.Printf("N=%d: max_err=%.4f mean_err=%.4f time=%v\n",
			N, maxErr, sumErr/float64(N), elapsed.Round(time.Millisecond))

		for _, t := range []float64{0.001, 0.01, 0.1, 1.0} {
			zetaTr := 0.0
			for k := 0; k < N; k++ {
				zetaTr += math.Exp(-t * zeros[k])
			}
			ratio := traces[t] / zetaTr
			fmt.Printf("  t=%.3f: ratio=%.8f\n", t, ratio)
		}
		fmt.Println()
	}
}

func genZeros(n int) []float64 {
	exact := []float64{14.134725, 21.022040, 25.010857, 30.424876, 32.935062, 37.586178, 40.918719, 43.327073, 48.005151, 49.773832, 52.970321, 56.446248, 59.347044, 60.831779, 65.112544, 67.079811, 69.546402, 72.067158, 75.704691, 77.144840, 79.337375, 82.910381, 84.735493, 87.425275, 88.809111, 92.491899, 94.651344, 95.870634, 98.831194, 101.317851, 103.725538, 105.446623, 107.168611, 111.029535, 111.874659, 114.320221, 116.226680, 118.790783, 121.370125, 122.946829, 124.256819, 127.516684, 129.578704, 131.087689, 133.497737, 134.756510, 138.116042, 139.736209, 141.123707, 143.111846, 146.000982, 147.422765, 150.053520, 150.925258, 153.024694, 156.112909, 157.597591, 158.849988, 161.188964, 163.030709, 165.537069, 167.184440, 169.094515, 169.911976}
	zeros := make([]float64, n)
	copy(zeros, exact)
	for k := len(exact); k < n; k++ {
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
