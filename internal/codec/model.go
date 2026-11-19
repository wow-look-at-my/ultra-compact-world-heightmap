// model.go predicts each sample from the samples before it and codes the error.

package codec

import (
	"math"
	"math/bits"
)

// margin is the border of copied samples that lets every neighbour read stay in bounds.
const margin = 3

// neighbours lists the causal neighbourhood by distance. The first is the reference
// that the least-squares features are taken against.
var neighbours = [...][2]int{
	{-1, 0}, {0, -1}, {-1, -1}, {1, -1}, {-2, 0}, {0, -2}, {-2, -1}, {2, -1}, {-1, -2}, {1, -2}, {-3, 0}, {0, -3},
	{-2, -2}, {2, -2}, {-3, -1}, {3, -1}, {-1, -3}, {1, -3}, {-3, -2}, {3, -2}, {-2, -3}, {2, -3}, {-3, -3}, {3, -3},
}

const fixedPredictors = 6

// linearFit is a predictor that learns its weights from the samples already coded.
type linearFit interface {
	features() []float64
	predict() float64
	update(target float64)
	startRow(first bool)
}

type fitted struct {
	fit     linearFit
	offsets []int
}

type model struct {
	rc           *residualCoder
	w, h, stride int
	img          []int32
	errs         []int32
	sq           [][]float32
	fits         []fitted
	pred         []float64
	costs        []float32
}

func newModel(c *coder, w, h int, base int32) *model {
	m := &model{rc: newResidualCoder(c), w: w, h: h, stride: w + 2*margin}
	size := m.stride * (h + margin)
	m.img = make([]int32, size)
	for i := range m.img {
		m.img[i] = base
	}
	m.errs = make([]int32, size)
	add := func(n int, fit linearFit) {
		f := fitted{fit: fit}
		for _, d := range neighbours[:n] {
			f.offsets = append(f.offsets, d[1]*m.stride+d[0])
		}
		m.fits = append(m.fits, f)
	}
	for i, spec := range []struct {
		n      float64
		lambda float64
	}{{7, 0.99}, {13, 0.998}, {24, 0.9995}} {
		id := string(rune('1' + i))
		if n := int(knob("r"+id+"n", spec.n)); n > 1 {
			add(n, newOLS(n-1, knob("r"+id+"l", spec.lambda), knob("rnu", 0.0005), 1))
		}
	}
	plane := []float64{1, -1}
	for i, spec := range []struct {
		n, radius, rows, row, gain float64
	}{{0, 8, 0.85, 0.85, 1}, {0, 12, 0.9, 0.9, 1}} {
		id := string(rune('1' + i))
		if n := int(knob("w"+id+"n", spec.n)); n > 2 {
			add(n, newWindowFit(n-1, w, int(knob("w"+id+"r", spec.radius)), knob("w"+id+"v", spec.rows), knob("w"+id+"h", spec.row), knob("w"+id+"g", spec.gain), knob("wnu", 0.0005), plane))
		}
	}
	m.pred = make([]float64, fixedPredictors+len(m.fits))
	m.sq = make([][]float32, len(m.pred))
	for k := range m.sq {
		m.sq[k] = make([]float32, size)
	}
	return m
}

func absInt(v int32) uint32 {
	if v < 0 {
		return uint32(-v)
	}
	return uint32(v)
}

// relative puts a signed difference into several levels against the local error scale.
func relative(d float64, scale8 uint32) int {
	t := float64(math.Abs(d) * 8)
	s := float64(scale8 + 2)
	level := 3
	switch {
	case float64(t*4) < s:
		return 3
	case t < s:
		level = 1
	case t < float64(s*3):
		level = 2
	}
	if d < 0 {
		return 3 - level
	}
	return 3 + level
}

// run codes every sample. src holds the samples when encoding and is nil when decoding.
func (m *model) run(src []int32) {
	s := m.stride
	img, errs := m.img, m.errs
	for y := 0; y < m.h; y++ {
		row := (y+margin)*s + margin
		for k := 1; k <= margin; k++ {
			img[row-k] = img[row-s]
		}
		for _, f := range m.fits {
			f.fit.startRow(y == 0)
		}
		for x := 0; x < m.w; x++ {
			i := row + x
			w, n, nw, ne := img[i-1], img[i-s], img[i-s-1], img[i-s+1]
			ww, nn := img[i-2], img[i-2*s]
			p := m.pred
			p[0] = float64(w)
			p[1] = float64(n)
			p[2] = float64(w + n - nw)
			p[3] = float64(2*w - ww)
			p[4] = float64(2*n - nn)
			p[5] = float64(w + ne - n)
			for k, f := range m.fits {
				ref := img[i+f.offsets[0]]
				feat := f.fit.features()
				for j, off := range f.offsets[1:] {
					feat[j] = float64(img[i+off] - ref)
				}
				p[fixedPredictors+k] = float64(ref) + f.fit.predict()
			}
			num, den := 0.0, 0.0
			for k, sq := range m.sq {
				e := float64(sq[i-1]) + float64(sq[i-s]) + float64(0.5*(float64(sq[i-s-1])+float64(sq[i-s+1])+float64(sq[i-2])+float64(sq[i-2*s]))) + 0.02
				wk := 1 / float64(e*e)
				num += float64(wk * p[k])
				den += wk
			}
			mu := num / den
			mid := math.Floor(mu + 0.5)
			frac := mu - mid
			center := int32(mid)

			ew, en := errs[i-1], errs[i-s]
			scale8 := 2*absInt(ew) + 2*absInt(en) + absInt(errs[i-s-1]) + absInt(errs[i-s+1]) + absInt(errs[i-2]) + absInt(errs[i-2*s])
			kq := min(quarterLog(scale8), scaleLevels-1)
			kc := kq / 3
			grad := absInt(w-nw) + absInt(n-nw) + absInt(n-ne) + absInt(w-ww)
			equal := b2i(w == n) | b2i(n == nw)<<1 | b2i(n == ne)<<2 | b2i(w == ww)<<3 | int(center&1)<<4
			rc := m.rc
			rc.riceBits = uint(bits.Len32(scale8 >> 4))
			rc.mixSet = kc
			rc.ctx[0] = kq
			rc.ctx[1] = kc*16 + min(max(int(float64((frac+0.5)*16)), 0), 15)
			rc.ctx[2] = kc*49 + relative(float64(ew), scale8)*7 + relative(float64(en), scale8)
			rc.ctx[3] = kc*49 + relative(p[2]-mu, scale8)*7 + relative(p[len(p)-1]-mu, scale8)
			rc.ctx[4] = kc*32 + equal
			rc.ctx[5] = kc*32 + min(quarterLog(grad)/2, 31)

			e := 0
			if src != nil {
				e = int(src[y*m.w+x] - center)
			}
			e = rc.code(e)
			if rc.probing {
				m.costs[y*m.w+x] = float32(rc.cost)
				rc.cost = 0
			}
			v := center + int32(e)
			img[i] = v
			errs[i] = int32(e)
			fv := float64(v)
			for k, sq := range m.sq {
				d := fv - p[k]
				sq[i] = float32(d * d)
			}
			for _, f := range m.fits {
				f.fit.update(float64(v - img[i+f.offsets[0]]))
			}
		}
		for k := 0; k < margin; k++ {
			img[row+m.w+k] = img[row+m.w-1]
		}
	}
}
