// rangecoder.go holds the binary arithmetic coder every decision goes through.

package codec

const (
	probBits = 16
	probOne  = 1 << probBits
	rcTop    = 1 << 24
)

// coder writes or reads one binary decision at a time.
type coder struct {
	decoding bool

	low       uint64
	rng       uint32
	cache     byte
	cacheSize int64
	started   bool
	out       []byte

	code uint32
	in   []byte
	pos  int
}

func newEncoder() *coder {
	return &coder{rng: 0xFFFFFFFF, cacheSize: 1}
}

func newDecoder(in []byte) *coder {
	c := &coder{decoding: true, rng: 0xFFFFFFFF, in: in}
	for i := 0; i < 4; i++ {
		c.code = c.code<<8 | uint32(c.next())
	}
	return c
}

// next reads past the end as zeros: the encoder drops the zero bytes that end a stream.
func (c *coder) next() byte {
	if c.pos < len(c.in) {
		b := c.in[c.pos]
		c.pos++
		return b
	}
	return 0
}

func (c *coder) shiftLow() {
	if uint32(c.low) < 0xFF000000 || c.low>>32 != 0 {
		carry := byte(c.low >> 32)
		b := c.cache
		for ; c.cacheSize > 0; c.cacheSize-- {
			// The first byte is always zero, so it is not written.
			if c.started {
				c.out = append(c.out, b+carry)
			}
			c.started = true
			b = 0xFF
		}
		c.cache = byte(c.low >> 24)
	}
	c.cacheSize++
	c.low = (c.low & 0x00FFFFFF) << 8
}

// bit codes one decision. The encoder passes the bit. The decoder ignores the argument
// and returns what it read.
func (c *coder) bit(bit int, p1 uint32) int {
	bound := uint32(uint64(c.rng) * uint64(p1) >> probBits)
	if c.decoding {
		if c.code < bound {
			c.rng = bound
			bit = 1
		} else {
			c.code -= bound
			c.rng -= bound
			bit = 0
		}
		for c.rng < rcTop {
			c.code = c.code<<8 | uint32(c.next())
			c.rng <<= 8
		}
		return bit
	}
	if bit != 0 {
		c.rng = bound
	} else {
		c.low += uint64(bound)
		c.rng -= bound
	}
	for c.rng < rcTop {
		c.shiftLow()
		c.rng <<= 8
	}
	return bit
}

// finish ends the stream on the value in the last interval that has the most zero bytes
// at its end, and drops those bytes.
func (c *coder) finish() []byte {
	for k := 32; k >= 0; k -= 8 {
		mask := uint64(1)<<k - 1
		v := (c.low + mask) &^ mask
		if v < c.low+uint64(c.rng) {
			c.low = v
			break
		}
	}
	for i := 0; i < 5; i++ {
		c.shiftLow()
	}
	for len(c.out) > 0 && c.out[len(c.out)-1] == 0 {
		c.out = c.out[:len(c.out)-1]
	}
	return c.out
}
