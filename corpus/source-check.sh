#!/bin/bash
# source-check.sh <directory> [x/y ...] codes served tiles of the deepest
# global zoom against the reprojection step of the Mapterhorn pipeline, run
# here.
set -eu
out="${1:?usage: source-check.sh <directory> [x/y ...]}"
shift
uch="${UCH:-uch}"
# SOURCE names a raster to warp in place of the Copernicus data. ZOOM is its deepest zoom.
zoom="${ZOOM:-12}"
mkdir -p "$out"

# bounds <x> <y> prints the Web Mercator edges of a tile, and the size of one sample.
bounds() {
	awk -v x="$1" -v y="$2" -v zoom="$zoom" 'BEGIN {
		half = 20037508.342789244; span = 2 * half / 2 ^ zoom
		printf "%.9f %.9f %.9f %.9f %.12f", -half + x * span, half - (y + 1) * span, -half + (x + 1) * span, half - y * span, span / 512
	}'
}

# cells <x> <y> prints the name of each Copernicus tile that a tile touches.
cells() {
	awk -v x="$1" -v y="$2" 'function lat(row) { n = pi * (1 - 2 * row / 4096); return atan2((exp(n) - exp(-n)) / 2, 1) * 180 / pi }
	function floor(v) { return v == int(v) || v > 0 ? int(v) : int(v) - 1 }
	BEGIN {
		pi = atan2(0, -1); edge = 0.003
		west = x / 4096 * 360 - 180; east = (x + 1) / 4096 * 360 - 180
		for (a = floor(lat(y + 1) - edge); a <= floor(lat(y) + edge); a++)
			for (o = floor(west - edge); o <= floor(east + edge); o++)
				printf "Copernicus_DSM_COG_10_%s%02d_00_%s%03d_00_DEM\n", a < 0 ? "S" : "N", a < 0 ? -a : a, o < 0 ? "W" : "E", o < 0 ? -o : o
	}'
}

# warp <grid> <x> <y> <out.tif> runs the gdalwarp command of the pipeline for one tile.
warp() {
	read -r left bottom right top res <<< "$(bounds "$2" "$3")"
	gdalwarp -q -overwrite -ot Float32 -t_srs EPSG:3857 -tr "$res" "$res" -te "$left" "$bottom" "$right" "$top" -r cubicspline -dstnodata -9999 "$1" "$4"
}

# check <x> <y> codes one served tile against the warp of the source, decodes it and compares.
check() {
	tile="$out/$zoom-$1-$2"
	[ -s "$tile.webp" ] || curl -fsS -o "$tile.webp" "https://tiles.mapterhorn.com/$zoom/$1/$2.webp"
	sources=()
	if [ -n "${SOURCE:-}" ]; then
		sources=("$SOURCE")
	else
		[ "$zoom" = 12 ] || { echo "the Copernicus data is deepest at zoom 12: name a SOURCE for zoom $zoom" >&2; exit 1; }
		for name in $(cells "$1" "$2"); do
			[ -s "$out/$name.tif" ] || curl -fsS -o "$out/$name.tif" "https://copernicus-dem-30m.s3.amazonaws.com/$name/$name.tif" || { rm -f "$out/$name.tif"; continue; }
			sources+=("$out/$name.tif")
		done
	fi
	[ "${#sources[@]}" -gt 0 ] || { echo "$zoom/$1/$2: no Copernicus tile is under it" >&2; exit 1; }
	gdalbuildvrt -q -overwrite "$tile.vrt" "${sources[@]}"
	warp "$tile.vrt" "$1" "$2" "$tile.tif"
	gdal_translate -q -of ENVI -ot Float32 "$tile.tif" "$tile.f32"
	"$uch" encode --reference "$tile.f32" "$tile.webp" "$tile.uch" > /dev/null
	"$uch" decode --reference "$tile.f32" "$tile.uch" "$tile.back.png"
	"$uch" encode "$tile.back.png" "$tile.alone.uch" > /dev/null
	"$uch" encode "$tile.webp" "$tile.check.uch" > /dev/null
	cmp -s "$tile.alone.uch" "$tile.check.uch" || { echo "$zoom/$1/$2 did not decode to the served samples" >&2; exit 1; }
	size="$(wc -c < "$tile.uch")"
	echo "$zoom/$1/$2  ${#sources[@]} source tiles  webp $(wc -c < "$tile.webp")  alone $(wc -c < "$tile.check.uch")  against the warp $size"
	total=$((total + size))
	served=$((served + $(wc -c < "$tile.webp")))
}

total=0
served=0
if [ "$#" -gt 0 ]; then
	for address in "$@"; do
		check "${address%/*}" "${address#*/}"
	done
	echo "$# tiles  webp $served  against the warp $total"
	exit 0
fi

for y in 1716 1717 1718 1719; do
	for x in 3032 3033 3034 3035 3036; do
		check "$x" "$y"
	done
done
echo "20 tiles  webp $served  against the warp $total"
source="$out/Copernicus_DSM_COG_10_N27_00_E086_00_DEM.tif"

# Can the source grid stand in for the tiles? Round it to the step of a zoom, compress
# windows of it, and code the served tiles against the warp of the rounded grid.
for zoom in 11 12 13 14; do
	per=$((1 << (zoom - 11)))
	grid="$out/grid-$zoom.tif"
	gdal_translate -q -ot Int32 -scale -10000 10000 $((-10000 * per)) $((10000 * per)) "$source" "$grid"
	bytes=0
	for window in "1800 100" "2400 300" "1900 700" "2900 600"; do
		read -r wx wy <<< "$window"
		gdal_translate -q -srcwin "$wx" "$wy" 512 512 -of ENVI -ot Float32 -scale $((-10000 * per)) $((10000 * per)) -10000 10000 "$grid" "$out/window.f32"
		"$uch" round --zoom "$zoom" "$out/window.f32" "$out/window.png"
		"$uch" encode "$out/window.png" "$out/window.uch" > /dev/null
		bytes=$((bytes + $(wc -c < "$out/window.uch")))
	done
	total=0
	for y in 1716 1717 1718 1719; do
		for x in 3032 3033 3034 3035 3036; do
			warp "$grid" "$x" "$y" "$out/rounded.tif"
			gdal_translate -q -of ENVI -ot Float32 -scale $((-10000 * per)) $((10000 * per)) -10000 10000 "$out/rounded.tif" "$out/rounded.f32"
			"$uch" encode --reference "$out/rounded.f32" "$out/12-$x-$y.webp" "$out/rounded.uch" > /dev/null
			total=$((total + $(wc -c < "$out/rounded.uch")))
		done
	done
	echo "source grid with $per steps in a metre: 4 windows of 512 by 512 take $bytes bytes, and the 20 tiles against its warp take $total"
done
