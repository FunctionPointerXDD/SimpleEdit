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
if [ ! -f "$root/tools/windows/onnxruntime.dll" ] || [ ! -f "$root/tools/models/face_yunet_2026may.onnx" ]; then
  "$root/scripts/fetch-tools.sh" ml
fi
if [ ! -f "$root/tools/windows/vcruntime140_1.dll" ]; then
  "$root/scripts/fetch-tools.sh" vcrt
fi
command -v x86_64-w64-mingw32-gcc >/dev/null || {
  echo "mingw-w64가 필요합니다: sudo apt install gcc-mingw-w64-x86-64" >&2
  exit 1
}

cd "$root"
# onnxruntime_go가 cgo라 mingw로 크로스 빌드
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
  "$wails" build -platform windows/amd64 -clean -trimpath -ldflags "-s -w"
# wails가 바인딩 파일 권한을 755로 바꿔 저장소 기준(644)으로 되돌린다
find frontend/wailsjs -type f -exec chmod 644 {} +

rm -rf "$out"
mkdir -p "$out/bin/models"
cp "build/bin/simpleEdit.exe" "$out/$name.exe"
cp tools/windows/yt-dlp.exe tools/windows/ffmpeg.exe tools/windows/ffprobe.exe tools/windows/deno.exe \
  tools/windows/onnxruntime.dll tools/windows/msvcp140.dll tools/windows/msvcp140_1.dll \
  tools/windows/vcruntime140.dll tools/windows/vcruntime140_1.dll "$out/bin/"
cp tools/models/face_yunet_2026may.onnx "$out/bin/models/"

if [ "${1:-}" != "--no-zip" ]; then
  rm -f "$root/dist/$name.zip"
  # zip 명령은 한글 이름에 UTF-8 표시가 없어 Windows에서 깨짐
  (cd "$root/dist" && python3 -m zipfile -c "$name.zip" "$name")
  ls -lh "$root/dist/$name.zip"
fi
echo "완료: $out"
