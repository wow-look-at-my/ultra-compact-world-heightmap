#!/bin/bash
# source-check.sh <directory> runs the reprojection step of the Mapterhorn pipeline on one tile of
# the public a later copernicus m data. Codes each served a later zoom tile over it against the
# result. It needs curl, gdalwarp, gdal_translate, awk and uch.
set -eu
out="${1:?usage: source-check.sh <directory>}"
uch="${UCH:-uch}"
mkdir -p "$out"
name=Copernicus_DSM_COG_10_N27_00_E086_00_DEM
source="$out/$name.tif"
[ -s "$source" ] || curl -fsS -o "$source" "https://copernicus-dem-30m.s3.amazonaws.com/$name/$name.tif"
total=0
served=0
for y in 1716 1717 1718 1719; do
	for x in 3032 3033 3034 3035 3036; do
		tile="$out/12-$x-$y"
		[ -s "$tile.webp" ] || curl -fsS -o "$tile.webp" "https://tiles.mapterhorn.com/12/$x/$y.webp"
		read -r left bottom right top res <<< "$(awk -v x="$x" -v y="$y" 'BEGIN {
			half = 20037508.342789244; span = 2 * half / 4096
			printf "%.9f %.9f %.9f %.9f %.12f", -half + x * span, half - (y + 1) * span, -half + (x + 1) * span, half - y * span, span / 512
		}')"
		gdalwarp -q -overwrite -t_srs EPSG:3857 -tr "$res" "$res" -te "$left" "$bottom" "$right" "$top" -r cubicspline -dstnodata -9999 "$source" "$tile.tif"
		gdal_translate -q -of ENVI -ot Float32 "$tile.tif" "$tile.f32"
		"$uch" round --zoom 12 "$tile.f32" "$tile.warp.png"
		"$uch" encode --reference "$tile.warp.png" "$tile.webp" "$tile.uch" > /dev/null
		"$uch" decode --reference "$tile.warp.png" "$tile.uch" "$tile.back.png"
		"$uch" encode "$tile.back.png" "$tile.alone.uch" > /dev/null
		"$uch" encode "$tile.webp" "$tile.check.uch" > /dev/null
		cmp -s "$tile.alone.uch" "$tile.check.uch" || { echo "12/$x/$y did not decode to the served samples" >&2; exit 1; }
		size="$(wc -c < "$tile.uch")"
		echo "12/$x/$y  webp $(wc -c < "$tile.webp")  alone $(wc -c < "$tile.check.uch")  against the warp $size"
		total=$((total + size))
		served=$((served + $(wc -c < "$tile.webp")))
	done
done
echo "20 tiles  webp $served  against the warp $total"
