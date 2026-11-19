// logistic.go holds the logistic tables, the adaptive probabilities and the mixer.

package codec

import "math"

const stretchMax = 2840

var (
	squashTable  [2*stretchMax + 1]uint16
	stretchTable [probOne]int16
	reciprocal   [1024]int64
)

func init() {
	for i := range squashTable {
		x := float64(i-stretchMax) / 256
		v := math.Round(probOne / (1 + math.Exp(-x)))
		squashTable[i] = uint16(min(max(v, 1), probOne-1))
	}
	x := 0
	for p := range stretchTable {
		for x < len(squashTable)-1 && int(squashTable[x]) < p {
			x++
		}
		stretchTable[p] = int16(x - stretchMax)
	}
	for n := range reciprocal {
		reciprocal[n] = 2 * 65536 / int64(2*n+3)
	}
}

func squash(x int) uint32 {
	return uint32(squashTable[min(max(x, -stretchMax), stretchMax)+stretchMax])
}

func stretch(p uint32) int {
	return int(stretchTable[p])
}

type counter uint32

const counterInit counter = 1 << 31

func (c counter) p() uint32 {
	return min(max(uint32(c>>16), 1), probOne-1)
}

func (c *counter) update(bit int, limit uint32) {
	s := uint32(*c)
	n := s & 1023
	p := int64(s >> 10)
	p += (int64(bit)<<22 - p) * reciprocal[n] >> 16
	p = min(max(p, 0), 1<<22-1)
	if n < limit {
		n++
	}
	*c = counter(uint32(p)<<10 | n)
}

// A mixer joins several stretched probabilities into one with a weight set that the
// context selects.
type mixer struct {
	n       int
	weights []int32
	inputs  []int32
	set     int
	pr      uint32
	shift   uint
}

func newMixer(inputs, sets int, shift uint) *mixer {
	m := &mixer{n: inputs, weights: make([]int32, inputs*sets), inputs: make([]int32, inputs), shift: shift}
	for i := range m.weights {
		m.weights[i] = int32(1<<16) / int32(inputs) * 3 / 2
	}
	return m
}

func (m *mixer) mix(set int) uint32 {
	m.set = set * m.n
	w := m.weights[m.set : m.set+m.n]
	dot := int64(0)
	for i, x := range m.inputs {
		dot += int64(w[i]) * int64(x)
	}
	m.pr = squash(int(dot >> 16))
	return m.pr
}

func (m *mixer) update(bit int) {
	err := int64(bit)<<probBits - int64(m.pr)
	w := m.weights[m.set : m.set+m.n]
	for i, x := range m.inputs {
		w[i] += int32((int64(x)*err + 1<<(m.shift-1)) >> m.shift)
	}
}
