package pyramid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

func TestQuantum(t *testing.T) {
	assert.Equal(t, int32(256), Quantum(0))
	assert.Equal(t, int32(256), Quantum(11))
	assert.Equal(t, int32(128), Quantum(12))
	assert.Equal(t, int32(4), Quantum(17))
	assert.Equal(t, int32(1), Quantum(19))
	assert.Equal(t, int32(1), Quantum(22))
}

func TestRoundHalfEven(t *testing.T) {
	for _, c := range [][3]int64{{0, 4, 0}, {1, 4, 0}, {2, 4, 0}, {3, 4, 1}, {6, 4, 2}, {10, 4, 2}, {-1, 4, 0}, {-2, 4, 0}, {-3, 4, -1}, {-6, 4, -2}, {-10, 4, -2}, {7, 2, 4}, {5, 2, 2}} {
		assert.Equal(t, c[2], RoundHalfEven(c[0], c[1]), "%d / %d", c[0], c[1])
	}
}

func flat(v int32) *terrarium.Tile {
	t := &terrarium.Tile{Width: 4, Height: 4, Samples: make([]int32, 16)}
	for i := range t.Samples {
		t.Samples[i] = v
	}
	return t
}

func TestParentAveragesEachBlockAndRoundsToTheQuantumOfItsZoom(t *testing.T) {
	a := flat(seaLevel + 256)
	a.Samples[0] = seaLevel + 3*256
	p, err := Parent([4]*terrarium.Tile{a, nil, flat(seaLevel - 512), flat(seaLevel + 128)}, 11)
	require.NoError(t, err)
	assert.Equal(t, int32(seaLevel+2*256), p.Samples[0])
	assert.Equal(t, int32(seaLevel+256), p.Samples[1])
	assert.Equal(t, int32(seaLevel), p.Samples[2])
	assert.Equal(t, int32(seaLevel-512), p.Samples[2*4])
	// Half a metre rounds to the even zero at a zoom that keeps whole metres.
	assert.Equal(t, int32(seaLevel), p.Samples[2*4+2])
	p, err = Parent([4]*terrarium.Tile{nil, nil, nil, flat(seaLevel + 128)}, 12)
	require.NoError(t, err)
	assert.Equal(t, int32(seaLevel+128), p.Samples[2*4+2])
}

func TestParentRejectsChildrenItCannotJoin(t *testing.T) {
	_, err := Parent([4]*terrarium.Tile{}, 3)
	require.Error(t, err)
	_, err = Parent([4]*terrarium.Tile{flat(0), {Width: 2, Height: 2, Samples: make([]int32, 4)}}, 3)
	require.Error(t, err)
}
