// Package warp reprojects a grid of heights in degrees to a Web Mercator tile. As the
// Mapterhorn pipeline does with gdalwarp and a cubic B-spline.
//
// A decoder repeats this arithmetic, so every machine must compute the same bits. The
// code uses only operations the rounds of a later IEEE exactly. The code wraps each
// product in a float64 conversion so that no compiler fuses it with an add.
package warp

import (
	"fmt"
	"math"
)

const (
	earthRadius = 6378137.0
	halfWorld   = math.Pi * earthRadius
)

// A Grid holds heights in metres on a grid of longitude and latitude.
type Grid struct {
	Width, Height int
	// West and North are the longitude and the latitude of the centre of the first sample, in degrees.
	West, North      float64
	StepLon, StepLat float64
	Heights          []float32
}

// exp gives e to the power of x from a short series and repeated squares.
func exp(x float64) float64 {
	const halvings = 10
	r := x / (1 << halvings)
	sum, term := 1.0, 1.0
	for n := 1; n <= 14; n++ {
		term = float64(term*r) / float64(n)
		sum += term
	}
	for i := 0; i < halvings; i++ {
		sum = float64(sum * sum)
	}
	return sum
}

// atan gives the arc tangent of x. It halves the angle until a short series is exact.
func atan(x float64) float64 {
	if x < 0 {
		return -atan(-x)
	}
	if x > 1 {
		return math.Pi/2 - atan(1/x)
	}
	const halvings = 4
	for i := 0; i < halvings; i++ {
		x = x / (1 + math.Sqrt(1+float64(x*x)))
	}
	square := float64(x * x)
	sum, power := 0.0, x
	for n := 0; n < 12; n++ {
		term := power / float64(2*n+1)
		if n%2 == 1 {
			term = -term
		}
		sum += term
		power = float64(power * square)
	}
	return sum * (1 << halvings)
}

// latitude gives the latitude in degrees of a Web Mercator northing in metres.
func latitude(northing float64) float64 {
	e := exp(northing / earthRadius)
	return float64(atan((e-1/e)/2)*180) / math.Pi
}

// spline gives the cubic B-spline weights of the samples around a place that lies t of
// the way from the second to the third.
func spline(t float64) [4]float64 {
	t2 := float64(t * t)
	t3 := float64(t2 * t)
	u := 1 - t
	return [4]float64{
		float64(float64(u*u)*u) / 6,
		(float64(3*t3) - float64(6*t2) + 4) / 6,
		(float64(-3*t3) + float64(3*t2) + float64(3*t) + 1) / 6,
		t3 / 6,
	}
}

// axis holds, for each sample along one side of a tile, the first of its grid
// samples and their weights.
type axis struct {
	first   []int
	weights [][4]float64
}

func newAxis(side, limit int, place func(i int) float64) (*axis, error) {
	a := &axis{first: make([]int, side), weights: make([][4]float64, side)}
	for i := range a.first {
		f := place(i)
		whole := math.Floor(f)
		if !(whole >= 1 && whole <= float64(limit-3)) {
			return nil, fmt.Errorf("the grid does not reach around the tile: sample %d lies at %.3f of %d grid samples", i, f, limit)
		}
		a.first[i] = int(whole) - 1
		a.weights[i] = spline(f - whole)
	}
	return a, nil
}

// Tile gives the heights of one tile of the given zoom, column and row, with side
// samples on a side.
func Tile(g *Grid, zoom, x, y, side int) ([]float32, error) {
	if g.Width < 4 || g.Height < 4 || len(g.Heights) != g.Width*g.Height || !(g.StepLon > 0) || !(g.StepLat > 0) {
		return nil, fmt.Errorf("a grid of %dx%d with %d heights and steps of %v and %v degrees cannot be warped", g.Width, g.Height, len(g.Heights), g.StepLon, g.StepLat)
	}
	if zoom < 0 || zoom > 24 || side < 1 || x < 0 || y < 0 || x >= 1<<zoom || y >= 1<<zoom {
		return nil, fmt.Errorf("%d/%d/%d is not a tile", zoom, x, y)
	}
	span := 2 * halfWorld / float64(int64(1)<<zoom)
	res := span / float64(side)
	left := -halfWorld + float64(float64(x)*span)
	top := halfWorld - float64(float64(y)*span)
	columns, err := newAxis(side, g.Width, func(c int) float64 {
		easting := left + float64((float64(c)+0.5)*res)
		lon := float64(easting/earthRadius*180) / math.Pi
		return (lon - g.West) / g.StepLon
	})
	if err != nil {
		return nil, err
	}
	rows, err := newAxis(side, g.Height, func(r int) float64 {
		return (g.North - latitude(top-float64((float64(r)+0.5)*res))) / g.StepLat
	})
	if err != nil {
		return nil, err
	}
	out := make([]float32, side*side)
	for r := 0; r < side; r++ {
		wy := rows.weights[r]
		for c := 0; c < side; c++ {
			wx := columns.weights[c]
			v := 0.0
			for b := 0; b < 4; b++ {
				line := g.Heights[(rows.first[r]+b)*g.Width+columns.first[c]:]
				sum := 0.0
				for a := 0; a < 4; a++ {
					sum += float64(wx[a] * float64(line[a]))
				}
				v += float64(wy[b] * sum)
			}
			out[r*side+c] = float32(v)
		}
	}
	return out, nil
}
