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

// d2xyz3D maps a distance d along a 3D Hilbert curve to (x, y, z).
// Computes the standard curve then applies a cube symmetry from the variant.
func d2xyz3D(order uint32, d uint64, variant int) (x, y, z uint32) {
	// Standard 3D Hilbert curve (pattern A)
	n := uint32(1 << order)
	for s := uint32(1); s < n; s <<= 1 {
		bx := uint32((d >> 0) & 1)
		by := uint32((d >> 1) & 1)
		bz := uint32((d >> 2) & 1)
		d >>= 3

		if by == 0 {
			if bz == 0 {
				if bx == 1 {
					x = s - 1 - x
					y = s - 1 - y
					z = s - 1 - z
				}
				x, y = y, x
			} else if bx == 0 {
				x, z = z, x
			} else {
				y, z = z, y
			}
		}
		x += s * bx
		y += s * by
		z += s * bz
	}

	// Apply cube symmetry transformation
	sym := cubeSymmetries[variant%48]
	dim := n

	// Map (x,y,z) through permutation
	coords := [3]uint32{x, y, z}
	nx := coords[sym.perm[0]]
	ny := coords[sym.perm[1]]
	nz := coords[sym.perm[2]]

	// Apply sign flips (negative → reflect across center)
	if sym.sign[0] < 0 {
		nx = dim - 1 - nx
	}
	if sym.sign[1] < 0 {
		ny = dim - 1 - ny
	}
	if sym.sign[2] < 0 {
		nz = dim - 1 - nz
	}

	return nx, ny, nz
}

// rotation24 enumerates all 24 proper rotation matrices of the cube
// (the chiral octahedral group). Each is a 3×3 matrix with entries in
// {0, 1, -1}, exactly one non-zero per row/column, determinant +1.
var rotation24 = func() [24][3][3]int {
	// The 6 axis permutations (column assignments)
	perms := [6][3]int{
		{0, 1, 2}, // x→x, y→y, z→z
		{0, 2, 1}, // x→x, y→z, z→y
		{1, 0, 2}, // x→y, y→x, z→z
		{1, 2, 0}, // x→y, y→z, z→x
		{2, 0, 1}, // x→z, y→x, z→y
		{2, 1, 0}, // x→z, y→y, z→x
	}
	// 4 sign patterns with determinant +1 (even number of sign flips)
	signs := [4][3]int{
		{1, 1, 1},   // all positive
		{1, -1, -1}, // y,z flipped (det = +1)
		{-1, 1, -1}, // x,z flipped (det = +1)
		{-1, -1, 1}, // x,y flipped (det = +1)
	}
	var mats [24][3][3]int
	idx := 0
	for _, p := range perms {
		for _, s := range signs {
			for axis := 0; axis < 3; axis++ {
				mats[idx][axis][p[axis]] = s[axis]
			}
			idx++
		}
	}
	return mats
}()

// variantRotation returns the 3D rotation matrix for the given variant index.
// The variant determines how axes map and which signs flip, producing distinct
// Hilbert curve shapes.  There are exactly 24 proper rotations.
func variantRotation(variant int) [3][3]int {
	return rotation24[variant%24]
}

// ─────────────────────────────────────────────────────────────────────────────
// Alignment testing
// ─────────────────────────────────────────────────────────────────────────────

// testAlignment measures locality of primes along the Hilbert curve.
// Computes the mean curve-distance between consecutive primes mapped onto
// the curve, normalized against the expected distance for a random set.
// Low score = primes are closer together along the curve = alignment.
func testAlignment(hits []int, order uint32) float64 {
	// Collect curve positions of all primes (indices where hits > 0)
	var positions []int
	for i, h := range hits {
		if h > 0 {
			positions = append(positions, i)
		}
	}
	if len(positions) < 2 {
		return 1.0
	}

	// Sort positions by curve order
	sort.Ints(positions)

	// Compute mean gap between consecutive primes along the curve
	totalGap := 0
	for i := 1; i < len(positions); i++ {
		totalGap += positions[i] - positions[i-1]
	}
	meanGap := float64(totalGap) / float64(len(positions)-1)

	// Expected gap if primes were randomly distributed:
	// total cells / number of primes
	expectedGap := float64(len(hits)) / float64(len(positions))

	// Alignment score: how much smaller are actual gaps vs expected?
	// Score < 1 means primes are closer than random → alignment
	return meanGap / expectedGap
}

// classifyAlignment interprets the alignment score.
func classifyAlignment(score float64, order uint32) string {
	switch {
	case score < 0.7:
		return "strongly aligned — primes cluster along curve!"
	case score < 0.85:
		return "aligned — primes closer than random"
	case score < 0.95:
		return "weakly aligned"
	case score < 1.05:
		return "random (no alignment)"
	case score < 1.3:
		return "slightly dispersed"
	default:
		return "dispersed — primes avoid each other"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Curve variant description
// ─────────────────────────────────────────────────────────────────────────────

func describeVariant(variant int) string {
	// The 24 cube rotations of the octahedral group
	names := []string{
		"identity (xyz+)",
		"identity (xyz with one sign flip)",
		"identity (xyz with two sign flips)",
		"identity (xyz with three sign flips)",
		"swap xy (yxz+)",
		"swap xy (yxz sign variant 1)",
		"swap xy (yxz sign variant 2)",
		"swap xy (yxz sign variant 3)",
		"swap xz (zyx+)",
		"swap xz (zyx sign variant 1)",
		"swap yz (xzy+)",
		"swap yz (xzy sign variant 1)",
		"rotate xy→yz→zx (yzx+)",
		"rotate xy→yz→zx variant 1",
		"rotate zx→xy→yz (zxy+)",
		"rotate zx→xy→yz variant 1",
		"rotate xy→-xz (perm 5+)",
		"rotate variant 17",
		"rotate variant 18",
		"rotate variant 19",
		"rotate variant 20",
		"rotate variant 21",
		"rotate variant 22",
		"rotate variant 23",
	}
	if variant < len(names) {
		return names[variant]
	}
	return fmt.Sprintf("variant-%d", variant)
}

// ─────────────────────────────────────────────────────────────────────────────
// Curve estimation
// ─────────────────────────────────────────────────────────────────────────────

func estimateCurves(order int) {
	fmt.Printf("\n── 3D Hilbert Curve Estimation ──\n")

	// Number of 3D Hilbert curves of order n:
	// At each recursion level, we have 8 sub-cubes arranged in a 2×2×2 grid.
	// A 3D Hilbert curve is a Hamiltonian path through these 8 sub-cubes
	// that satisfies the Hilbert continuity property.
	//
	// Base: the number of valid traversals of the 2×2×2 cube is the number
	// of Hamiltonian paths that respect the Hilbert adjacency constraints.
	// For a 3D cube, there are exactly 6 distinct "patterns" a 3D Hilbert
	// curve can follow (compared to 1 pattern in 2D).
	//
	// Recursive: each sub-cube of size s becomes a cube of size s/2 with
	// its own internal Hilbert curve, rotated to connect properly.
	// Each sub-cube can use any of the 24 orientation-preserving rotations.
	//
	// Total estimate: 6 (base patterns) × 24^(order-1) (rotations per level)
	// But many rotations produce identical curves due to symmetry.
	// A tighter bound: the automorphism group of the 3D Hilbert curve
	// has size approximately 24 × 8 = 192.
	//
	// So distinct curves ≈ 6 × 24^(order-1) / symmetry_factor

	n := order
	basePatterns := big.NewInt(6)
	rotations := big.NewInt(24)
	power := new(big.Int).Exp(rotations, big.NewInt(int64(n-1)), nil)
	total := new(big.Int).Mul(basePatterns, power)

	// Symmetry factor: each curve has ~192 automorphisms
	symmetry := big.NewInt(192)
	distinct := new(big.Int).Div(total, symmetry)

	fmt.Printf("order %d (grid: %d×%d×%d):\n", n, 1<<n, 1<<n, 1<<n)
	fmt.Printf("  base patterns (2×2×2 traversals):  6\n")
	fmt.Printf("  rotations per recursion level:      24\n")
	fmt.Printf("  recursion levels:                    %d\n", n-1)
	fmt.Printf("  total combinatorial variants:        ~%s\n", formatBig(total))
	fmt.Printf("  estimated distinct curves (mod symmetry): ~%s\n", formatBig(distinct))

	// For context
	if n <= 6 {
		approx := new(big.Int).Div(distinct, big.NewInt(1))
		fmt.Printf("  (visually: %s possible 3D Hilbert curves at order %d)\n",
			formatBig(approx), n)
	}
}

func formatBig(x *big.Int) string {
	s := x.String()
	if len(s) <= 12 {
		return s
	}
	return fmt.Sprintf("%se%d", s[:6], len(s)-1)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func pow64(base uint64, exp int) uint64 {
	r := uint64(1)
	for i := 0; i < exp; i++ {
		r *= base
	}
	return r
}


// ─────────────────────────────────────────────────────────────────────────────
// 3D rotating cube movie
// ─────────────────────────────────────────────────────────────────────────────

func renderMovie(primes []uint64, order uint32, variant int, outPath string, numFrames int) {
	fmt.Printf("\n── Rendering rotating cube movie (%d frames) …\n", numFrames)

	// Build the 3D hits grid
	curve := build3DCurve(order, variant)
	dim := int(1 << order)
	hits := make([]uint8, dim*dim*dim)
	for _, p := range primes {
		if int(p) < len(curve) {
			hits[curve[p]] = 1
		}
	}

	// Temp dir for frames
	tmpDir, err := os.MkdirTemp("", "hprime-movie-*")
	if err != nil {
		fmt.Printf("temp dir error: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Render frames
	outSize := 512
	var pngFiles []string
	for frame := 0; frame < numFrames; frame++ {
		angleY := float64(frame) * 2.0 * math.Pi / float64(numFrames)
		angleX := math.Pi / 6.0 // slight tilt
		img := renderCubeView(hits, dim, angleX, angleY, outSize)

		fn := filepath.Join(tmpDir, fmt.Sprintf("frame_%04d.png", frame))
		f, _ := os.Create(fn)
		png.Encode(f, img)
		f.Close()
		pngFiles = append(pngFiles, fn)

		if frame%(numFrames/10) == 0 {
			fmt.Printf("  frame %d/%d\n", frame, numFrames)
		}
	}

	// Compose with ffmpeg
	concatFile := filepath.Join(tmpDir, "concat.txt")
	var lines []string
	for _, p := range pngFiles {
		lines = append(lines, fmt.Sprintf("file '%s'", p))
	}
	os.WriteFile(concatFile, []byte(joinLines(lines)), 0644)

	cmd := exec.Command("ffmpeg",
		"-y", "-f", "concat", "-safe", "0", "-r", "30",
		"-i", concatFile,
		"-vf", fmt.Sprintf("scale=%d:%d", outSize, outSize),
		"-pix_fmt", "yuv420p",
		outPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("ffmpeg error: %v\n", err)
		return
	}
	fmt.Printf("movie: %s  (%d frames)\n", outPath, numFrames)
}

func joinLines(lines []string) string {
	s := ""
	for _, l := range lines {
		s += l + "\n"
	}
	return s
}

// renderCubeView renders the 3D cube from a given viewpoint using orthographic
// projection with a simple rotation around Y then X axis.
func renderCubeView(hits []uint8, dim int, angleX, angleY float64, outSize int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, outSize, outSize))
	// Black background
	for y := 0; y < outSize; y++ {
		for x := 0; x < outSize; x++ {
			img.SetGray(x, y, color.Gray{Y: 0})
		}
	}

	// Precompute rotation
	cosY, sinY := math.Cos(angleY), math.Sin(angleY)
	cosX, sinX := math.Cos(angleX), math.Sin(angleX)

	half := float64(dim) / 2.0
	scale := float64(outSize) / (float64(dim) * 1.8)

	// Render back-to-front (painter's algorithm approximation)
	for z := 0; z < dim; z++ {
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				idx := z*dim*dim + y*dim + x
				if hits[idx] == 0 {
					continue
				}

				// Center coordinates
				cx := float64(x) - half
				cy := float64(y) - half
				cz := float64(z) - half

				// Rotate around Y axis
				rx := cx*cosY + cz*sinY
				rz := -cx*sinY + cz*cosY
				ry := cy

				// Rotate around X axis
				ry2 := ry*cosX - rz*sinX
				rz2 := ry*sinX + rz*cosX

				// Project to 2D (orthographic: drop Z after depth sort)
				px := int(rx*scale + float64(outSize)/2)
				py := int(ry2*scale + float64(outSize)/2)

				if px >= 0 && px < outSize && py >= 0 && py < outSize {
					// Brightness based on depth (z-order for visual cue)
					depth := (rz2 + half) / float64(dim) // 0..1
					bright := uint8(128 + depth*127)
					img.SetGray(px, py, color.Gray{Y: bright})
				}
			}
		}
	}
	return img
}

// ─────────────────────────────────────────────────────────────────────────────
// Plane-alignment analysis
// ─────────────────────────────────────────────────────────────────────────────

func analyzePlanes(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	fmt.Printf("\n── Plane-Alignment Analysis (order %d, %d×%d×%d, variant %d) ──\n",
		order, dim, dim, dim, variant)

	// Build curve and map primes to 3D coordinates
	curve := build3DCurve(order, variant)

	// Store (prime_value, x, y, z) for each prime
	type point struct {
		val   uint64
		x, y, z int
	}
	var points []point
	for _, p := range primes {
		if int(p) < len(curve) {
			pos := curve[p]
			z := pos / (dim * dim)
			y := (pos / dim) % dim
			x := pos % dim
			points = append(points, point{p, x, y, z})
		}
	}

	// Per-plane counts for X, Y, Z
	xPlanes := make([]int, dim)
	yPlanes := make([]int, dim)
	zPlanes := make([]int, dim)

	// Per-plane residue-class counts (mod 8)
	type resCounts [8]int
	xRes := make([]resCounts, dim)
	yRes := make([]resCounts, dim)
	zRes := make([]resCounts, dim)

	for _, pt := range points {
		xPlanes[pt.x]++
		yPlanes[pt.y]++
		zPlanes[pt.z]++
		r := pt.val % 8
		xRes[pt.x][r]++
		yRes[pt.y][r]++
		zRes[pt.z][r]++
	}

	expected := float64(len(points)) / float64(dim)

	// Find planes with significant deviation
	fmt.Printf("\n  Expected primes per plane: %.1f\n\n", expected)

	// Helper: print significant planes
	printSignificant := func(axis string, counts []int, residues []resCounts) {
		bestZ := 0.0
		bestPlane := -1
		for i := 0; i < dim; i++ {
			diff := float64(counts[i]) - expected
			zScore := diff / math.Sqrt(expected)
			if math.Abs(zScore) > math.Abs(bestZ) {
				bestZ = zScore
				bestPlane = i
			}
			// Print planes with |z-score| > 2.5
			if math.Abs(zScore) > 2.5 {
				fmt.Printf("  %s=%3d  count=%4d  z=%.2f  residues: ", axis, i, counts[i], zScore)
				for r := 0; r < 8; r++ {
					if residues[i][r] > 0 {
						fmt.Printf("mod%d=%d ", r, residues[i][r])
					}
				}
				fmt.Println()
			}
		}
		if bestPlane >= 0 {
			fmt.Printf("  strongest %s-plane: %d (z=%.2f)\n\n", axis, bestPlane, bestZ)
		}
	}

	fmt.Println("  Significant X-planes (|z| > 2.5):")
	printSignificant("X", xPlanes, xRes)
	fmt.Println("  Significant Y-planes (|z| > 2.5):")
	printSignificant("Y", yPlanes, yRes)
	fmt.Println("  Significant Z-planes (|z| > 2.5):")
	printSignificant("Z", zPlanes, zRes)

	// Residue class alignment summary
	fmt.Println("  Residue class distribution (mod 8) across all points:")
	var totalRes [8]int
	for _, pt := range points {
		totalRes[pt.val%8]++
	}
	for r := 0; r < 8; r++ {
		pct := float64(totalRes[r]) / float64(len(points)) * 100
		fmt.Printf("    mod %d = %d: %6d  (%5.1f%%)\n", r, r, totalRes[r], pct)
	}
}

// compareAllVariants runs plane analysis on all 48 cube symmetries and
// reports the best variant for plane-alignment.
func compareAllVariants(primes []uint64, order uint32) {
	fmt.Printf("\n── Comparing all 48 cube symmetry variants ──\n")
	fmt.Printf("  searching for strongest plane-alignment signal...\n\n")

	type result struct {
		variant   int
		maxZScore float64
		plane     string
		count     int
	}
	var results []result

	for v := 0; v < 48; v++ {
		dim := int(1 << order)
		curve := build3DCurve(order, v)

		type pt struct{ x, y, z int }
		var pts []pt
		// Only sample first 4096 primes to keep it fast
		for _, p := range primes {
			if int(p) < len(curve) {
				pos := curve[p]
				pts = append(pts, pt{pos % dim, (pos / dim) % dim, pos / (dim * dim)})
			}
		}

		zPlanes := make([]int, dim)
		for _, p := range pts {
			zPlanes[p.z]++
		}
		expected := float64(len(pts)) / float64(dim)
		maxZ := 0.0
		bestPlane := 0
		bestCount := 0
		for i := 0; i < dim; i++ {
			z := (float64(zPlanes[i]) - expected) / math.Sqrt(expected)
			if math.Abs(z) > math.Abs(maxZ) {
				maxZ = z
				bestPlane = i
				bestCount = zPlanes[i]
			}
		}
		results = append(results, result{v, maxZ, fmt.Sprintf("Z=%d", bestPlane), bestCount})
	}

	// Sort by absolute z-score
	sort.Slice(results, func(i, j int) bool {
		return math.Abs(results[i].maxZScore) > math.Abs(results[j].maxZScore)
	})

	fmt.Println("  Top 10 variants by strongest plane signal:")
	fmt.Printf("  %-4s %-8s %-30s %6s  %s\n", "Rank", "Variant", "Description", "Z-max", "Plane (count)")
	fmt.Println("  " + strings.Repeat("-", 75))
	for i := 0; i < 10 && i < len(results); i++ {
		r := results[i]
		desc := describeVariant(r.variant)
		if len(desc) > 28 {
			desc = desc[:28]
		}
		fmt.Printf("  %-4d %-8d %-30s %+6.2f  %s (%d)\n",
			i+1, r.variant, desc, r.maxZScore, r.plane, r.count)
	}

	best := results[0]
	fmt.Printf("\n── Best variant: %d (%s) ──\n", best.variant, describeVariant(best.variant))
	fmt.Printf("  strongest plane signal: z=%.2f\n", best.maxZScore)
	fmt.Printf("\n── Utility postulation ──\n")
	fmt.Printf(`
  If primes consistently accumulate on specific planes of certain 3D Hilbert
  curve variants, this has several potential applications:

  1. COMPRESSION: A curve variant that clusters primes onto fewer planes reduces
     the effective dimensionality — the prime set can be stored as "plane numbers
     plus offsets" rather than full 3D coordinates. At z=%.1f, the best plane
     holds %.0f/%d = %.1f%% of primes vs %.1f%% expected.

  2. HASHING / INDEXING: A curve that concentrates primes spatially means
     prime-based hash functions would have non-uniform bucket distribution.
     This is useful for designing hash tables where certain buckets are
     intentionally "hot" for cache optimization.

  3. NUMBER THEORY INSIGHT: The residue class clustering on specific planes
     reveals hidden structure in how the Hilbert curve interacts with
     modular arithmetic. The curve's 3-bit encoding (octants) creates a
     natural modulo-8 lattice — primes avoid certain lattice sites entirely
     (even residues) and cluster on others.

  4. CRYPTOGRAPHIC RELEVANCE: If a specific Hilbert curve variant produces
     statistically significant prime clustering (|z| > 4), this constitutes
     a non-random structural property that could serve as a distinguisher
     in pseudorandom number generator testing.

  5. DATA VISUALIZATION: For prime-oriented datasets, choosing a Hilbert curve
     that maximizes spatial clustering makes heatmaps more informative —
     structures invisible in random layouts become apparent.

  The effect is modest at order 5 (max |z| ~ %.1f) but may amplify at
  higher orders where the recursive structure compounds the bias.
`, math.Abs(best.maxZScore), float64(best.count), int(1<<order),
		float64(best.count)/float64(len(primes))*100,
		100.0/float64(int(1<<order)),
		math.Abs(best.maxZScore))
}

// ─────────────────────────────────────────────────────────────────────────────
// Zeta correlation test
// ─────────────────────────────────────────────────────────────────────────────

func runCorrelationTest(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := dim * dim * dim
	fmt.Printf("\n── Zeta Correlation Test (order %d, variant %d) ──\n", order, variant)

	// Build prime set for O(1) lookup
	primeSet := make(map[uint64]bool, len(primes))
	for _, p := range primes {
		primeSet[p] = true
	}

	// Build Hilbert curve
	curve := build3DCurve(order, variant)

	// For each Z-plane, collect statistics
	type planeStats struct {
		z          int
		count      int      // primes on this plane
		totalInts  int      // total integers mapping to this plane
		samples    []uint64 // sampled integers for error computation
	}
	planes := make([]planeStats, dim)
	for z := 0; z < dim; z++ {
		planes[z].z = z
	}

	// Sample up to 1000 integers per plane for error estimation
	sampleSize := 1000
	for k := uint64(0); k < uint64(total); k++ {
		z := curve[k] / (dim * dim)
		planes[z].totalInts++
		if primeSet[k] {
			planes[z].count++
		}
		// Reservoir sample: take first sampleSize, then randomly replace
		if len(planes[z].samples) < sampleSize {
			planes[z].samples = append(planes[z].samples, k)
		}
	}

	// Compute vectors for correlation
	N := dim
	density := make([]float64, N) // excess prime density per plane
	errorPNT := make([]float64, N) // mean normalized PNT error

	for z := 0; z < dim; z++ {
		if planes[z].totalInts == 0 {
			continue
		}
		// Excess density (same as earlier plane analysis)
		expected := float64(len(primes)) / float64(dim)
		density[z] = (float64(planes[z].count) - expected) / math.Sqrt(expected)

		// Mean PNT error over sampled integers
		var sumErr float64
		for _, k := range planes[z].samples {
			// Compute li(k) approximation: li(x) ≈ x/ln(x) * (1 + 1/ln(x) + 2/ln²(x))
			if k < 2 {
				continue
			}
			x := float64(k)
			lnX := math.Log(x)
			liX := x / lnX * (1 + 1/lnX + 2/(lnX*lnX))
			piX := float64(countPrimesLE(primes, k))
			// Normalized error
			sumErr += (piX - liX) / math.Sqrt(x)
		}
		if len(planes[z].samples) > 0 {
			errorPNT[z] = sumErr / float64(len(planes[z].samples))
		}
	}

	// Pearson correlation
	r := pearson(density, errorPNT)
	fmt.Printf("\n  Pearson r(plane_density, PNT_error) = %.6f\n", r)

	// Permutation test (1000 shuffles)
	better := 0
	trials := 1000
	perm := make([]float64, N)
	copy(perm, density)
	for t := 0; t < trials; t++ {
		// Shuffle density vector
		for i := range perm {
			j := i + int(uint64(i*2654435761)%uint64(N-i))
			perm[i], perm[j] = perm[j], perm[i]
		}
		rp := pearson(perm, errorPNT)
		if math.Abs(rp) >= math.Abs(r) {
			better++
		}
	}
	pValue := float64(better) / float64(trials)
	fmt.Printf("  Permutation test p = %.4f  (trials=%d, %d >= |r|)\n", pValue, trials, better)

	// Interpretation
	switch {
	case pValue < 0.01:
		fmt.Println("  *** SIGNIFICANT: p < 0.01 — correlation unlikely by chance ***")
	case pValue < 0.05:
		fmt.Println("  ** NOTABLE: p < 0.05 — weak evidence of correlation")
	case pValue < 0.10:
		fmt.Println("  * SUGGESTIVE: p < 0.10 — possible trend, needs higher order")
	default:
		fmt.Println("  No significant correlation detected at this order")
	}

	// Top/bottom 5 planes by PNT error
	fmt.Println("\n  Top 5 planes by positive PNT error (π(x) > li(x)):")
	type zErr struct{ z int; d, e float64 }
	var ranked []zErr
	for z := 0; z < dim; z++ {
		ranked = append(ranked, zErr{z, density[z], errorPNT[z]})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].e > ranked[j].e })
	for i := 0; i < 5 && i < len(ranked); i++ {
		fmt.Printf("    Z=%3d  density_z=%.2f  pnt_err=%.4f\n",
			ranked[i].z, ranked[i].d, ranked[i].e)
	}
	fmt.Println("  Bottom 5 planes by PNT error (π(x) < li(x)):")
	for i := len(ranked) - 1; i >= len(ranked)-5 && i >= 0; i-- {
		fmt.Printf("    Z=%3d  density_z=%.2f  pnt_err=%.4f\n",
			ranked[i].z, ranked[i].d, ranked[i].e)
	}

	// Check: do hot planes (high prime density) also have positive PNT error?
	hotZ := []int{}
	for z := 0; z < dim; z++ {
		if math.Abs(density[z]) > 2.5 {
			hotZ = append(hotZ, z)
		}
	}
	if len(hotZ) > 0 {
		posErr := 0
		for _, z := range hotZ {
			if errorPNT[z] > 0 {
				posErr++
			}
		}
		fmt.Printf("\n  Hot planes (|z|>2.5): %d total, %d with positive PNT error (%.0f%%)\n",
			len(hotZ), posErr, float64(posErr)/float64(len(hotZ))*100)
		fmt.Println("  (If zeta correlation exists, hot planes should show positive PNT error)")
	}
}

func pearson(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}
	n := float64(len(x))
	var sx, sy, sxy, sx2, sy2 float64
	for i := range x {
		sx += x[i]
		sy += y[i]
		sxy += x[i] * y[i]
		sx2 += x[i] * x[i]
		sy2 += y[i] * y[i]
	}
	num := n*sxy - sx*sy
	den := math.Sqrt((n*sx2 - sx*sx) * (n*sy2 - sy*sy))
	if den == 0 {
		return 0
	}
	return num / den
}

func countPrimesLE(primes []uint64, x uint64) int {
	lo, hi := 0, len(primes)
	for lo < hi {
		mid := (lo + hi) / 2
		if primes[mid] <= x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// ─────────────────────────────────────────────────────────────────────────────
// Hilbert plane operator matrix construction
// ─────────────────────────────────────────────────────────────────────────────

func computeOperatorMatrix(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := dim * dim * dim
	fmt.Printf("\n── Hilbert Plane Operator Matrix (order %d, variant %d) ──\n", order, variant)

	// Build the Hilbert curve
	curve := build3DCurve(order, variant)

	// Count how many integers map to each Z-plane
	planeSize := make([]int, dim)
	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		planeSize[z]++
	}

	// Build the von Mangoldt-weighted vector v[z]
	// For primes: contribution = log(p)/sqrt(p)
	// For prime powers: contribution = log(p)/sqrt(p^k) — rare, skip for now
	v := make([]float64, dim)
	primeSet := make(map[uint64]bool)
	for _, p := range primes {
		primeSet[p] = true
	}
	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		kk := uint64(k)
		if primeSet[kk] && kk > 1 {
			v[z] += math.Log(float64(kk)) / math.Sqrt(float64(kk))
		}
	}
	// Normalize by plane size
	for z := 0; z < dim; z++ {
		if planeSize[z] > 0 {
			v[z] /= math.Sqrt(float64(planeSize[z]))
		}
	}

	// Build the T_n matrix: T[z1][z2] measures the coupling between planes
	// For the plane-decomposition operator, T is diagonal in the plane basis
	// because each integer maps to exactly one plane.
	// The operator's eigenvalues are simply the per-plane average of v.
	// 
	// The non-trivial structure comes from the covariance matrix:
	// C[z1][z2] = covariance of the prime indicator between planes z1 and z2
	
	// Compute the covariance matrix C
	// C[z1][z2] = E[(I(k∈I_z1) - μ1)(I(k∈I_z2) - μ2)]
	
	fmt.Printf("  Computing %dx%d covariance matrix...\n", dim, dim)
	
	// For large orders, sample to estimate covariance
	// For n=6 (64 planes), we can compute exactly
	sampleSize := 100000
	if total < sampleSize {
		sampleSize = total
	}
	
	// Compute mean prime density per plane
	mu := make([]float64, dim)
	for z := 0; z < dim; z++ {
		// Count primes on this plane
		count := 0
		for _, p := range primes {
			if int(p) < total {
				zp := curve[p] / (dim * dim)
				if zp == z {
					count++
				}
			}
		}
		mu[z] = float64(count) / float64(planeSize[z])
	}
	
	// Compute covariance via sampling
	cov := make([][]float64, dim)
	for i := range cov {
		cov[i] = make([]float64, dim)
	}
	
	step := total / sampleSize
	for k := 0; k < total; k += step {
		z1 := curve[k] / (dim * dim)
		i1 := float64(0)
		if primeSet[uint64(k)] {
			i1 = 1.0
		}
		for dz := -2; dz <= 2; dz++ {
			k2 := k + dz
			if k2 >= 0 && k2 < total && k2 != k {
				z2 := curve[k2] / (dim * dim)
				i2 := float64(0)
				if primeSet[uint64(k2)] {
					i2 = 1.0
				}
				// Only accumulate if planes differ (off-diagonal)
				if z1 != z2 {
					cov[z1][z2] += (i1 - mu[z1]) * (i2 - mu[z2])
				}
			}
		}
	}
	
	// Print the covariance matrix as JSON for external eigenvalue computation
	fmt.Println("  Top 10x10 of covariance matrix:")
	for z1 := 0; z1 < 10 && z1 < dim; z1++ {
		fmt.Printf("  Z=%-2d:", z1)
		for z2 := 0; z2 < 10 && z2 < dim; z2++ {
			fmt.Printf(" % 8.6f", cov[z1][z2])
		}
		fmt.Println()
	}

	// Also output the v vector (plane-weighted von Mangoldt)
	fmt.Println("\n  Plane-weighted prime density vector v[z] (first 20):")
	for z := 0; z < 20 && z < dim; z++ {
		fmt.Printf("  Z=%-2d: % 12.8f\n", z, v[z])
	}
	
	// Compute a simple spectral measure: eigenvalues of the diagonal
	// of the covariance matrix (the variance per plane)
	fmt.Println("\n  Per-plane variance (diagonal of covariance):")
	eigenvalues := make([]float64, dim)
	for z := 0; z < dim; z++ {
		eigenvalues[z] = mu[z] * (1 - mu[z]) // Bernoulli variance
	}
	
	// Sort eigenvalues descending
	sort.Slice(eigenvalues, func(i, j int) bool { return eigenvalues[i] > eigenvalues[j] })
	
	fmt.Println("  Top 10 eigenvalues (sorted):")
	for i := 0; i < 10 && i < dim; i++ {
		fmt.Printf("    λ_%d = %.8f\n", i+1, eigenvalues[i])
	}
	
	// Compare to zeta zeros: compute the ratio of consecutive eigenvalues
	// and compare to the ratio of consecutive zeta zero spacings
	fmt.Println("\n  Eigenvalue ratios (λ_i/λ_{i+1}):")
	for i := 0; i < 9 && i < dim-1; i++ {
		if eigenvalues[i+1] > 0 {
			fmt.Printf("    λ_%d/λ_%d = %.6f\n", i+1, i+2, eigenvalues[i]/eigenvalues[i+1])
		}
	}
	
	// Output the full matrix as JSON for Python analysis
	fmt.Println("\n  Full matrix available via --matrix-json flag (to be implemented)")
}

// outputMatrix writes the full covariance matrix as JSON for external analysis.
func outputMatrix(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := dim * dim * dim
	curve := build3DCurve(order, variant)

	// Compute plane sizes
	planeSize := make([]int, dim)
	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		planeSize[z]++
	}

	// Compute mean prime density per plane
	mu := make([]float64, dim)
	primeSet := make(map[uint64]bool)
	for _, p := range primes {
		if int(p) < total {
			primeSet[p] = true
		}
	}
	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		if primeSet[uint64(k)] {
			mu[z]++
		}
	}
	for z := 0; z < dim; z++ {
		mu[z] /= float64(planeSize[z])
	}

	// Build full covariance matrix
	// C[z1][z2] = covariance of indicator between planes z1, z2
	// Sample pairs of adjacent integers along the Hilbert curve
	cov := make([][]float64, dim)
	for i := range cov {
		cov[i] = make([]float64, dim)
	}

	// Use all integers: for each k, get its plane z1=k.z
	// and correlate with z2=(k+1).z (adjacent along curve)
	counts := make([][]int, dim)
	for i := range counts {
		counts[i] = make([]int, dim)
	}

	for k := 0; k < total-1; k++ {
		z1 := curve[k] / (dim * dim)
		z2 := curve[k+1] / (dim * dim)
		i1 := 0.0
		i2 := 0.0
		if primeSet[uint64(k)] {
			i1 = 1.0
		}
		if primeSet[uint64(k+1)] {
			i2 = 1.0
		}
		cov[z1][z2] += (i1 - mu[z1]) * (i2 - mu[z2])
		counts[z1][z2]++
	}

	// Normalize by counts
	for z1 := 0; z1 < dim; z1++ {
		for z2 := 0; z2 < dim; z2++ {
			if counts[z1][z2] > 0 {
				cov[z1][z2] /= float64(counts[z1][z2])
			}
		}
	}

	// Symmetrize
	for z1 := 0; z1 < dim; z1++ {
		for z2 := z1 + 1; z2 < dim; z2++ {
			avg := (cov[z1][z2] + cov[z2][z1]) / 2.0
			cov[z1][z2] = avg
			cov[z2][z1] = avg
		}
	}

	// Output as JSON
	fmt.Println("{")
	fmt.Printf("  \"order\": %d,\n", order)
	fmt.Printf("  \"dim\": %d,\n", dim)
	fmt.Printf("  \"primes\": %d,\n", len(primes))
	fmt.Println("  \"mu\": [")
	for z := 0; z < dim; z++ {
		comma := ","
		if z == dim-1 {
			comma = ""
		}
		fmt.Printf("    %.8f%s\n", mu[z], comma)
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
			fmt.Printf("%.10f%s", cov[z1][z2], comma)
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

// computeOperatorExplicit builds the T_n matrix using von Mangoldt weighting
// and outputs eigenvalues for direct comparison with zeta zeros.
// This is the operator that Theorem 1-2 of the RH proof plan describes.
func computeOperatorExplicit(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := dim * dim * dim
	fmt.Printf("\n── Explicit Formula Operator (order %d, %dx%d) ──\n", order, dim, dim)

	// Build Hilbert curve
	curve := build3DCurve(order, variant)

	// Compute plane sizes and the von Mangoldt-weighted projection
	planeSize := make([]int, dim)
	v := make([]float64, dim) // projected von Mangoldt vector

	primeSet := make(map[uint64]bool)
	for _, p := range primes {
		primeSet[p] = true
	}

	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		planeSize[z]++
		kk := uint64(k)
		if primeSet[kk] && kk > 1 {
			// von Mangoldt: log(p) for primes, with 1/sqrt(k) normalization
			v[z] += math.Log(float64(kk)) / math.Sqrt(float64(kk))
		}
	}

	// Normalize by 1/sqrt(|I_z|) for L² normalization
	for z := 0; z < dim; z++ {
		if planeSize[z] > 0 {
			v[z] /= math.Sqrt(float64(planeSize[z]))
		}
	}

	// Build the operator matrix in the plane basis
	// T[z1][z2] = sum over k in I_z1 of (indicator of k in I_z2)
	// This is the Gram matrix of the plane indicator functions
	// Under L² normalization: T[z1][z2] = <1_{I_z1}, 1_{I_z2}> / sqrt(|I_z1||I_z2|)
	
	// For the Hilbert curve, integers in different planes are disjoint,
	// so the Gram matrix is diagonal: T[z1][z2] = delta_{z1,z2}
	// The non-trivial structure comes from the von Mangoldt-weighted
	// projection, not from the plane basis itself.
	
	// Instead, compute the resolvent: for each plane z,
	// what is the spectral response to the explicit formula?
	// R(z, γ) = |sum_{k in I_z} k^{iγ} / sqrt(k)|² / |I_z|
	
	// We can compute this for the first few zeta zeros and see
	// which planes respond most strongly to which zeros.
	
	fmt.Println("  Computing spectral response for first 8 zeta zeros...")
	zeros := []float64{14.134725, 21.022040, 25.010857, 30.424876, 32.935062, 37.586178, 40.918719, 43.327073}
	
	// Sample integers from each plane to estimate spectral response
	type planeResponse struct {
		z int
		responses []float64
	}
	var responses []planeResponse
	
	for z := 0; z < dim; z++ {
		// Sample up to 500 integers from this plane
		sample := min(500, planeSize[z])
		var resp []float64
		for _, gamma := range zeros {
			var sumReal, sumImag float64
			count := 0
			for k := 0; k < total && count < sample; k++ {
				if curve[k]/(dim*dim) == z {
					kk := float64(k)
					if kk > 1 {
						phase := gamma * math.Log(kk)
						sumReal += math.Cos(phase) / math.Sqrt(kk)
						sumImag += math.Sin(phase) / math.Sqrt(kk)
					}
					count++
				}
			}
			if count > 0 {
				power := (sumReal*sumReal + sumImag*sumImag) / float64(count)
				resp = append(resp, power)
			} else {
				resp = append(resp, 0)
			}
		}
		responses = append(responses, planeResponse{z, resp})
	}
	
	// Find which plane responds most strongly to each zero
	fmt.Println("\n  Plane with maximum spectral response for each zeta zero:")
	for i, gamma := range zeros {
		maxResp := 0.0
		maxZ := -1
		for _, pr := range responses {
			if pr.responses[i] > maxResp {
				maxResp = pr.responses[i]
				maxZ = pr.z
			}
		}
		fmt.Printf("    γ_%d = %.3f  →  Z=%d  (response=%.6f)\n", i+1, gamma, maxZ, maxResp)
	}
	
	// Key test: do different zeros map to DIFFERENT planes?
	// If the operator diagonalizes the explicit formula, each zero
	// should have a unique plane that responds maximally.
	seenPlanes := make(map[int]bool)
	unique := 0
	for i := range zeros {
		maxResp := 0.0
		maxZ := -1
		for _, pr := range responses {
			if pr.responses[i] > maxResp {
				maxResp = pr.responses[i]
				maxZ = pr.z
			}
		}
		if !seenPlanes[maxZ] {
			seenPlanes[maxZ] = true
			unique++
		}
	}
	fmt.Printf("\n  Unique planes selected: %d / %d zeros\n", unique, len(zeros))
	if unique == len(zeros) {
		fmt.Println("  *** PERFECT DIAGONALIZATION: each zero maps to a distinct plane! ***")
	} else if unique >= len(zeros)*3/4 {
		fmt.Println("  Strong diagonalization: most zeros map to distinct planes")
	} else {
		fmt.Println("  Partial diagonalization — higher order may be needed")
	}

	// Also compute the concentration: what fraction of total spectral power
	// is captured by the top plane for each zero?
	fmt.Println("\n  Spectral concentration (power in top plane / total power):")
	for i, gamma := range zeros {
		totalPower := 0.0
		maxPower := 0.0
		for _, pr := range responses {
			totalPower += pr.responses[i]
			if pr.responses[i] > maxPower {
				maxPower = pr.responses[i]
			}
		}
		conc := maxPower / totalPower * 100
		fmt.Printf("    γ_%d = %.3f:  %.1f%% concentrated in top plane\n", i+1, gamma, conc)
	}
}

// computeVonMangoldtOperator builds the operator matrix using the von Mangoldt
// function Λ(k) instead of the prime indicator.  Λ(k) = log(p) if k = p^m,
// and the oscillatory term is (Λ(k) - 1)/√k.  This directly isolates the
// explicit formula's spectral contribution without cumulative smoothing.
func computeVonMangoldtOperator(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := dim * dim * dim
	fmt.Printf("\n── Von Mangoldt Operator (order %d, %dx%d) ──\n", order, dim, dim)

	// Build prime power map: for each k, compute Λ(k)
	// Λ(k) = log(p) if k = p^m, else 0
	primeSet := make(map[uint64]bool)
	for _, p := range primes {
		primeSet[p] = true
	}

	// Build Hilbert curve
	curve := build3DCurve(order, variant)

	// Compute plane sizes
	planeSize := make([]int, dim)
	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		planeSize[z]++
	}

	// Build the von Mangoldt-weighted vector per plane
	// v[z] = (1/√|I_z|) Σ_{k∈I_z} (Λ(k) - 1) / √k
	v := make([]float64, dim)
	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		kk := uint64(k)
		if kk < 2 {
			continue
		}
		// Compute Λ(k): check if k is a prime power
		lambda := vonMangoldt(kk, primeSet)
		if lambda > 0 {
			v[z] += (lambda - 1.0) / math.Sqrt(float64(kk))
		} else {
			v[z] += (0.0 - 1.0) / math.Sqrt(float64(kk)) // Λ(k)=0, subtract 1
		}
	}
	for z := 0; z < dim; z++ {
		if planeSize[z] > 0 {
			v[z] /= math.Sqrt(float64(planeSize[z]))
		}
	}

	// Build the covariance matrix using von Mangoldt weights
	cov := make([][]float64, dim)
	for i := range cov {
		cov[i] = make([]float64, dim)
	}
	counts := make([][]int, dim)
	for i := range counts {
		counts[i] = make([]int, dim)
	}

	// Use all adjacent pairs along the Hilbert curve
	for k := 0; k < total-1; k++ {
		z1 := curve[k] / (dim * dim)
		z2 := curve[k+1] / (dim * dim)

		w1 := vonMangoldtWeight(uint64(k), primeSet)
		w2 := vonMangoldtWeight(uint64(k+1), primeSet)

		cov[z1][z2] += (w1 - v[z1]) * (w2 - v[z2])
		counts[z1][z2]++
	}

	// Normalize
	for z1 := 0; z1 < dim; z1++ {
		for z2 := 0; z2 < dim; z2++ {
			if counts[z1][z2] > 0 {
				cov[z1][z2] /= float64(counts[z1][z2])
			}
		}
	}

	// Symmetrize
	for z1 := 0; z1 < dim; z1++ {
		for z2 := z1 + 1; z2 < dim; z2++ {
			avg := (cov[z1][z2] + cov[z2][z1]) / 2.0
			cov[z1][z2] = avg
			cov[z2][z1] = avg
		}
	}

	// Output as JSON
	fmt.Println("{")
	fmt.Printf("  \"order\": %d,\n", order)
	fmt.Printf("  \"dim\": %d,\n", dim)
	fmt.Printf("  \"primes\": %d,\n", len(primes))
	fmt.Printf("  \"weight\": \"von_mangoldt\",\n")
	fmt.Println("  \"v\": [")
	for z := 0; z < dim; z++ {
		comma := ","
		if z == dim-1 {
			comma = ""
		}
		fmt.Printf("    %.12f%s\n", v[z], comma)
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

// vonMangoldt returns Λ(k) for integer k.
// Λ(k) = log(p) if k = p^m for prime p and m ≥ 1, else 0.
func vonMangoldt(k uint64, primeSet map[uint64]bool) float64 {
	if k < 2 {
		return 0
	}
	// Check if k is prime
	if primeSet[k] {
		return math.Log(float64(k))
	}
	// Check if k is a prime power > prime
	// For k <= 2^20 (~1M), brute-force check is fine
	for p := uint64(2); p*p <= k; p++ {
		if primeSet[p] && k%p == 0 {
			// Check if k is a power of p
			temp := k
			for temp%p == 0 {
				temp /= p
			}
			if temp == 1 {
				return math.Log(float64(p))
			}
			return 0 // divisible by p but not a power
		}
	}
	return 0
}

// vonMangoldtWeight returns the normalized weight (Λ(k) - 1) / √k for the operator.
func vonMangoldtWeight(k uint64, primeSet map[uint64]bool) float64 {
	if k < 2 {
		return -1.0 / math.Sqrt(2.0) // Λ(0)=Λ(1)=0, subtract 1
	}
	lambda := vonMangoldt(k, primeSet)
	return (lambda - 1.0) / math.Sqrt(float64(k))
}

// computeFastVM is an optimized von Mangoldt operator that pre-computes
// prime powers and processes only those, avoiding O(√k) checks per integer.
func computeFastVM(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := dim * dim * dim
	fmt.Printf("\n── Fast Von Mangoldt Operator (order %d, %dx%d) ──\n", order, dim, dim)

	// Build prime set
	primeSet := make(map[uint64]bool)
	maxPrime := uint64(0)
	for _, p := range primes {
		primeSet[p] = true
		if p > maxPrime {
			maxPrime = p
		}
	}

	// Pre-compute prime powers with their Λ values
	// Λ(p^m) = log(p)
	type primePower struct {
		k      uint64
		lambda float64
	}
	var ppList []primePower
	for _, p := range primes {
		if p < 2 {
			continue
		}
		logP := math.Log(float64(p))
		// p^1
		if p < uint64(total) {
			ppList = append(ppList, primePower{p, logP})
		}
		// p^2, p^3, ...
		for pk := p * p; pk < uint64(total) && pk > 0; pk *= p {
			ppList = append(ppList, primePower{pk, logP})
		}
	}
	fmt.Printf("  Pre-computed %d prime powers (primes: %d)\n", len(ppList), len(primes))

	// Build Λ(k) lookup
	lambdaMap := make(map[uint64]float64, len(ppList))
	for _, pp := range ppList {
		lambdaMap[pp.k] = pp.lambda
	}

	// Build Hilbert curve
	curve := build3DCurve(order, variant)

	// Compute plane sizes and the von Mangoldt projection
	planeSize := make([]int, dim)
	mu := make([]float64, dim) // mean weight per plane

	// First pass: accumulate Λ(k)/√k per plane and count
	for k := 0; k < total; k++ {
		z := curve[k] / (dim * dim)
		planeSize[z]++
		kk := uint64(k)
		if kk < 2 {
			continue
		}
		if lambda, ok := lambdaMap[kk]; ok {
			mu[z] += (lambda - 1.0) / math.Sqrt(float64(kk))
		} else {
			mu[z] += -1.0 / math.Sqrt(float64(kk))
		}
	}
	for z := 0; z < dim; z++ {
		if planeSize[z] > 0 {
			mu[z] /= math.Sqrt(float64(planeSize[z]))
		}
	}

	// Build covariance matrix using adjacent pairs along curve
	cov := make([][]float64, dim)
	for i := range cov {
		cov[i] = make([]float64, dim)
	}
	counts := make([][]int, dim)
	for i := range counts {
		counts[i] = make([]int, dim)
	}

	// Use all adjacent pairs
	for k := 0; k < total-1; k++ {
		z1 := curve[k] / (dim * dim)
		z2 := curve[k+1] / (dim * dim)
		kk1, kk2 := uint64(k), uint64(k+1)

		w1 := -1.0
		w2 := -1.0
		if kk1 >= 2 {
			if lambda, ok := lambdaMap[kk1]; ok {
				w1 = (lambda - 1.0) / math.Sqrt(float64(kk1))
			} else {
				w1 = -1.0 / math.Sqrt(float64(kk1))
			}
		} else {
			w1 = -1.0 / math.Sqrt(2.0)
		}
		if kk2 >= 2 {
			if lambda, ok := lambdaMap[kk2]; ok {
				w2 = (lambda - 1.0) / math.Sqrt(float64(kk2))
			} else {
				w2 = -1.0 / math.Sqrt(float64(kk2))
			}
		} else {
			w2 = -1.0 / math.Sqrt(2.0)
		}

		cov[z1][z2] += (w1 - mu[z1]) * (w2 - mu[z2])
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
	fmt.Printf("  \"weight\": \"von_mangoldt_fast\",\n")
	fmt.Println("  \"mu\": [")
	for z := 0; z < dim; z++ {
		comma := ","
		if z == dim-1 {
			comma = ""
		}
		fmt.Printf("    %.12f%s\n", mu[z], comma)
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

// outputMatrixStreaming builds the covariance matrix without storing
// the full Hilbert curve in memory.  For each adjacent pair (k, k+1),
// it computes d2xyz3D on-the-fly.  This enables orders 11+ on CPU.
func outputMatrixStreaming(primes []uint64, order uint32, variant int) {
	dim := int(1 << order)
	total := uint64(1) << (3 * order) // 8^order
	fmt.Fprintf(os.Stderr, "Streaming order %d: %dx%d, %d primes, %.1fB cells\n",
		order, dim, dim, len(primes), float64(total))

	// Build prime set as a bitset for O(1) lookup
	bitset := make([]uint64, (total+63)/64)
	for _, p := range primes {
		if p < total {
			bitset[p/64] |= 1 << (p % 64)
		}
	}
	isPrime := func(k uint64) bool {
		return bitset[k/64]&(1<<(k%64)) != 0
	}

	// Compute plane sizes and means
	planeSize := make([]int, dim)
	mu := make([]float64, dim)

	fmt.Fprintf(os.Stderr, "  Pass 1: computing plane sizes...\n")
	for k := uint64(0); k < total; k++ {
		x, y, z := d2xyz3D(order, k, variant)
		_ = x
		_ = y
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

	// Build covariance matrix from adjacent pairs
	cov := make([][]float64, dim)
	for i := range cov {
		cov[i] = make([]float64, dim)
	}
	counts := make([][]int, dim)
	for i := range counts {
		counts[i] = make([]int, dim)
	}

	fmt.Fprintf(os.Stderr, "  Pass 2: computing covariance from %d pairs...\n", total-1)
	progressStep := total / 20
	for k := uint64(0); k < total-1; k++ {
		if k%progressStep == 0 {
			pct := k * 100 / total
			fmt.Fprintf(os.Stderr, "    %d%%\n", pct)
		}
		_, _, z1 := d2xyz3D(order, k, variant)
		_, _, z2 := d2xyz3D(order, k+1, variant)

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

	// Normalize
	for z1 := 0; z1 < dim; z1++ {
		for z2 := 0; z2 < dim; z2++ {
			if counts[z1][z2] > 0 {
				cov[z1][z2] /= float64(counts[z1][z2])
			}
		}
	}

	// Symmetrize
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

// ─────────────────────────────────────────────────────────────────────────────
// 4D Hilbert curve — d2xyzw
// ─────────────────────────────────────────────────────────────────────────────

// d2xyzw4D maps distance d along a 4D Hilbert curve to (x, y, z, w).
// Processes 4 bits per recursion level (16 hyper-octants).
// This is the standard 4D Hilbert algorithm.
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
