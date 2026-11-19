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
	pred         []float64
	// sharpness is the power of the local error that a predictor's weight falls with.
	sharpness int
	// stacks hold a fit over the predictions for each place in a block and error class.
	stacks       []*ols
	stackClasses int
	flatCap      int
	// sea is sea level in the units of the tile, or negative when the quantum of the tile does not divide it.
	sea      int32
	seaTable []counter
	// share has a bit for each context table that the unbounded places of a block share.
	share int
	costs []float32
}

// newModel builds the model for a tile. With a parent grid, it predicts each sample
// from the parent as well. With a warm model, it starts from what that model learned.
func newModel(c *coder, w, h int, base, sea int32, up *parentGrid, warm *model) *model {
	m := &model{w: w, h: h, stride: w + 2*margin, up: up, phases: 1, fixed: 9, sea: sea}
	size := m.stride * (h + margin)
	m.img = make([]int32, size)
	for i := range m.img {
		m.img[i] = base
	}
	m.errs = make([]int32, size)
	if up == nil {
		m.standaloneFits()
	} else {
		m.phases, m.fixed = 4, 11
		m.childFits()
	}
	m.rc = newResidualCoder(c, m.phases)
	m.seaTable = make([]counter, 128*m.phases)
	for i := range m.seaTable {
		m.seaTable[i] = counterInit
	}
	m.pred = make([]float64, m.fixed+len(m.fits[0]))
	m.sharpness = int(knob("wexp", 1))
	m.share = int(knob("share", 63))
	m.flatCap = int(knob("flatcap", 2))
	m.stackClasses = int(knob("stackc", 3))
	if lambda := knob("stack", 0.9995); lambda > 0 {
		for n := 0; n < m.phases*m.stackClasses; n++ {
			m.stacks = append(m.stacks, newOLS(len(m.pred), lambda, knob("stacknu", 0.001), 1))
		}
	}
	m.sq = make([][]float32, len(m.pred))
	for k := range m.sq {
		m.sq[k] = make([]float32, size)
	}
	if warm != nil {
		m.adopt(warm, c)
	}
	return m
}

// adopt takes over what a model of the same shape learned. This is its probabilities,
// and the weights of every fit that does not depend on the width of the tile.
func (m *model) adopt(warm *model, c *coder) {
	m.rc = warm.rc
	m.rc.c = c
	m.seaTable = warm.seaTable
	for phase := range m.fits {
		for k := range m.fits[phase] {
			if learned, ok := warm.fits[phase][k].fit.(*ols); ok {
				m.fits[phase][k].fit = learned
			}
		}
	}
	m.stacks = warm.stacks
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
	for i, spec := range []struct{ n, radius, rows, row, gain float64 }{{13, 8, 0.85, 0.85, 1}, {24, 12, 0.9, 0.9, 1}} {
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
		for phase := range m.fits {
			fit := newOLS(nc+np, knob("c"+id+"l", spec.lambda), knob("cnu", 0.0005), 1)
			m.fits[phase] = append(m.fits[phase], fitted{fit: fit, child: m.offsets(neighbours[:nc]), parent: parent})
		}
	}
	if n := int(knob("cw", 12)); n > 0 {
		np := int(knob("cwp", 8))
		parent := make([]int, np)
		for j, d := range parentNeighbours[:np] {
			parent[j] = d[1]*m.up.stride + d[0]
		}
		for phase := range m.fits {
			fit := newWindowFit(n+np, m.w/2, int(knob("cwr", 8)), knob("cwv", 0.9), knob("cwh", 0.9), 1, knob("wnu", 0.0005), nil)
			m.fits[phase] = append(m.fits[phase], fitted{fit: fit, child: m.offsets(neighbours[:n]), parent: parent})
		}
	}
}

// energy sums the squared errors that a predictor made on the neighbours of sample i.
// step is how far away the nearest sample at the same place in its block lies.
func (m *model) energy(sq []float32, i, step int) float64 {
	far := i - step*m.stride
	e := float64(sq[i-step]) + float64(sq[far]) + float64(0.5*(float64(sq[far-step])+float64(sq[far+step]))) + 0.02
	if m.up == nil {
		e += float64(0.5 * (float64(sq[i-2]) + float64(sq[i-2*m.stride])))
	}
	return e
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
		if up == nil {
			for _, f := range m.fits[0] {
				f.fit.startRow(y == 0)
			}
		} else {
			// A row holds some of the places in a block.
			for _, phase := range [2]int{y & 1 * 2, y&1*2 + 1} {
				for _, f := range m.fits[phase] {
					f.fit.startRow(y < 2)
				}
			}
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
			// The median of the neighbours and the plane through them.
			p[6] = float64(max(min(w, n), min(max(w, n), w+n-nw)))
			p[7] = float64(2*(w+n)+ne-nw) / 4
			p[8] = float64(n + ne - img[i-2*s+1])
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
					p[9] = u - hx - hy
					p[10] = u
				case 1:
					p[9] = u + hx - hy
					p[10] = 2*(u-hy) - float64(w)
				case 2:
					p[9] = u - hx + hy
					p[10] = float64(4*ref-n-ne)/2 - hx
				default:
					p[9] = u + hx + hy
					p[10] = float64(4*ref - nw - n - w)
				}
			}
			fits := m.fits[phase]
			for k, f := range fits {
				feat := f.fit.features()
				for j, off := range f.child {
					feat[j] = float64(img[i+off] - ref)
				}
				at := len(f.child)
				for j, off := range f.parent {
					feat[at+j] = float64(up.mean[pi+off] - ref)
				}
				p[m.fixed+k] = float64(ref) + f.fit.predict()
			}
			// A neighbour samples away sits at the same place in its block.
			near, far := i-step, i-step*s
			num, den := 0.0, 0.0
			for k, sq := range m.sq {
				e := m.energy(sq, i, step)
				wk := 1 / e
				for n := 1; n < m.sharpness; n++ {
					wk /= e
				}
				num += float64(wk * p[k])
				den += wk
			}
			mu := num / den
			blend := mu
			ew, en := errs[i-1], errs[i-s]
			scale8 := 3*absInt(errs[near]) + 3*absInt(errs[far]) + absInt(errs[far-step]) + absInt(errs[far+step])
			if up == nil {
				scale8 = 2*absInt(ew) + 2*absInt(en) + absInt(errs[i-s-1]) + absInt(errs[i-s+1]) + absInt(errs[i-2]) + absInt(errs[i-2*s])
			}
			kq := min(quarterLog(scale8), scaleLevels-1)
			kc := kq / 3
			var stack *ols
			if m.stacks != nil {
				stack = m.stacks[phase*m.stackClasses+min(kq*m.stackClasses/24, m.stackClasses-1)]
				feat := stack.features()
				for k, v := range p {
					feat[k] = v - blend
				}
				mu += stack.predict()
			}
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
			grad := absInt(w-nw) + absInt(n-nw) + absInt(n-ne) + absInt(w-ww)
			shape := b2i(w == n) | b2i(n == nw)<<1 | b2i(n == ne)<<2 | b2i(w == ww)<<3 | int(center&1)<<4
			if bounded {
				shape = min(-lo, 10)*11 + min(hi, 10)
			}
			rc := m.rc
			rc.riceBits = uint(bits.Len32(scale8 >> 4))
			rc.unaryCap = unaryCap
			if scale8 == 0 {
				// A sample that breaks a flat run is as likely large as small.
				rc.unaryCap = m.flatCap
			}
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
			if phase < 3 {
				// The places that the parent does not bound can share a table.
				for t := range rc.ctx {
					if m.share>>t&1 != 0 {
						rc.ctx[t] -= phase
					}
				}
			}

			e := 0
			if src != nil {
				e = int(src[y*m.w+x] - center)
			}
			// Next to water, the first question is whether the sample is water too.
			shore := b2i(w == m.sea) | b2i(n == m.sea)<<1 | b2i(nw == m.sea)<<2 | b2i(ne == m.sea)<<3
			switch {
			case bounded:
				e = rc.codeBounded(e, lo, hi)
			case shore != 0:
				slot := (shore|b2i(ww == m.sea)<<4|b2i(nn == m.sea)<<5|b2i(up != nil && ref == m.sea)<<6)*np + phase
				if rc.plain(&m.seaTable[slot], b2i(e == int(m.sea-center))) == 1 {
					e = int(m.sea - center)
				} else {
					e = rc.code(e)
				}
			default:
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
			if stack != nil {
				stack.update(fv - blend)
			}
			for _, f := range fits {
				f.fit.update(float64(v - ref))
			}
		}
		for k := 0; k < margin; k++ {
			img[row+m.w+k] = img[row+m.w-1]
		}
	}
}
