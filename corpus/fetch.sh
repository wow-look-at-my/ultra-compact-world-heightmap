#!/bin/bash
# fetch.sh <directory> downloads the tiles of every list beside it, one directory for each list.
# A tile that is already there is kept. A tile the server does not have is skipped.
set -u
out="${1:?usage: fetch.sh <directory>}"
here="$(cd "$(dirname "$0")" && pwd)"
for list in "$here"/*.txt; do
	set="$(basename "$list" .txt)"
	mkdir -p "$out/$set"
	while IFS=/ read -r z x y; do
		file="$out/$set/$z-$x-$y.webp"
		[ -s "$file" ] && continue
		code="$(curl -sS -m 60 -o "$file.part" -w '%{http_code}' "https://tiles.mapterhorn.com/$z/$x/$y.webp")"
		if [ "$code" = 200 ]; then
			mv "$file.part" "$file"
		else
			rm -f "$file.part"
			[ "$code" = 404 ] || echo "$set $z/$x/$y: HTTP $code" >&2
		fi
	done < "$list"
done
