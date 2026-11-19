// residual.go codes one prediction error as a short run of modelled binary decisions.

package codec

import (
	"math"
	"math/bits"
)

const (
	unaryCap     = 18
	decZero      = 0
	decSign      = 1
	decUnary     = 2
	decLow       = decUnary + unaryCap
	decisions    = decLow + 4
	modelInputs  = 6
	scaleLevels  = 64
	coarseLevels = 22
	counterLimit = 255
	escapeBits   = 24
)

// quarterLog maps a magnitude to steps for each doubling.
func quarterLog(v uint32) int {
	if v < 4 {
		return int(v)
	}
	l := bits.Len32(v) - 1
	return (l-1)<<2 | int(v>>(l-2))&3
}

// residualCoder holds the adaptive state for the error of one sample.
type residualCoder struct {
	c       *coder
	tables  [modelInputs][]counter
	sizes   [modelInputs]int
	mix     *mixer
	mixSets int
	phases  int

	refine      *refiner
	refineShare uint32
	escape      [2 * escapeBits]counter

	// The context of the sample in hand, set by the model before each call.
	ctx      [modelInputs]int
	mixSet   int
	riceBits uint

	// probing adds up the bits each sample takes, for the cost map.
	probing bool
	cost    float64
}

// newResidualCoder sizes every table for a context that also holds the place of the
// sample in its block, which takes one of phases values.
func newResidualCoder(c *coder, phases int) *residualCoder {
	r := &residualCoder{c: c}
	r.sizes = [modelInputs]int{scaleLevels, coarseLevels * 16, coarseLevels * 49, coarseLevels * 49, coarseLevels * 32, coarseLevels * 32}
	for i, n := range r.sizes {
		r.tables[i] = make([]counter, n*phases*decisions)
		for j := range r.tables[i] {
			r.tables[i][j] = counterInit
		}
	}
	for i := range r.escape {
		r.escape[i] = counterInit
	}
	r.mixSets = coarseLevels * phases
	r.phases = phases
	if share := uint32(knob("sse", 2)); share > 0 {
		r.refine, r.refineShare = newRefiner(decisions*r.sizes[0]*phases, 6), share
	}
	r.mix = newMixer(modelInputs+1, decisions*r.mixSets, uint(knob("mshift", 15)))
	return r
}

// codeBounded codes an error that is known to lie from lo to hi, where lo is at most
// zero and hi is at least zero. It spends nothing on a decision the bounds settle.
func (r *residualCoder) codeBounded(e, lo, hi int) int {
	if lo == 0 && hi == 0 {
		return 0
	}
	if r.decide(decZero, b2i(e == 0)) == 1 {
		return 0
	}
	negative := hi == 0
	if lo != 0 && hi != 0 {
		negative = r.decide(decSign, b2i(e < 0)) == 1
	}
	limit, a := hi, e
	if negative {
		limit, a = -lo, -e
	}
	j := 1
	for j < limit && r.decide(decUnary+j-1, b2i(a > j)) == 1 {
		j++
	}
	if negative {
		return -j
	}
	return j
}

// decide codes one decision of the given kind in the context of the sample in hand.
func (r *residualCoder) decide(kind, bit int) int {
	var slots [modelInputs]*counter
	for i := range slots {
		s := &r.tables[i][r.ctx[i]*decisions+kind]
		slots[i] = s
		r.mix.inputs[i] = int32(stretch(s.p()))
	}
	r.mix.inputs[modelInputs] = 256
	p := r.mix.mix(kind*r.mixSets + r.mixSet)
	if r.refine != nil {
		fine := r.refine.refine(p, kind*r.sizes[0]*r.phases+r.ctx[0])
		p = (p*(4-r.refineShare) + fine*r.refineShare) >> 2
	}
	bit = r.c.bit(bit, p)
	if r.refine != nil {
		r.refine.update(bit)
	}
	if r.probing {
		r.cost += bitCost(bit, p)
	}
	for _, s := range slots {
		s.update(bit, counterLimit)
	}
	r.mix.update(bit)
	return bit
}

func (r *residualCoder) plain(slot *counter, bit int) int {
	bit = r.c.bit(bit, slot.p())
	if r.probing {
		r.cost += bitCost(bit, slot.p())
	}
	slot.update(bit, counterLimit)
	return bit
}

// bitCost gives the bits that a decision takes at the probability it was coded with.
func bitCost(bit int, p1 uint32) float64 {
	p := float64(p1) / probOne
	if bit == 0 {
		p = 1 - p
	}
	return -math.Log2(p)
}

// code writes e when encoding, and returns the error either way.
func (r *residualCoder) code(e int) int {
	if r.decide(decZero, b2i(e == 0)) == 1 {
		return 0
	}
	negative := r.decide(decSign, b2i(e < 0)) == 1
	a := e
	if e < 0 {
		a = -e
	}
	a--
	k := r.riceBits
	hi := a >> k
	j := 0
	for j < unaryCap {
		if r.decide(decUnary+j, b2i(hi > j)) == 0 {
			break
		}
		j++
	}
	if j == unaryCap {
		j += r.codeEscape(hi - unaryCap)
	}
	a = j<<k | r.codeLow(a, k)
	a++
	if negative {
		return -a
	}
	return a
}

// codeLow codes the low k bits of a, from the top one down.
func (r *residualCoder) codeLow(a int, k uint) int {
	low := 0
	for i := uint(0); i < k; i++ {
		b := r.decide(decLow+int(min(i, 3)), a>>(k-1-i)&1)
		low = low<<1 | b
	}
	return low
}

// codeEscape codes a value past the unary cap as a bit count and then the bits.
func (r *residualCoder) codeEscape(v int) int {
	v++
	n := bits.Len(uint(v))
	length := 1
	for length < escapeBits {
		if r.plain(&r.escape[length], b2i(n > length)) == 0 {
			break
		}
		length++
	}
	out := 1
	for i := length - 2; i >= 0; i-- {
		out = out<<1 | r.plain(&r.escape[escapeBits+i], v>>i&1)
	}
	return out - 1
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
