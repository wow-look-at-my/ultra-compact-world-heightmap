package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

func TestStatsDescribesATile(t *testing.T) {
	out, err := run(t, "stats", "../../testdata/tiles/12-2144-1434.webp", "../../testdata/tiles/2-0-2.webp")
	require.NoError(t, err)
	assert.Contains(t, out, "12-2144-1434.webp")
	assert.Contains(t, out, "2-0-2.webp")
}

func TestStatsFailsOnAMissingFile(t *testing.T) {
	_, err := run(t, "stats", "no-such-tile.webp")
	require.Error(t, err)
}

func TestTrailingZeroBits(t *testing.T) {
	assert.Equal(t, 3, trailingZeroBits([]int32{8, 16, 24}))
	assert.Equal(t, 0, trailingZeroBits([]int32{8, 1}))
	assert.Equal(t, 24, trailingZeroBits([]int32{0}))
}
