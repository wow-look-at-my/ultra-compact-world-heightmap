// smooth.go turns the parent tile into smooth surfaces at the resolution of the tile below it.

package codec

// A kernel fits a polynomial through a run of parent samples by least squares.
type kernel [][3]float64

var kernels = []kernel{
	// A line through samples.
	{{1.0 / 3, -0.5, 0}, {1.0 / 3, 0, 0}, {1.0 / 3, 0.5, 0}},
	// The parabola through samples.
	{{0, -0.5, 0.5}, {1, 0, -1}, {0, 0.5, 0.5}},
	// A parabola fitted to samples.
	{{-3.0 / 35, -0.2, 2.0 / 14}, {12.0 / 35, -0.1, -1.0 / 14}, {17.0 / 35, 0, -2.0 / 14}, {12.0 / 35, 0.1, -1.0 / 14}, {-3.0 / 35, 0.2, 2.0 / 14}},
	// A parabola fitted to samples.
	{{-2.0 / 21, -3.0 / 28, 5.0 / 84}, {3.0 / 21, -2.0 / 28, 0}, {6.0 / 21, -1.0 / 28, -3.0 / 84}, {7.0 / 21, 0, -4.0 / 84}, {6.0 / 21, 1.0 / 28, -3.0 / 84}, {3.0 / 21, 2.0 / 28, 0}, {-2.0 / 21, 3.0 / 28, 5.0 / 84}},
}

// taps gives the weights of the kernel for the sample on one side of the middle.
func (k kernel) taps(side float64) []float64 {
	t := float64(side * 0.25)
	out := make([]float64, len(k))
	for i, c := range k {
		out[i] = c[0] + float64(c[1]*t) + float64(c[2]*float64(t*t))
	}
	return out
}

// surface applies the kernel along both axes of the parent. The result has the layout
// of the padded tile, so one index reads a sample and the surface under it. Each
// product is rounded on its own, so that every platform builds the same surface.
func (g *parentGrid) surface(k kernel, w, h, stride int) []float64 {
	half := len(k) / 2
	sides := [2][]float64{k.taps(-1), k.taps(1)}
	rows := h/2 + 2*parentMargin
	// across holds each padded parent row at the width of the tile.
	across := make([]float64, rows*w)
	for r := 0; r < rows; r++ {
		for x := 0; x < w; x++ {
			at := r*g.stride + x/2 + parentMargin - half
			s := 0.0
			for j, c := range sides[x&1] {
				s += float64(c * float64(g.mean[at+j]))
			}
			across[r*w+x] = s
		}
	}
	out := make([]float64, stride*(h+margin))
	for py := 0; py < h+margin; py++ {
		y := min(max(py-margin, 0), h-1)
		taps := sides[y&1]
		top := y/2 + parentMargin - half
		for px := 0; px < stride; px++ {
			x := min(max(px-margin, 0), w-1)
			s := 0.0
			for j, c := range taps {
				s += float64(c * across[(top+j)*w+x])
			}
			out[py*stride+px] = s
		}
	}
	return out
}
