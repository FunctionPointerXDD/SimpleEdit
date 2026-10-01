#!/usr/bin/env bash
# 외부 도구를 tools/<windows|linux|ml|vcrt>/에 받는다.
set -euo pipefail

target="${1:-windows}"
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/tools/$target"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$out"

# 추론 런타임 버전 (onnxruntime_go 헤더 버전과 맞춤)
ORT_VERSION=1.29.0

fetch_checked() { # url, 저장 경로, sha256
  curl -fL -o "$2" "$1"
  echo "$3  $2" | sha256sum -c --quiet - || { rm -f "$2"; echo "해시 불일치: $1" >&2; exit 1; }
}

gh_asset_url() { # repo, grep 패턴 → 최신 릴리스 자산 URL 하나
  curl -fsSL "https://api.github.com/repos/$1/releases/latest" |
    grep -o '"browser_download_url": *"[^"]*"' | cut -d'"' -f4 | grep -E "$2" | head -1
}

case "$target" in
windows)
  echo "yt-dlp.exe 받는 중"
  curl -fL -o "$out/yt-dlp.exe" https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe

  echo "ffmpeg 받는 중"
  url="$(gh_asset_url BtbN/FFmpeg-Builds 'ffmpeg-n[0-9.]+-latest-win64-gpl-[0-9.]+\.zip$' | sort -V | tail -1)"
  [ -n "$url" ] || url=https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip
  curl -fL -o "$tmp/ffmpeg.zip" "$url"
  unzip -q -j -o "$tmp/ffmpeg.zip" '*/bin/ffmpeg.exe' '*/bin/ffprobe.exe' -d "$out"

  echo "deno.exe 받는 중"
  curl -fL -o "$tmp/deno.zip" https://github.com/denoland/deno/releases/latest/download/deno-x86_64-pc-windows-msvc.zip
  unzip -q -o "$tmp/deno.zip" deno.exe -d "$out"
  ;;
linux)
  echo "yt-dlp 받는 중"
  curl -fL -o "$out/yt-dlp" https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux
  chmod +x "$out/yt-dlp"

  echo "deno 받는 중"
  curl -fL -o "$tmp/deno.zip" https://github.com/denoland/deno/releases/latest/download/deno-x86_64-unknown-linux-gnu.zip
  unzip -q -o "$tmp/deno.zip" deno -d "$out"
  chmod +x "$out/deno"

  # ffmpeg/ffprobe는 시스템 것을 링크 (WSL 개발용)
  for t in ffmpeg ffprobe; do ln -sf "$(command -v $t)" "$out/$t"; done
  ;;
ml)
  echo "onnxruntime $ORT_VERSION 받는 중"
  ort=https://github.com/microsoft/onnxruntime/releases/download/v$ORT_VERSION
  mkdir -p "$root/tools/linux" "$root/tools/windows" "$root/tools/models"
  fetch_checked "$ort/onnxruntime-linux-x64-$ORT_VERSION.tgz" "$tmp/ort.tgz" \
    c3fddc4f139a045b0c4902c57410f0694f1c2fdf9b6939fbe38b1aeae7cd14ba
  tar xzf "$tmp/ort.tgz" -C "$tmp"
  cp "$tmp/onnxruntime-linux-x64-$ORT_VERSION/lib/libonnxruntime.so.$ORT_VERSION" "$root/tools/linux/"
  fetch_checked "$ort/onnxruntime-win-x64-$ORT_VERSION.zip" "$tmp/ort.zip" \
    c9b4b7086b529ad814f428c1bad028e20a25d7dc0699836775faace4ab5b78b2
  unzip -q -j -o "$tmp/ort.zip" "*/lib/onnxruntime.dll" -d "$root/tools/windows"

  echo "YuNet 얼굴 모델 받는 중 (OpenCV Zoo, MIT)"
  fetch_checked https://media.githubusercontent.com/media/opencv/opencv_zoo/main/models/face_detection_yunet/face_detection_yunet_2026may.onnx \
    "$root/tools/models/face_yunet_2026may.onnx" \
    ebafce4e3c118d6554634be5c27ab333b4c047a9a8c3faf1d7cf93101c22f0f0
  ln -sfn ../models "$root/tools/linux/models"
  out="$root/tools/models"
  ;;
vcrt)
  # onnxruntime.dll이 쓰는 VC++ 런타임. WSL에서 Windows 설치본을 복사
  sys32=/mnt/c/Windows/System32
  mkdir -p "$root/tools/windows"
  for f in msvcp140.dll msvcp140_1.dll vcruntime140.dll vcruntime140_1.dll; do
    [ -f "$sys32/$f" ] || { echo "$sys32/$f 없음: VC++ 재배포 패키지(x64)를 먼저 설치" >&2; exit 1; }
    cp "$sys32/$f" "$root/tools/windows/$f"
  done
  out="$root/tools/windows"
  ;;
*)
  echo "사용법: $0 windows|linux|ml|vcrt" >&2; exit 1 ;;
esac

ls -lh "$out"
