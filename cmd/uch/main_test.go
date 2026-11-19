package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// A flag keeps its value from one run to the next, so each run starts from the defaults.
	encodeParent, decodeParent, benchPerTile, benchMode = parentFlags{quadrant: -1}, parentFlags{quadrant: -1}, false, "both"
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

func TestPhasePrintsACostTable(t *testing.T) {
	t.Serial()
	out, err := run(t, "phase", "../../testdata/tiles/12-2144-1434.webp")
	require.NoError(t, err)
	assert.Contains(t, out, "12-2144-1434:")
	assert.Contains(t, out, "bits per sample")
}

func TestPhaseFailsOnAMissingFile(t *testing.T) {
	t.Serial()
	_, err := run(t, "phase", "1-2-3.webp")
	require.Error(t, err)
	_, err = run(t, "phase", "tile.webp")
	require.Error(t, err)
}
