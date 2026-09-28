#!/usr/bin/env bash
# Windows 빌드 후 dist/에 폴더·zip 생성 (--no-zip)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
name="운전영상편집기"
out="$root/dist/$name"
wails="$(command -v wails || echo "$HOME/go/bin/wails")"

if [ ! -f "$root/tools/windows/yt-dlp.exe" ]; then
  "$root/scripts/fetch-tools.sh" windows
fi

cd "$root"
"$wails" build -platform windows/amd64 -clean -trimpath -ldflags "-s -w"

rm -rf "$out"
mkdir -p "$out/bin"
cp "build/bin/simpleEdit.exe" "$out/$name.exe"
cp tools/windows/yt-dlp.exe tools/windows/ffmpeg.exe tools/windows/ffprobe.exe tools/windows/deno.exe "$out/bin/"

if [ "${1:-}" != "--no-zip" ]; then
  rm -f "$root/dist/$name.zip"
  (cd "$root/dist" && zip -qr -9 "$name.zip" "$name")
  ls -lh "$root/dist/$name.zip"
fi
echo "완료: $out"
