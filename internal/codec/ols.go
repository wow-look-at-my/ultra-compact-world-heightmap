// ols.go holds the online least-squares predictor.

package codec

import "math"

// ols fits a linear predictor to the samples it has seen, and forgets old
// samples at a fixed rate.
type ols struct {
	n      int
	lambda float64
	nu     float64
	cov    []float64
	cross  []float64
	chol   []float64
	w      []float64
	tmp    []float64
	feat   []float64
	every  int
	since  int
}

func newOLS(n int, lambda, nu float64, every int) *ols {
	return &ols{
		n: n, lambda: lambda, nu: nu, every: every,
		cov: make([]float64, n*n), cross: make([]float64, n), chol: make([]float64, n*n),
		w: make([]float64, n), tmp: make([]float64, n), feat: make([]float64, n),
	}
}

func (o *ols) predict() float64 {
	s := 0.0
	for i, f := range o.feat {
		s += float64(o.w[i] * f)
	}
	return s
}

func (o *ols) update(target float64) {
	n, lam := o.n, o.lambda
	gain := 1 - lam
	for j := 0; j < n; j++ {
		fj := float64(gain * o.feat[j])
		row := o.cov[j*n : j*n+j+1]
		for i := range row {
			row[i] = float64(lam*row[i]) + float64(fj*o.feat[i])
		}
		o.cross[j] = float64(lam*o.cross[j]) + float64(fj*target)
	}
	o.since++
	if o.since >= o.every {
		o.since = 0
		o.solve()
	}
}

// solve factors the covariance and replaces the weights. It keeps the weights
// when the matrix is not positive definite.
func (o *ols) solve() {
	n := o.n
	trace := 0.0
	for j := 0; j < n; j++ {
		trace += o.cov[j*n+j]
	}
	reg := float64(o.nu*trace)/float64(n) + 1e-9
	l := o.chol
	for j := 0; j < n; j++ {
		sum := o.cov[j*n+j] + reg
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
			s := o.cov[i*n+j]
			li := l[i*n : i*n+j]
			for k, v := range lj {
				s -= float64(li[k] * v)
			}
			l[i*n+j] = s / d
		}
	}
	y := o.tmp
	for i := 0; i < n; i++ {
		s := o.cross[i]
		li := l[i*n : i*n+i]
		for k, v := range li {
			s -= float64(v * y[k])
		}
		y[i] = s / l[i*n+i]
	}
	for i := n - 1; i >= 0; i-- {
		s := y[i]
		for k := i + 1; k < n; k++ {
			s -= float64(l[k*n+i] * o.w[k])
		}
		o.w[i] = s / l[i*n+i]
	}
}
