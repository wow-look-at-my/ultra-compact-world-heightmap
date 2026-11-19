// tune.go holds the flag that overrides a model parameter for an experiment.

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
)

type tuneFlag struct{}

func (tuneFlag) String() string { return "" }

func (tuneFlag) Type() string { return "name=value" }

func (tuneFlag) Set(s string) error {
	for _, pair := range strings.Split(s, ",") {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("%q is not name=value", pair)
		}
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		codec.Tune[name] = v
	}
	return nil
}
