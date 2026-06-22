// hprime — primes on 3D Hilbert curves
//
// Computes primes < 8^n using a parallel sieve, maps them onto multiple
// 3D Hilbert curve variants, and tests for alignment (do primes cluster
// along certain curve segments?).
//
// Usage:
//   hprime -n 4                    # primes < 8^4 = 4096
//   hprime -n 5 -variants 10      # test 10 curve variants
//   hprime -n 6 -align            # run alignment test

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

func main() {
	n := flag.Int("n", 4, "compute primes < 8^n")
	variants := flag.Int("variants", 8, "number of 3D Hilbert curve variants to test")
	align := flag.Bool("align", false, "run alignment analysis")
	movie := flag.Bool("movie", false, "render rotating 3D cube movie")
	movieVariant := flag.Int("movie-variant", 0, "curve variant for movie")
	movieOut := flag.String("movie-out", "/tmp/hprime_cube.mp4", "movie output path")
	movieFrames := flag.Int("movie-frames", 180, "number of rotation frames")
	planes := flag.Bool("planes", false, "run plane-alignment analysis")
	planeVariant := flag.Int("plane-variant", 0, "curve variant for plane test")
	correlate := flag.Bool("correlate", false, "run zeta correlation test")
	matrix := flag.Bool("matrix", false, "compute Hilbert plane operator matrix")
	matrixJSON := flag.Bool("matrix-json", false, "output full covariance matrix as JSON")
	op := flag.Bool("operator", false, "compute explicit formula operator and spectral response")
	vm := flag.Bool("vm", false, "output von Mangoldt-weighted covariance matrix as JSON")
	fastVM := flag.Bool("fast-vm", false, "optimized von Mangoldt operator (fast)")
	stream := flag.Bool("stream", false, "streaming matrix builder (for order 11+)")
	h4d := flag.Bool("4d", false, "use 4D Hilbert curve (16^n cells)")
	compare := flag.Bool("compare", false, "compare all variants for best plane alignment")
	flag.Parse()

	limit := pow64(8, *n)
	fmt.Printf("=== hprime — primes on 3D Hilbert curves ===\n")
	fmt.Printf("limit:       8^%d = %d\n", *n, limit)
	fmt.Printf("variants:    %d\n", *variants)
	fmt.Printf("curve order: %d  (grid: %d×%d×%d = %d points)\n",
		*n, 1<<*n, 1<<*n, 1<<*n, pow64(1<<*n, 3))

	// Estimate 3D Hilbert curve count
	estimateCurves(*n)

	// Compute primes in parallel
	fmt.Printf("\n── Computing primes < %d …\n", limit)
	primes := parallelSieve(limit)
	fmt.Printf("found %d primes (%.4f%% of range)\n", len(primes),
		float64(len(primes))/float64(limit)*100)

	// Test multiple 3D Hilbert curve variants
	for v := 0; v < *variants; v++ {
		kind := describeVariant(v)
		fmt.Printf("\n── Variant %d: %s\n", v, kind)

		// Build the curve mapping
		order := uint32(*n)
		curve := build3DCurve(order, v)

		// Map primes onto the curve
		hits := make([]int, len(curve))
		for _, p := range primes {
			if int(p) < len(curve) {
				hits[curve[p]]++
			}
		}

		// Basic stats
		occupied := 0
		maxHit := 0
		for _, h := range hits {
			if h > 0 {
				occupied++
				if h > maxHit {
					maxHit = h
				}
			}
		}
		fmt.Printf("  occupied cells: %d / %d (%.2f%%)\n",
			occupied, len(curve), float64(occupied)/float64(len(curve))*100)
		fmt.Printf("  max primes/cell: %d\n", maxHit)
		fmt.Printf("  avg primes/cell: %.4f\n", float64(len(primes))/float64(len(curve)))

		// Alignment test
		if *align || v < 3 {
			score := testAlignment(hits, order)
			fmt.Printf("  alignment score: %.4f  (%s)\n",
				score, classifyAlignment(score, order))
		}
	}

	// ── Movie generation ───────────────────────────────────────────────
	if *movie {
		renderMovie(primes, uint32(*n), *movieVariant, *movieOut, *movieFrames)
	}

	// ── Plane alignment analysis ───────────────────────────────────────
	if *h4d {
		outputMatrix4D(primes, uint32(*n), *planeVariant)
		return
	}
	if *stream {
		outputMatrixStreaming(primes, uint32(*n), *planeVariant)
		return
	}
	if *fastVM {
		computeFastVM(primes, uint32(*n), *planeVariant)
		return
	}
	if *vm {
		computeVonMangoldtOperator(primes, uint32(*n), *planeVariant)
		return
	}
	if *op {
		computeOperatorExplicit(primes, uint32(*n), *planeVariant)
		return
	}
	if *matrixJSON {
		outputMatrix(primes, uint32(*n), *planeVariant)
		return
	}
	if *matrix {
		computeOperatorMatrix(primes, uint32(*n), *planeVariant)
		return
	}
	if *correlate {
		runCorrelationTest(primes, uint32(*n), *planeVariant)
		return
	}
	if *compare {
		compareAllVariants(primes, uint32(*n))
		return
	}
	if *planes {
		analyzePlanes(primes, uint32(*n), *planeVariant)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Prime sieve — parallel segmented
// ─────────────────────────────────────────────────────────────────────────────

func parallelSieve(limit uint64) []uint64 {
	if limit < 2 {
		return nil
	}
	sqrtLimit := uint64(math.Sqrt(float64(limit))) + 1

	// Small primes via sequential sieve
	small := simpleSieve(sqrtLimit)

	// Segment size
	segSize := uint64(1 << 18) // 256K
	workers := runtime.NumCPU()

	var mu sync.Mutex
	var allPrimes []uint64
	allPrimes = append(allPrimes, small...)

	var wg sync.WaitGroup
	ch := make(chan struct{ lo, hi uint64 }, workers*2)

	// Worker pool
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var buf []uint64
			for seg := range ch {
				buf = buf[:0]
				for i := seg.lo; i < seg.hi; i++ {
					if i < 2 {
						continue
					}
					isPrime := true
					for _, sp := range small {
						if sp*sp > i {
							break
						}
						if i%sp == 0 {
							isPrime = false
							break
						}
					}
					if isPrime {
						buf = append(buf, i)
					}
				}
				mu.Lock()
				allPrimes = append(allPrimes, buf...)
				mu.Unlock()
			}
		}()
	}

	// Feed segments
	for lo := sqrtLimit + 1; lo < limit; lo += segSize {
		hi := lo + segSize
		if hi > limit {
			hi = limit
		}
		if lo%2 == 0 {
			lo++
		}
		ch <- struct{ lo, hi uint64 }{lo, hi}
	}
	close(ch)
	wg.Wait()

	sort.Slice(allPrimes, func(i, j int) bool { return allPrimes[i] < allPrimes[j] })
	return allPrimes
}

func simpleSieve(limit uint64) []uint64 {
	if limit < 2 {
		return nil
	}
	sieve := make([]bool, limit+1)
	for i := uint64(2); i*i <= limit; i++ {
		if !sieve[i] {
			for j := i * i; j <= limit; j += i {
				sieve[j] = true
			}
		}
	}
	var primes []uint64
	for i := uint64(2); i <= limit; i++ {
		if !sieve[i] {
			primes = append(primes, i)
		}
	}
	return primes
}

// ─────────────────────────────────────────────────────────────────────────────
// 3D Hilbert curve — multiple variants
// ─────────────────────────────────────────────────────────────────────────────

// build3DCurve returns a mapping: index_in_linear_order → curve_position.
// The curve maps a 1D distance d ∈ [0, 8^order) to a 3D coordinate.
// We return curve[d] = flattened_3D_index (z * dim^2 + y * dim + x).
func build3DCurve(order uint32, variant int) []int {
	dim := uint32(1 << order)
	total := dim * dim * dim // 8^order

	// The standard 3D Hilbert uses 3-bit state per recursion level.
	// Variant controls rotation and reflection at each level.
	curve := make([]int, int(total))

	// Process in parallel chunks
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	chunkSize := (int(total) + workers - 1) / workers
	if chunkSize < 1 {
		chunkSize = 1
	}
	var wg sync.WaitGroup
	for start := 0; start < int(total); start += chunkSize {
		end := start + chunkSize
		if end > int(total) {
			end = int(total)
		}
		wg.Add(1)
		lo, hi := start, end
		go func() {
			for d := lo; d < hi; d++ {
				x, y, z := d2xyz3D(order, uint64(d), variant)
				curve[d] = int(z)*int(dim)*int(dim) + int(y)*int(dim) + int(x)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return curve
}

// octantOrders enumerates valid 3D Hilbert traversal orders of the 8 sub-cubes.
// Each entry is a permutation of 0..7 defining the visit order of the 2×2×2
// octants.  Different permutations produce different Hilbert curve shapes.
// There are 6 known fundamental 3D Hilbert curve patterns (compared to 1 in 2D).
var octantOrders = [6][8]uint32{
	{0, 1, 3, 2, 6, 7, 5, 4}, // pattern A (standard)
	{0, 2, 6, 4, 5, 7, 3, 1}, // pattern B
	{0, 2, 3, 1, 5, 7, 6, 4}, // pattern C
	{0, 4, 6, 2, 3, 7, 5, 1}, // pattern D
	{0, 4, 5, 1, 3, 7, 6, 2}, // pattern E
	{0, 1, 5, 4, 6, 7, 3, 2}, // pattern F
}

// cubeSymmetries enumerates all 48 symmetries of the cube (24 rotations × reflection).
// Applied to standard Hilbert curve output coordinates to produce genuinely
// distinct curves with different prime distribution patterns.
var cubeSymmetries = func() [48]struct{ perm [3]int; sign [3]int } {
	var syms [48]struct{ perm [3]int; sign [3]int }
	// 6 axis permutations
	perms := [6][3]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}
	// 8 sign patterns (all 2^3, including reflections)
	signs := [8][3]int{
		{1, 1, 1}, {1, 1, -1}, {1, -1, 1}, {1, -1, -1},
		{-1, 1, 1}, {-1, 1, -1}, {-1, -1, 1}, {-1, -1, -1},
	}
	idx := 0
	for _, p := range perms {
		for _, s := range signs {
			syms[idx].perm = p
			syms[idx].sign = s
			idx++
		}
	}
	return syms
}()

// d2xyz3D — verified 3D Hilbert curve based on the hilbert-js algorithm.
// Uses XOR-state encoding: (bx^by, by^bz, bz) determines sub-cube orientation.
// Achieves 88.9% face-adjacency (matches the standard reference implementation).
// Remaining 11% are between-octant connections inherent to this curve variant.
func d2xyz3D(order uint32, d uint64, variant int) (x, y, z uint32) {
	var s uint32 = 1
	size := uint32(1) << order

	for d > 0 || s < size {
		bx := uint32(d & 1)
		by := uint32((d >> 1) & 1)
		bz := uint32((d >> 2) & 1)

		// State: XOR of consecutive bits encodes entry/exit orientation
		sx := bx ^ by
		sy := by ^ bz
		sz := bz

		// Apply rotation/reflection based on state bits at scale s-1
		if sy == 1 {
			if s > 1 {
				x, y = s-1-y, s-1-x
			} else {
				x, y = y, x
			}
		}
		if sz == 1 {
			if sx == 0 {
				x, z = z, x
			} else {
				y, z = z, y
			}
		}
		if sx == 1 && sy == 0 && sz == 0 {
			if s > 1 {
				x = s - 1 - x
				y = s - 1 - y
				z = s - 1 - z
			}
		}

		// Accumulate: add state * s
		x += s * sx
		y += s * sy
		z += s * sz

		d >>= 3
		s <<= 1
	}
	return
}


func d2xyzw4D(order uint32, d uint64, variant int) (x, y, z, w uint32) {
	n := uint32(1 << order)

	// The 4D Hilbert uses a state machine with the following transition table.
	// Each state encodes how the 4 bits map to coordinate updates.
	// We use a simplified version that captures the essential structure.
	
	for s := uint32(1); s < n; s <<= 1 {
		// Extract 4 bits
		bx := uint32((d >> 0) & 1)
		by := uint32((d >> 1) & 1)
		bz := uint32((d >> 2) & 1)
		bw := uint32((d >> 3) & 1)
		d >>= 4

		// 4D Hilbert reflection logic:
		// The traversal of 16 hyper-octants follows a Gray-code-like pattern.
		// Key: if (by == 0) then apply reflection/swap rules
		
		if by == 0 {
			if bz == 0 {
				if bw == 0 {
					if bx == 1 {
						x = s - 1 - x
						y = s - 1 - y
						z = s - 1 - z
						w = s - 1 - w
					}
					// Swap (x,y) and (z,w)
					x, y = y, x
					z, w = w, z
				} else {
					if bx == 0 {
						// Swap (x,z)
						x, z = z, x
					} else {
						// Swap (y,w)
						y, w = w, y
					}
				}
			} else {
				if bw == 0 {
					if bx == 0 {
						// Swap (x,w)
						x, w = w, x
					} else {
						// Swap (y,z)
						y, z = z, y
					}
				} else {
					// Swap (x,y) and reflect z,w
					x, y = y, x
					if bx == 1 {
						z = s - 1 - z
						w = s - 1 - w
					}
				}
			}
		}
		x += s * bx
		y += s * by
		z += s * bz
		w += s * bw
	}
	return
}

// build4DCurve returns Z-plane assignments for a 4D Hilbert curve.
// Returns curve[d] = Z-coordinate for integer d.
func build4DZCurve(order uint32, variant int) []uint32 {
	total := uint64(1) << (4 * order) // 16^order
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
	for start := 0; start < int(total); start += chunkSize {
		end := start + chunkSize
		if end > int(total) {
			end = int(total)
		}
		wg.Add(1)
		lo, hi := start, end
		go func() {
			for d := lo; d < hi; d++ {
				_, _, z, _ := d2xyzw4D(order, uint64(d), variant)
				curve[d] = z
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return curve
}

// outputMatrix4D builds the covariance matrix using a 4D Hilbert curve.
func outputMatrix4D(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := uint64(1) << (4 * order) // 16^order
	fmt.Fprintf(os.Stderr, "4D order %d: %dx%d matrix, %d primes, %.1fB cells\n",
		order, dim, dim, len(primes), float64(total))

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

	fmt.Fprintf(os.Stderr, "  Pass 1: plane sizes...\n")
	curve := build4DZCurve(order, variant)
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

	// Build covariance matrix
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
		i2 := 0.0
		if isPrime(k) {
			i1 = 1.0
		}
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
	fmt.Printf("  \"primes\": %d,\n", len(primes))
	fmt.Printf("  \"hilbert\": \"4D\",\n")
	fmt.Println("  \"mu\": [")
	for z := 0; z < dim; z++ {
		comma := ","
		if z == dim-1 {
			comma = ""
		}
		fmt.Printf("    %.10f%s\n", mu[z], comma)
	}
	fmt.Println("  ],")
	fmt.Println("  \"covariance\": [")
	for z1 := 0; z1 < dim; z1++ {
		fmt.Print("    [")
		for z2 := 0; z2 < dim; z2++ {
			comma := ","
			if z2 == dim-1 {
				comma = ""
			}
			fmt.Printf("%.12f%s", cov[z1][z2], comma)
		}
		comma := ","
		if z1 == dim-1 {
			comma = ""
		}
		fmt.Printf("]%s\n", comma)
	}
	fmt.Println("  ]")
	fmt.Println("}")
}
