package archive

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

const sea = 32768 * 256

// threeLevels makes rough tiles and the tiles that the pipeline builds above them.
func threeLevels(t *testing.T) map[Address]*terrarium.Tile {
	t.Helper()
	rng := rand.New(rand.NewSource(5))
	tiles := map[Address]*terrarium.Tile{}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			tile := &terrarium.Tile{Width: 32, Height: 32, Samples: make([]int32, 32*32)}
			for i := range tile.Samples {
				tile.Samples[i] = sea + int32((x*32+i%32)*(y*32+i/32)/50+rng.Intn(7))<<8
			}
			tiles[Address{6, 8 + x, 12 + y}] = tile
		}
	}
	for _, a := range []Address{{5, 4, 6}, {5, 5, 6}, {5, 4, 7}, {5, 5, 7}, {4, 2, 3}} {
		var children [4]*terrarium.Tile
		for k := range children {
			children[k] = tiles[a.child(k)]
		}
		parent, err := pyramid.Parent(children, a.Z)
		require.NoError(t, err)
		tiles[a] = parent
	}
	return tiles
}

func TestBothModesGiveEveryTileBack(t *testing.T) {
	tiles := threeLevels(t)
	sizes := map[Mode]int{}
	for _, mode := range []Mode{Chain, Leaves} {
		packed, err := Pack(tiles, mode)
		require.NoError(t, err)
		back, err := Unpack(packed)
		require.NoError(t, err)
		require.Len(t, back, len(tiles))
		for a, tile := range tiles {
			require.Equal(t, tile.Samples, back[a].Samples, "%s", a)
		}
		sizes[mode] = len(packed)
	}
	assert.Less(t, sizes[Leaves], sizes[Chain])
}

func TestATileThatItsChildrenDoNotRebuildIsStored(t *testing.T) {
	tiles := threeLevels(t)
	tiles[Address{5, 4, 6}].Samples[3] += 9 << 8
	packed, err := Pack(tiles, Leaves)
	require.NoError(t, err)
	back, err := Unpack(packed)
	require.NoError(t, err)
	for a, tile := range tiles {
		require.Equal(t, tile.Samples, back[a].Samples, "%s", a)
	}
}

func TestUnpackRejectsWhatIsNotAnArchive(t *testing.T) {
	packed, err := Pack(threeLevels(t), Chain)
	require.NoError(t, err)
	for _, data := range [][]byte{nil, []byte("UCHA"), []byte("NOPE\x01\x00\x00"), packed[:40], append([]byte(magic), 0, 1, 5, 1, 1, 200)} {
		_, err := Unpack(data)
		require.Error(t, err)
	}
	_, err = Pack(map[Address]*terrarium.Tile{{1, 0, 0}: {Width: 2, Height: 2, Samples: []int32{1}}}, Chain)
	require.Error(t, err)
}
