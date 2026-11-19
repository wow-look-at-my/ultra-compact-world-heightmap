// model.go predicts each sample from the samples before it and codes the error.

package codec

import (
	"math"
	"math/bits"
)

const (
	// margin is the border of copied samples that lets every neighbour read stay in bounds.
	margin       = 3
	parentMargin = 3
	// maxBlockStep bounds the sums that a parent sample allows for its block.
	maxBlockStep = 16
)

// neighbours lists the causal neighbourhood of a sample by distance, as column and row.
var neighbours = [...][2]int{
	{-1, 0}, {0, -1}, {-1, -1}, {1, -1}, {-2, 0}, {0, -2}, {-2, -1}, {2, -1}, {-1, -2}, {1, -2}, {-3, 0}, {0, -3},
	{-2, -2}, {2, -2}, {-3, -1}, {3, -1}, {-1, -3}, {1, -3}, {-3, -2}, {3, -2}, {-2, -3}, {2, -3}, {-3, -3}, {3, -3},
}

// parentNeighbours lists the parent samples around the one that covers a sample.
var parentNeighbours = [...][2]int{{1, 0}, {0, 1}, {1, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}, {-1, -1}, {2, 0}, {0, 2}, {2, 1}, {1, 2}}

// parentGrid is the part of the parent tile over the tile in hand, in the
// units of the tile in hand.
type parentGrid struct {
	stride int
	blocks int
	mean   []int32
	// lo and hi bound the sum of the samples of each block.
	lo, hi []int32
}

// linearFit is a predictor that learns its weights from the samples already coded.
type linearFit interface {
	features() []float64
	predict() float64
	update(target float64)
	startRow(first bool)
}

type fitted struct {
	fit    linearFit
	child  []int
	parent []int
	// With a surface, the fit predicts how far the sample lies from the surface, from how far its neighbours do.
	surface []float64
	// extra adds the height of each surface over the sample to the features.
	extra [][]float64
}

type model struct {
	rc           *residualCoder
	w, h, stride int
	img          []int32
	errs         []int32
	sq           [][]float32
	up           *parentGrid
	phases       int
	fixed        int
	fits         [4][]fitted
	surfaces     [][]float64
	pred         []float64
	costs        []float32
}

func newModel(c *coder, w, h int, base int32, up *parentGrid) *model {
	m := &model{w: w, h: h, stride: w + 2*margin, up: up, phases: 1, fixed: 6}
	size := m.stride * (h + margin)
	m.img = make([]int32, size)
	for i := range m.img {
		m.img[i] = base
	}
	m.errs = make([]int32, size)
	if up == nil {
		m.standaloneFits()
	} else {
		for _, k := range kernels[:int(knob("sk", float64(len(kernels))))] {
			m.surfaces = append(m.surfaces, up.surface(k, w, h, m.stride))
		}
		m.phases, m.fixed = 4, 8+len(m.surfaces)
		m.childFits()
	}
	m.rc = newResidualCoder(c, m.phases)
	m.pred = make([]float64, m.fixed+len(m.fits[0]))
	m.sq = make([][]float32, len(m.pred))
	for k := range m.sq {
		m.sq[k] = make([]float32, size)
	}
	return m
}

func (m *model) offsets(list [][2]int) []int {
	out := make([]int, len(list))
	for i, d := range list {
		out[i] = d[1]*m.stride + d[0]
	}
	return out
}

// standaloneFits takes each feature against the sample to the left.
func (m *model) standaloneFits() {
	for i, spec := range []struct{ n, lambda float64 }{{7, 0.99}, {13, 0.998}, {24, 0.9995}} {
		id := string(rune('1' + i))
		if n := int(knob("r"+id+"n", spec.n)); n > 1 {
			fit := newOLS(n-1, knob("r"+id+"l", spec.lambda), knob("rnu", 0.0005), 1)
			m.fits[0] = append(m.fits[0], fitted{fit: fit, child: m.offsets(neighbours[1:n])})
		}
	}
	plane := []float64{1, -1}
	for i, spec := range []struct{ n, radius, rows, row, gain float64 }{{0, 8, 0.85, 0.85, 1}, {0, 12, 0.9, 0.9, 1}} {
		id := string(rune('1' + i))
		if n := int(knob("w"+id+"n", spec.n)); n > 2 {
			fit := newWindowFit(n-1, m.w, int(knob("w"+id+"r", spec.radius)), knob("w"+id+"v", spec.rows), knob("w"+id+"h", spec.row), knob("w"+id+"g", spec.gain), knob("wnu", 0.0005), plane)
			m.fits[0] = append(m.fits[0], fitted{fit: fit, child: m.offsets(neighbours[1:n])})
		}
	}
}

// childFits gives each place in a block its own predictors, and takes each feature
// against the parent sample over the block.
func (m *model) childFits() {
	for i, spec := range []struct{ child, parent, lambda float64 }{{6, 4, 0.998}, {12, 8, 0.9995}} {
		id := string(rune('1' + i))
		nc, np := int(knob("c"+id+"c", spec.child)), int(knob("c"+id+"p", spec.parent))
		if nc+np == 0 {
			continue
		}
		parent := make([]int, np)
		for j, d := range parentNeighbours[:np] {
			parent[j] = d[1]*m.up.stride + d[0]
		}
		var extra [][]float64
		if i == 1 && knob("cx", 1) > 0 {
			extra = m.surfaces
		}
		for phase := range m.fits {
			fit := newOLS(nc+np+len(extra), knob("c"+id+"l", spec.lambda), knob("cnu", 0.0005), 1)
			m.fits[phase] = append(m.fits[phase], fitted{fit: fit, child: m.offsets(neighbours[:nc]), parent: parent, extra: extra})
		}
	}
	if n := int(knob("sn", 8)); n > 0 {
		for _, surface := range m.surfaces {
			for phase := range m.fits {
				fit := newOLS(n, knob("sl", 0.999), knob("snu", 0.0005), 1)
				m.fits[phase] = append(m.fits[phase], fitted{fit: fit, child: m.offsets(neighbours[:n]), surface: surface})
			}
		}
	}
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
	up := m.up
	step := 1
	if up != nil {
		step = 2
	}
	for y := 0; y < m.h; y++ {
		row := (y+margin)*s + margin
		for k := 1; k <= margin; k++ {
			img[row-k] = img[row-s]
		}
		for _, f := range m.fits[0] {
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
			phase, pi := 0, 0
			ref := w
			if up != nil {
				phase = (y&1)<<1 | x&1
				pi = (y/2+parentMargin)*up.stride + x/2 + parentMargin
				ref = up.mean[pi]
				u := float64(ref)
				hx := float64(up.mean[pi+1]-up.mean[pi-1]) / 8
				hy := float64(up.mean[pi+up.stride]-up.mean[pi-up.stride]) / 8
				switch phase {
				case 0:
					p[6] = u - hx - hy
					p[7] = u
				case 1:
					p[6] = u + hx - hy
					p[7] = 2*(u-hy) - float64(w)
				case 2:
					p[6] = u - hx + hy
					p[7] = float64(4*ref-n-ne)/2 - hx
				default:
					p[6] = u + hx + hy
					p[7] = float64(4*ref - nw - n - w)
				}
			}
			for k, surface := range m.surfaces {
				p[8+k] = surface[i]
			}
			fits := m.fits[phase]
			for k, f := range fits {
				feat := f.fit.features()
				if f.surface != nil {
					for j, off := range f.child {
						feat[j] = float64(img[i+off]) - f.surface[i+off]
					}
					p[m.fixed+k] = f.surface[i] + f.fit.predict()
					continue
				}
				for j, off := range f.child {
					feat[j] = float64(img[i+off] - ref)
				}
				at := len(f.child)
				for j, off := range f.parent {
					feat[at+j] = float64(up.mean[pi+off] - ref)
				}
				at += len(f.parent)
				for j, surface := range f.extra {
					feat[at+j] = surface[i] - float64(ref)
				}
				p[m.fixed+k] = float64(ref) + f.fit.predict()
			}
			// A neighbour samples away sits at the same place in its block.
			near, far := i-step, i-step*s
			num, den := 0.0, 0.0
			for k, sq := range m.sq {
				e := float64(sq[near]) + float64(sq[far]) + float64(0.5*(float64(sq[far-step])+float64(sq[far+step]))) + 0.02
				if up == nil {
					e += float64(0.5 * (float64(sq[i-2]) + float64(sq[i-2*s])))
				}
				wk := 1 / float64(e*e)
				num += float64(wk * p[k])
				den += wk
			}
			mu := num / den
			center := int32(math.Floor(mu + 0.5))
			lo, hi := 0, 0
			bounded := phase == 3
			if bounded {
				block := (y/2)*up.blocks + x/2
				rest := nw + n + w
				center = min(max(center, up.lo[block]-rest), up.hi[block]-rest)
				lo, hi = int(up.lo[block]-rest-center), int(up.hi[block]-rest-center)
			}
			frac := mu - float64(center)

			ew, en := errs[i-1], errs[i-s]
			scale8 := 3*absInt(errs[near]) + 3*absInt(errs[far]) + absInt(errs[far-step]) + absInt(errs[far+step])
			if up == nil {
				scale8 = 2*absInt(ew) + 2*absInt(en) + absInt(errs[i-s-1]) + absInt(errs[i-s+1]) + absInt(errs[i-2]) + absInt(errs[i-2*s])
			}
			kq := min(quarterLog(scale8), scaleLevels-1)
			kc := kq / 3
			grad := absInt(w-nw) + absInt(n-nw) + absInt(n-ne) + absInt(w-ww)
			shape := b2i(w == n) | b2i(n == nw)<<1 | b2i(n == ne)<<2 | b2i(w == ww)<<3 | int(center&1)<<4
			if bounded {
				shape = min(-lo, 10)*11 + min(hi, 10)
			}
			rc := m.rc
			rc.riceBits = uint(bits.Len32(scale8 >> 4))
			np := m.phases
			rc.mixSet = kc*np + phase
			rc.ctx[0] = kq*np + phase
			rc.ctx[1] = (kc*16+min(max(int(float64((frac+0.5)*16)), 0), 15))*np + phase
			rc.ctx[2] = (kc*49+relative(float64(ew), scale8)*7+relative(float64(en), scale8))*np + phase
			rc.ctx[3] = (kc*49+relative(p[2]-mu, scale8)*7+relative(p[len(p)-1]-mu, scale8))*np + phase
			rc.ctx[4] = (kc*32)*np + phase
			if bounded {
				rc.ctx[4] = shape*np + phase
			} else {
				rc.ctx[4] += shape * np
			}
			rc.ctx[5] = (kc*32+min(quarterLog(grad)/2, 31))*np + phase

			e := 0
			if src != nil {
				e = int(src[y*m.w+x] - center)
			}
			if bounded {
				e = rc.codeBounded(e, lo, hi)
			} else {
				e = rc.code(e)
			}
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
			for _, f := range fits {
				if f.surface != nil {
					f.fit.update(fv - f.surface[i])
				} else {
					f.fit.update(float64(v - ref))
				}
			}
		}
		for k := 0; k < margin; k++ {
			img[row+m.w+k] = img[row+m.w-1]
		}
	}
}
