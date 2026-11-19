// Package geo places the samples of a web mercator tile on the globe.
package geo

import "math"

const arcsecPerTurn = 360 * 3600

// LonArcsec gives the longitude of the centre of a sample column, in arc seconds east.
func LonArcsec(zoom, tileX, column, side int) float64 {
	n := float64(tileX*side+column) + 0.5
	return n*arcsecPerTurn/float64(side<<zoom) - arcsecPerTurn/2
}

// LatArcsec gives the latitude of the centre of a sample row, in arc seconds north.
func LatArcsec(zoom, tileY, row, side int) float64 {
	n := float64(tileY*side+row) + 0.5
	y := math.Pi * (1 - 2*n/float64(side<<zoom))
	return math.Atan(math.Sinh(y)) * (arcsecPerTurn / (2 * math.Pi))
}

// GLO30LonStep gives the longitude spacing of the Copernicus GLO-30 grid at a latitude,
// in arc seconds. The latitude spacing is one arc second everywhere.
func GLO30LonStep(latArcsec float64) float64 {
	lat := math.Abs(latArcsec) / 3600
	switch {
	case lat < 50:
		return 1
	case lat < 60:
		return 1.5
	case lat < 70:
		return 2
	case lat < 80:
		return 3
	case lat < 85:
		return 5
	}
	return 10
}

// Phase gives the place of a coordinate inside its grid cell, from zero up to one.
func Phase(coordinate, step float64) float64 {
	p := coordinate / step
	return p - math.Floor(p)
}
