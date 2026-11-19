// window.go holds the least-squares predictor that trains on a window above and beside the sample.

package codec

import "math"

// windowFit fits a linear predictor to the samples in a window around the
// sample in hand. This covers the rows above within a column radius, and the
// samples to the left in its own row.
type windowFit struct {
	n, width, radius int
	block            int
	decayRows        float64
	decayRow         float64
	rowGain          float64
	nu               float64
	prior            []float64

	prev, next []float64
	win, row   []float64
	full       []float64
	chol       []float64
	w, tmp     []float64
	feat       []float64
	x          int
}

func newWindowFit(n, width, radius int, decayRows, decayRow, rowGain, nu float64, prior []float64) *windowFit {
	block := n*(n+1)/2 + n
	f := &windowFit{
		n: n, width: width, radius: radius, block: block,
		decayRows: decayRows, decayRow: decayRow, rowGain: rowGain, nu: nu,
		prior: make([]float64, n),
		prev:  make([]float64, width*block), next: make([]float64, width*block),
		win: make([]float64, block), row: make([]float64, block),
		full: make([]float64, n*n), chol: make([]float64, n*n),
		w: make([]float64, n), tmp: make([]float64, n), feat: make([]float64, n),
	}
	copy(f.prior, prior)
	copy(f.w, prior)
	return f
}

func (f *windowFit) features() []float64 { return f.feat }

// startRow moves to the next row. The first call opens the first row.
func (f *windowFit) startRow(first bool) {
	if !first {
		f.prev, f.next = f.next, f.prev
	}
	f.x = 0
	clear(f.row)
	clear(f.win)
	for c := 0; c <= f.radius && c < f.width; c++ {
		col := f.prev[c*f.block : (c+1)*f.block]
		for i, v := range col {
			f.win[i] += v
		}
	}
}

// predict solves for the weights at the sample in hand and applies them to feat.
func (f *windowFit) predict() float64 {
	f.solve()
	s := 0.0
	for i, v := range f.feat {
		s += float64(f.w[i] * v)
	}
	return s
}

// update records the sample that feat predicted, and moves the window one column on.
func (f *windowFit) update(target float64) {
	n, b := f.n, f.block
	prev := f.prev[f.x*b : (f.x+1)*b]
	next := f.next[f.x*b : (f.x+1)*b]
	k := 0
	for j := 0; j < n; j++ {
		fj := f.feat[j]
		for i := 0; i <= j; i++ {
			p := float64(fj * f.feat[i])
			next[k] = float64(f.decayRows*prev[k]) + p
			f.row[k] = float64(f.decayRow*f.row[k]) + p
			k++
		}
	}
	for j := 0; j < n; j++ {
		p := float64(f.feat[j] * target)
		next[k] = float64(f.decayRows*prev[k]) + p
		f.row[k] = float64(f.decayRow*f.row[k]) + p
		k++
	}
	if in := f.x + f.radius + 1; in < f.width {
		for i, v := range f.prev[in*b : (in+1)*b] {
			f.win[i] += v
		}
	}
	if out := f.x - f.radius; out >= 0 {
		for i, v := range f.prev[out*b : (out+1)*b] {
			f.win[i] -= v
		}
	}
	f.x++
}

// solve keeps the weights it has when the window does not determine new ones.
func (f *windowFit) solve() {
	n := f.n
	a := f.full
	k := 0
	trace := 0.0
	for j := 0; j < n; j++ {
		for i := 0; i <= j; i++ {
			a[j*n+i] = f.win[k] + float64(f.rowGain*f.row[k])
			k++
		}
		trace += a[j*n+j]
	}
	reg := float64(f.nu*trace)/float64(n) + 1e-6
	l := f.chol
	for j := 0; j < n; j++ {
		sum := a[j*n+j] + reg
		lj := l[j*n : j*n+j]
		for _, v := range lj {
			sum -= float64(v * v)
		}
		if sum <= 1e-12 {
			return
		}
		d := math.Sqrt(sum)
		l[j*n+j] = d
		for i := j + 1; i < n; i++ {
			s := a[i*n+j]
			li := l[i*n : i*n+j]
			for q, v := range lj {
				s -= float64(li[q] * v)
			}
			l[i*n+j] = s / d
		}
	}
	y := f.tmp
	for i := 0; i < n; i++ {
		s := f.win[k+i] + float64(f.rowGain*f.row[k+i]) + float64(reg*f.prior[i])
		li := l[i*n : i*n+i]
		for q, v := range li {
			s -= float64(v * y[q])
		}
		y[i] = s / l[i*n+i]
	}
	for i := n - 1; i >= 0; i-- {
		s := y[i]
		for q := i + 1; q < n; q++ {
			s -= float64(l[q*n+i] * f.w[q])
		}
		f.w[i] = s / l[i*n+i]
	}
}
