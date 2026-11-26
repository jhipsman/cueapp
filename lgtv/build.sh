#!/usr/bin/env bash
# Builds Cue's LG TV app (lgtv/) into an .ipk to install on the TV:
#   ./lgtv/build.sh            -> lgtv/dist/app.cue.tv_<version>_all.ipk
# Needs Node (npx fetches LG's webOS tools the first time).
set -euo pipefail
cd "$(dirname "$0")"
version="$(tr -d ' \n' < ../VERSION 2>/dev/null || echo 1.0.0)"
version="${version#v}"
# webOS versions are three numbers.
version="$(echo "$version" | grep -oE '^[0-9]+\.[0-9]+\.[0-9]+' || echo 1.0.0)"
rm -rf dist build && mkdir -p dist build
cp index.html icon.png largeIcon.png splash.png build/
sed "s/\"version\": \"[^\"]*\"/\"version\": \"$version\"/" appinfo.json > build/appinfo.json
npx --yes @webos-tools/cli@3 ares-package build -o dist
ls dist/*.ipk
