#!/usr/bin/env bash
# 배포 zip에 넣을 제3자 고지(THIRD_PARTY_NOTICES.txt)를 표준 출력으로 만든다.
# 실행 파일에 들어가는 Go 모듈은 go list(Windows 빌드 태그)로 찾아 모듈 캐시의 LICENSE를 붙이고,
# 함께 넣는 도구·모델은 docs/licenses/의 전문을 붙인다. FFmpeg 버전은 tools/windows/ffmpeg.exe에서 읽는다.
# 사용: scripts/notices.sh > THIRD_PARTY_NOTICES.txt
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
lic="$root/docs/licenses"
cd "$root"

section() { # 제목, 설명 줄...
  printf '\n%s\n%s\n' "================================================================================" "$1"
  shift
  printf '%s\n' "$@"
  printf '%s\n\n' "--------------------------------------------------------------------------------"
}

ffver="$( (chmod +x tools/windows/ffmpeg.exe && tools/windows/ffmpeg.exe -version) 2>/dev/null | head -1 | cut -d' ' -f3 || true)"

cat <<'EOF'
운전영상편집기 (simpleEdit) - 제3자 구성 요소 고지 / Third-party notices

이 앱 자체의 코드는 MIT 라이선스입니다(저장소의 LICENSE).
아래 구성 요소는 각자의 라이선스를 따르며, 전문을 그대로 싣습니다.
EOF

section "FFmpeg (bin/ffmpeg.exe, bin/ffprobe.exe) - GNU GPL v3" \
  "버전: ${ffver:-알 수 없음} (BtbN/FFmpeg-Builds win64-gpl, --enable-gpl --enable-version3)" \
  "앱은 FFmpeg를 별도 프로그램으로 실행만 하며, FFmpeg 코드를 앱에 넣거나 고치지 않았습니다." \
  "소스 코드: https://github.com/FFmpeg/FFmpeg (버전 이름의 g 뒤 해시가 커밋)" \
  "빌드 스크립트와 함께 쓴 라이브러리: https://github.com/BtbN/FFmpeg-Builds"
cat "$lic/gpl-3.0.txt"

section "ONNX Runtime 1.29.0 (bin/onnxruntime.dll) - MIT" "https://github.com/microsoft/onnxruntime"
cat "$lic/onnxruntime.txt"

section "yt-dlp (bin/yt-dlp.exe) - Unlicense" "https://github.com/yt-dlp/yt-dlp"
cat "$lic/yt-dlp.txt"

section "Deno (bin/deno.exe) - MIT" "https://github.com/denoland/deno"
cat "$lic/deno.txt"

section "Microsoft Visual C++ 재배포 가능 패키지 (bin/msvcp140*.dll, bin/vcruntime140*.dll)" \
  "Microsoft Visual C++ 2015-2022 Redistributable (x64)의 파일로, 그 사용 조건에 따라 앱과 함께 배포합니다." \
  "https://visualstudio.microsoft.com/license-terms/"

section "모델: YuNet (bin/models/face_yunet_2026may.onnx) - MIT" "https://github.com/opencv/opencv_zoo"
cat "$lic/yunet.txt"

section "모델: VitTrack (bin/models/object_tracking_vittrack_2023sep.onnx) - Apache-2.0" \
  "https://github.com/opencv/opencv_zoo"
cat "$lic/vittrack.txt"

section "모델: 번호판 (bin/models/plate_yolox_tiny.onnx) - YOLOX, Apache-2.0" \
  "YOLOX-Tiny를 번호판 1종류로 이어 학습하고 INT8로 바꿨습니다. 학습 데이터 출처는 bin/models/출처.txt" \
  "https://github.com/Megvii-BaseDetection/YOLOX"
cat "$lic/yolox.txt"

section "Go 표준 라이브러리 - BSD-3-Clause" "https://go.dev"
cat "$lic/go.txt"

modcache="$(go env GOMODCACHE)"
GOOS=windows CGO_ENABLED=1 go list -tags desktop,production -deps \
  -f '{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}' . | sort -u | grep -v '^simpleEdit' |
  while read -r m; do
    dir="$modcache/$(printf '%s' "$m" | sed 's/[A-Z]/!\L&/g')"
    f="$(ls "$dir"/LICENSE* "$dir"/COPYING* 2>/dev/null | head -1 || true)"
    [ -n "$f" ] || { echo "라이선스 파일 없음: $m" >&2; exit 1; }
    section "Go 모듈: $m" "https://pkg.go.dev/${m%@*}"
    cat "$f"
  done
