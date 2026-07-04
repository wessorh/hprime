// ndhprime — primes on N-dimensional Hilbert curves
//
// Computes the Z-plane covariance matrix for arbitrary dimension D.
// Usage:
//   ndhprime -d 5 -n 3    # 5D Hilbert, order 3 (32^3 = 32768 cells)
//   ndhprime -d 6 -n 4    # 6D Hilbert, order 4 (64^4 = 16M cells)

package main

import (
	"flag"
	"fmt"
	"math/bits"
	"os"
	"runtime"
	"sync"
)

// ─── N-dimensional Butz Hilbert algorithm ──────────────────────────────

func gc(i uint32) uint32 { return i ^ (i >> 1) }

func gcInv(g, D uint32) uint32 {
	i := g
	for j := uint32(1); j < D; j++ {
		i ^= g >> j
	}
	return i
}

func gFunc(i uint32) uint32 {
	if i == 0 {
		return 0
	}
	return uint32(bits.TrailingZeros32(i))
}

func dmapND(i, D uint32) uint32 {
	if i == 0 {
		return 0
	}
	if i&1 == 0 {
		return gFunc(i-1) % D
	}
	return gFunc(i) % D
}

func emapND(i uint32) uint32 {
	if i == 0 {
		return 0
	}
	return gc(2 * ((i - 1) / 2))
}

func maxMask(D uint32) uint32 {
	if D >= 32 {
		return 0xFFFFFFFF
	}
	return (uint32(1) << D) - 1
}

func rotRight(b, i, D uint32) uint32 {
	i %= D
	return (b >> i) | ((b << (D - i)) & maxMask(D))
}

func rotLeft(b, i, D uint32) uint32 {
	i %= D
	return ((b << i) | (b >> (D - i))) & maxMask(D)
}

func transform(b, e, d, D uint32) uint32 {
	return rotRight(b^e, d+1, D)
}

func transformInv(b, e, d, D uint32) uint32 {
	return rotLeft(b, d+1, D) ^ e
}

// hilbertToPoint maps Hilbert index h to D-dimensional coordinates at given order.
func hilbertToPoint(D uint32, h uint64, order uint32) []uint32 {
	e := uint32(0)
	d := uint32(0)
	coords := make([]uint32, D)

	for i := int(order) - 1; i >= 0; i-- {
		shift := i * int(D)
		var wBits uint32
		for k := uint32(0); k < D; k++ {
			wBits |= uint32((h>>(shift+int(k)))&1) << k
		}

		l := transformInv(gc(wBits), e, d, D)

		for k := uint32(0); k < D; k++ {
			coords[k] = (coords[k] << 1) | ((l >> k) & 1)
		}

		e = e ^ rotLeft(emapND(wBits), d+1, D)
		d = (d + dmapND(wBits, D) + 1) % D
	}
	return coords
}

// pointToHilbert maps D-dimensional coordinates back to Hilbert index.
func pointToHilbert(D uint32, coords []uint32, order uint32) uint64 {
	e := uint32(0)
	d := uint32(0)
	h := uint64(0)

	for i := int(order) - 1; i >= 0; i-- {
		var l uint32
		for k := uint32(0); k < D; k++ {
			l |= ((coords[k] >> i) & 1) << k
		}
		wBits := gcInv(transform(l, e, d, D), D)

		shift := i * int(D)
		for k := uint32(0); k < D; k++ {
			h |= uint64((wBits>>k)&1) << (shift + int(k))
		}

		e = e ^ rotLeft(emapND(wBits), d+1, D)
		d = (d + dmapND(wBits, D) + 1) % D
	}
	return h
}

// ─── Prime sieve ────────────────────────────────────────────────────────

func parallelSieve(limit uint64) []uint64 {
	if limit < 2 {
		return nil
	}

	// Segmented sieve
	segmentSize := uint64(1 << 20) // 1M per segment
	composite := make([]bool, segmentSize)

	var primes []uint64
	primes = append(primes, 2)

	// Small primes up to sqrt(limit) for sieving
	sqrtLimit := uint64(1)
	for sqrtLimit*sqrtLimit <= limit {
		sqrtLimit++
	}
	smallPrimes := simpleSieve(sqrtLimit)

	for low := uint64(3); low < limit; low += segmentSize {
		high := low + segmentSize
		if high > limit {
			high = limit
		}

		// Reset composite array
		for i := range composite {
			composite[i] = false
		}

		for _, p := range smallPrimes {
			if p == 2 {
				continue
			}
			start := low / p * p
			if start < low {
				start += p
			}
			// Never start below p*p: p itself (and its multiples below
			// p*p) are not composite w.r.t. p, and smaller prime factors
			// already sieved them if they were composite. Without this,
			// the first segment (low=3) marks every small prime as its
			// own trivial multiple, wrongly excluding it from the output.
			if start < p*p {
				start = p * p
			}
			if start%2 == 0 {
				start += p
			}
			for j := start; j < high; j += 2 * p {
				composite[j-low] = true
			}
		}

		for j := low | 1; j < high; j += 2 {
			if !composite[j-low] {
				primes = append(primes, j)
			}
		}
	}

	return primes
}

func simpleSieve(limit uint64) []uint64 {
	composite := make([]bool, limit+1)
	var primes []uint64
	for i := uint64(2); i <= limit; i++ {
		if !composite[i] {
			primes = append(primes, i)
			for j := i * i; j <= limit; j += i {
				composite[j] = true
			}
		}
	}
	return primes
}

// ─── ND Curve-Adjacent Covariance ────────────────────────────────────────

func comma(last bool) string {
	if last {
		return ""
	}
	return ","
}

// buildNDZCurve returns Z-plane assignments for an ND Hilbert curve.
func buildNDZCurve(D, order uint32) []uint32 {
	total := uint64(1) << (D * order)
	curve := make([]uint32, total)

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	chunkSize := (int(total) + workers - 1) / workers
	if chunkSize < 1 {
		chunkSize = 1
	}
	var wg sync.WaitGroup
	fmt.Fprintf(os.Stderr, "  Building Z-curve for %d cells with %d workers...\n", total, workers)
	for start := 0; start < int(total); start += chunkSize {
		end := start + chunkSize
		if end > int(total) {
			end = int(total)
		}
		wg.Add(1)
		lo, hi := start, end
		go func() {
			for d := lo; d < hi; d++ {
				coords := hilbertToPoint(D, uint64(d), order)
				// Z is the 3rd coordinate (index 2)
				curve[d] = coords[2]
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return curve
}

func outputMatrixND(primes []uint64, D, order uint32) {
	dim := int(1 << order)
	total := uint64(1) << (D * order)
	fmt.Fprintf(os.Stderr, "%dD order %d: %dx%d matrix, %d primes, %.1fB cells\n",
		D, order, dim, dim, len(primes), float64(total))

	// Build prime bitset
	bitset := make([]uint64, (total+63)/64)
	for _, p := range primes {
		if p < total {
			bitset[p/64] |= 1 << (p % 64)
		}
	}
	isPrime := func(k uint64) bool {
		return bitset[k/64]&(1<<(k%64)) != 0
	}

	// Pass 1: compute plane sizes and means
	planeSize := make([]int, dim)
	mu := make([]float64, dim)

	curve := buildNDZCurve(D, order)
	fmt.Fprintf(os.Stderr, "  Pass 1: plane sizes...\n")
	for k := uint64(0); k < total; k++ {
		z := int(curve[k])
		planeSize[z]++
		if isPrime(k) && k > 1 {
			mu[z]++
		}
	}
	for z := 0; z < dim; z++ {
		if planeSize[z] > 0 {
			mu[z] /= float64(planeSize[z])
		}
	}

	// Build covariance matrix from curve-adjacent pairs
	cov := make([][]float64, dim)
	for i := range cov {
		cov[i] = make([]float64, dim)
	}
	counts := make([][]int, dim)
	for i := range counts {
		counts[i] = make([]int, dim)
	}

	fmt.Fprintf(os.Stderr, "  Pass 2: covariance from %d pairs...\n", total-1)
	progressStep := total / 20
	for k := uint64(0); k < total-1; k++ {
		if k%progressStep == 0 {
			pct := k * 100 / total
			fmt.Fprintf(os.Stderr, "    %d%%\n", pct)
		}
		z1 := int(curve[k])
		z2 := int(curve[k+1])

		i1 := 0.0
		if isPrime(k) {
			i1 = 1.0
		}
		i2 := 0.0
		if isPrime(k+1) {
			i2 = 1.0
		}

		cov[z1][z2] += (i1 - mu[z1]) * (i2 - mu[z2])
		counts[z1][z2]++
	}

	// Normalize and symmetrize
	for z1 := 0; z1 < dim; z1++ {
		for z2 := 0; z2 < dim; z2++ {
			if counts[z1][z2] > 0 {
				cov[z1][z2] /= float64(counts[z1][z2])
			}
		}
	}
	for z1 := 0; z1 < dim; z1++ {
		for z2 := z1 + 1; z2 < dim; z2++ {
			avg := (cov[z1][z2] + cov[z2][z1]) / 2.0
			cov[z1][z2] = avg
			cov[z2][z1] = avg
		}
	}

	// JSON output
	fmt.Println("{")
	fmt.Printf("  \"order\": %d,\n", order)
	fmt.Printf("  \"dim\": %d,\n", dim)
	fmt.Printf("  \"D\": %d,\n", D)
	fmt.Printf("  \"primes\": %d,\n", len(primes))
	fmt.Printf("  \"hilbert\": \"%dD\",\n", D)
	fmt.Println("  \"mu\": [")
	for z := 0; z < dim; z++ {
		fmt.Printf("    %.10f%s\n", mu[z], comma(z == dim-1))
	}
	fmt.Println("  ],")
	fmt.Println("  \"covariance\": [")
	for z1 := 0; z1 < dim; z1++ {
		fmt.Print("    [")
		for z2 := 0; z2 < dim; z2++ {
			fmt.Printf("%.12f%s", cov[z1][z2], comma(z2 == dim-1))
		}
		fmt.Printf("]%s\n", comma(z1 == dim-1))
	}
	fmt.Println("  ]")
	fmt.Println("}")
}

func pow64(a, b int) uint64 {
	result := uint64(1)
	for i := 0; i < b; i++ {
		result *= uint64(a)
	}
	return result
}

func main() {
	D := flag.Int("d", 5, "Hilbert curve dimension (3-6)")
	n := flag.Int("n", 3, "Hilbert curve order")
	flag.Parse()

	ND := uint32(*D)
	order := uint32(*n)

	base := uint64(1) << ND // 2^D
	limit := pow64(int(base), int(order))

	fmt.Fprintf(os.Stderr, "=== ndhprime — %dD Hilbert curve, order %d ===\n", ND, order)
	fmt.Fprintf(os.Stderr, "Cells: %d^%d = %d\n", base, order, limit)
	fmt.Fprintf(os.Stderr, "Z-plane dimension: %d\n", 1<<order)

	// Compute primes
	fmt.Fprintf(os.Stderr, "Sieving primes < %d...\n", limit)
	primes := parallelSieve(limit)
	fmt.Fprintf(os.Stderr, "Found %d primes (%.4f%%)\n", len(primes),
		float64(len(primes))/float64(limit)*100)

	outputMatrixND(primes, ND, order)
}
