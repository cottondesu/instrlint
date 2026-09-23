#!/usr/bin/env bash
set -euo pipefail

version=${1:?usage: package-release.sh VERSION GOOS GOARCH OUTPUT_DIR}
target_os=${2:?usage: package-release.sh VERSION GOOS GOARCH OUTPUT_DIR}
target_arch=${3:?usage: package-release.sh VERSION GOOS GOARCH OUTPUT_DIR}
output_dir=${4:?usage: package-release.sh VERSION GOOS GOARCH OUTPUT_DIR}

if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release tag: $version" >&2
  exit 1
fi

case "$target_os/$target_arch" in
  darwin/amd64|darwin/arm64|linux/amd64|linux/arm64|windows/amd64|windows/arm64) ;;
  *) echo "unsupported release target: $target_os/$target_arch" >&2; exit 1 ;;
esac

stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
mkdir -p "$output_dir"

binary=instrlint
if [[ $target_os == windows ]]; then
  binary=instrlint.exe
fi

CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
  go build -trimpath -o "$stage/$binary" ./cmd/instrlint
cp LICENSE "$stage/LICENSE"

archive="instrlint_${version}_${target_os}_${target_arch}"
if [[ $target_os == windows ]]; then
  zip -q -j "$output_dir/$archive.zip" "$stage/$binary" "$stage/LICENSE"
else
  tar -czf "$output_dir/$archive.tar.gz" -C "$stage" "$binary" LICENSE
fi
