#!/usr/bin/env bash
# 외부 도구를 tools/<windows|linux>/에 내려받는다.
set -euo pipefail

target="${1:-windows}"
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/tools/$target"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$out"

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
*)
  echo "사용법: $0 windows|linux" >&2; exit 1 ;;
esac

ls -lh "$out"
