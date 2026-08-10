// primecube — exact, gap-free 2D renders of the prime-Hilbert-curve cube.
//
// The rotating-cube movie (main.go -movie) projects the 3D grid through a
// rotation and an orthographic camera, so it necessarily maps `dim` distinct
// integer coordinates onto a wider pixel canvas -- most rotation angles
// leave empty pixel-columns that don't correspond to any grid cell at all
// (see the picket-fence artifact at angleY=0). This tool renders flat,
// unrotated Z-slices instead: one pixel per grid cell, exactly, with no
// scaling and no projection, so "empty" in the image always means composite,
// never "no corresponding integer."
//
// It also has a -movie mode for a genuine rotating-cube render. That one
// necessarily goes back through rotation + orthographic projection (a
// spinning view can't be gap-free in the same sense a flat slice can), so it
// reuses the corrected renderCubeView from main.go verbatim: a per-pixel
// z-buffer (keep the nearest depth per pixel instead of whatever was drawn
// last) and a clamped depth-to-brightness conversion (composing the two
// rotations pushes the depth value outside [0,1], and casting an
// out-of-range float to uint8 in Go wraps instead of clamping -- that wrap
// is what produced the dark "hole" artifact in the original renderer).
//
// Usage:
//
//	primecube -n 6 -slice 10 -out slice10.png   # a single Z-plane, dim x dim pixels
//	primecube -n 6 -out cube.png                # every Z-plane tiled into one montage image
//	primecube -n 6 -movie -movie-out cube.mp4   # rotating cube movie
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
)

// ─── Prime sieve (identical to main.go's parallelSieve/simpleSieve) ───────

func parallelSieve(limit uint64) []uint64 {
	if limit < 2 {
		return nil
	}
	sqrtLimit := uint64(math.Sqrt(float64(limit))) + 1
	small := simpleSieve(sqrtLimit)

	segSize := uint64(1 << 18)
	workers := runtime.NumCPU()

	var mu sync.Mutex
	var allPrimes []uint64
	allPrimes = append(allPrimes, small...)

	var wg sync.WaitGroup
	ch := make(chan struct{ lo, hi uint64 }, workers*2)

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

// ─── 3D Hilbert curve (identical to main.go's build3DCurve/d2xyz3D) ───────

var cubeSymmetries = func() [48]struct {
	perm [3]int
	sign [3]int
} {
	var syms [48]struct {
		perm [3]int
		sign [3]int
	}
	perms := [6][3]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}
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

func d2xyz3D(order uint32, d uint64, variant int) (x, y, z uint32) {
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

	sym := cubeSymmetries[variant%48]
	dim := n
	coords := [3]uint32{x, y, z}
	nx := coords[sym.perm[0]]
	ny := coords[sym.perm[1]]
	nz := coords[sym.perm[2]]
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

func build3DCurve(order uint32, variant int) []int {
	dim := uint32(1 << order)
	total := dim * dim * dim
	curve := make([]int, int(total))

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

// ─── Rendering: one pixel per integer, no scale factor, no rotation ───────

func save(img *image.Gray, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()
	png.Encode(f, img)
}

// renderSlice draws one flat Z-plane at exact 1:1 scale: dim x dim pixels,
// pixel (x,y) lit iff the integer at grid cell (x,y,z) is prime.
func renderSlice(isPrimeAtGrid []bool, dim, z int, out string) {
	img := image.NewGray(image.Rect(0, 0, dim, dim))
	base := z * dim * dim
	for y := 0; y < dim; y++ {
		row := base + y*dim
		for x := 0; x < dim; x++ {
			c := uint8(0)
			if isPrimeAtGrid[row+x] {
				c = 255
			}
			img.SetGray(x, y, color.Gray{Y: c})
		}
	}
	save(img, out)
	fmt.Printf("wrote %s (%dx%d px, z=%d, 1 pixel = 1 integer, no spacing)\n", out, dim, dim, z)
}

// renderMontage tiles every Z-slice into one image, each tile at exact 1:1
// scale, so the whole cube is visible with no scaling or gaps anywhere.
func renderMontage(isPrimeAtGrid []bool, dim int, out string) {
	cols := int(math.Ceil(math.Sqrt(float64(dim))))
	rows := (dim + cols - 1) / cols
	img := image.NewGray(image.Rect(0, 0, dim*cols, dim*rows))

	for z := 0; z < dim; z++ {
		tileX := (z % cols) * dim
		tileY := (z / cols) * dim
		base := z * dim * dim
		for y := 0; y < dim; y++ {
			row := base + y*dim
			for x := 0; x < dim; x++ {
				c := uint8(0)
				if isPrimeAtGrid[row+x] {
					c = 255
				}
				img.SetGray(tileX+x, tileY+y, color.Gray{Y: c})
			}
		}
	}
	save(img, out)
	fmt.Printf("wrote %s (%dx%d px = %d slices of %dx%d tiled %dx%d, 1 pixel = 1 integer everywhere)\n",
		out, dim*cols, dim*rows, dim, dim, dim, cols, rows)
}

// renderCubeView is main.go's renderCubeView with both fixes applied:
//   - a per-pixel z-buffer, so the nearest point wins at each pixel instead
//     of whichever grid-z happened to be iterated last (the original code's
//     "painter's algorithm approximation" iterated fixed grid order, which
//     does not match rotated depth order at most angles);
//   - a clamped depth before the uint8 brightness conversion, since composing
//     the Y and X rotations pushes depth outside [0,1] for ~8% of points,
//     and an unclamped out-of-range float wraps around under uint8() instead
//     of saturating, turning the nearest (brightest) points black.
func renderCubeView(isPrimeAtGrid []bool, dim int, angleX, angleY float64, outSize int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, outSize, outSize))
	for y := 0; y < outSize; y++ {
		for x := 0; x < outSize; x++ {
			img.SetGray(x, y, color.Gray{Y: 0})
		}
	}

	cosY, sinY := math.Cos(angleY), math.Sin(angleY)
	cosX, sinX := math.Cos(angleX), math.Sin(angleX)
	half := float64(dim) / 2.0
	scale := float64(outSize) / (float64(dim) * 1.8)

	depthBuf := make([]float64, outSize*outSize)
	for i := range depthBuf {
		depthBuf[i] = math.Inf(-1)
	}

	for z := 0; z < dim; z++ {
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				idx := z*dim*dim + y*dim + x
				if !isPrimeAtGrid[idx] {
					continue
				}

				cx := float64(x) - half
				cy := float64(y) - half
				cz := float64(z) - half

				rx := cx*cosY + cz*sinY
				rz := -cx*sinY + cz*cosY
				ry := cy

				ry2 := ry*cosX - rz*sinX
				rz2 := ry*sinX + rz*cosX

				px := int(rx*scale + float64(outSize)/2)
				py := int(ry2*scale + float64(outSize)/2)

				if px >= 0 && px < outSize && py >= 0 && py < outSize {
					pi := py*outSize + px
					if rz2 > depthBuf[pi] {
						depthBuf[pi] = rz2
						depth := (rz2 + half) / float64(dim)
						if depth < 0 {
							depth = 0
						} else if depth > 1 {
							depth = 1
						}
						bright := uint8(128 + depth*127)
						img.SetGray(px, py, color.Gray{Y: bright})
					}
				}
			}
		}
	}
	return img
}

func renderMovie(isPrimeAtGrid []bool, dim int, outPath string, numFrames int) {
	fmt.Fprintf(os.Stderr, "rendering rotating cube movie (%d frames)...\n", numFrames)

	tmpDir, err := os.MkdirTemp("", "primecube-movie-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir error: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	outSize := 512
	angleX := math.Pi / 6.0 // slight fixed tilt, same as main.go's movie

	var pngFiles []string
	for frame := 0; frame < numFrames; frame++ {
		angleY := float64(frame) * 2.0 * math.Pi / float64(numFrames)
		img := renderCubeView(isPrimeAtGrid, dim, angleX, angleY, outSize)

		fn := filepath.Join(tmpDir, fmt.Sprintf("frame_%04d.png", frame))
		f, err := os.Create(fn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create frame: %v\n", err)
			os.Exit(1)
		}
		png.Encode(f, img)
		f.Close()
		pngFiles = append(pngFiles, fn)

		if numFrames >= 10 && frame%(numFrames/10) == 0 {
			fmt.Fprintf(os.Stderr, "  frame %d/%d\n", frame, numFrames)
		}
	}

	concatFile := filepath.Join(tmpDir, "concat.txt")
	var lines string
	for _, p := range pngFiles {
		lines += fmt.Sprintf("file '%s'\n", p)
	}
	if err := os.WriteFile(concatFile, []byte(lines), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write concat file: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("ffmpeg",
		"-y", "-f", "concat", "-safe", "0", "-r", "30",
		"-i", concatFile,
		"-vf", fmt.Sprintf("scale=%d:%d", outSize, outSize),
		"-pix_fmt", "yuv420p",
		outPath,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d frames, rotating cube, z-buffered + depth-clamped)\n", outPath, numFrames)
}

func main() {
	order := flag.Int("n", 6, "Hilbert curve order (grid = 2^n per side, 8^n cells total)")
	variant := flag.Int("variant", 0, "Hilbert curve variant (0-47)")
	slice := flag.Int("slice", -1, "render a single Z-slice 0..dim-1 (default: montage of every slice)")
	out := flag.String("out", "primecube.png", "output PNG path")
	movie := flag.Bool("movie", false, "render a rotating cube movie instead of a static image")
	movieFrames := flag.Int("movie-frames", 180, "number of rotation frames")
	movieOut := flag.String("movie-out", "primecube.mp4", "movie output path (used with -movie)")
	flag.Parse()

	order32 := uint32(*order)
	dim := 1 << order32
	total := dim * dim * dim

	fmt.Fprintf(os.Stderr, "order=%d dim=%d total_cells=%d\n", *order, dim, total)

	fmt.Fprintf(os.Stderr, "building Hilbert curve (variant %d)...\n", *variant)
	curve := build3DCurve(order32, *variant) // curve[d] = grid index for Hilbert distance d

	fmt.Fprintf(os.Stderr, "sieving primes < %d...\n", total)
	primes := parallelSieve(uint64(total))
	fmt.Fprintf(os.Stderr, "found %d primes\n", len(primes))

	// isPrimeByHilbertDistance[d] = true iff integer d is prime
	isPrimeByDist := make([]bool, total)
	for _, p := range primes {
		if p < uint64(total) {
			isPrimeByDist[p] = true
		}
	}

	// Re-index by grid cell: isPrimeAtGrid[gridIdx] = isPrimeByDist[d] where curve[d] = gridIdx
	isPrimeAtGrid := make([]bool, total)
	for d, gridIdx := range curve {
		isPrimeAtGrid[gridIdx] = isPrimeByDist[d]
	}

	if *movie {
		renderMovie(isPrimeAtGrid, dim, *movieOut, *movieFrames)
		return
	}
	if *slice >= 0 {
		if *slice >= dim {
			fmt.Fprintf(os.Stderr, "slice %d out of range (dim=%d)\n", *slice, dim)
			os.Exit(1)
		}
		renderSlice(isPrimeAtGrid, dim, *slice, *out)
		return
	}
	renderMontage(isPrimeAtGrid, dim, *out)
}
