#!/usr/bin/env bash
# 검사 모음: Go 테스트·vet·staticcheck, 주석 규칙, 모의 UI 테스트
set -euo pipefail

qa="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$qa/../.." && pwd)"
cache="$qa/.cache"
mkdir -p "$cache"

[ -d "$qa/node_modules" ] || (cd "$qa" && npm install --silent)
if [ -z "${CHROME:-}" ]; then
  CHROME="$(ls -d "$HOME"/.cache/ms-playwright/chromium-*/chrome-linux64/chrome 2>/dev/null | sort -V | tail -1)"
fi
[ -x "$CHROME" ] || { echo "Chromium 없음: npx playwright install chromium" >&2; exit 1; }
export CHROME

# 모의 테스트용 영상(가로·세로). Chromium은 H.264를 못 틀어 VP9 사용
for v in wide:1280x720 tall:540x960; do
  name="${v%%:*}"; size="${v#*:}"
  [ -f "$cache/$name.webm" ] || ffmpeg -v error -y -f lavfi -i "testsrc2=size=$size:rate=30:duration=20" \
    -c:v libvpx-vp9 -deadline realtime -cpu-used 8 -b:v 1M "$cache/$name.webm"
done

win=(env CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ GOOS=windows)
echo "== Go 테스트"
(cd "$root" && go test . ./internal/... -count=1 | grep -v "no test files")
echo "== Windows 대상 vet (cgo)"
(cd "$root" && "${win[@]}" go vet ./... && echo ok)
echo "== staticcheck (Windows 대상)"
[ -x "$cache/bin/staticcheck" ] || GOBIN="$cache/bin" GOTOOLCHAIN=go1.26.8 go install honnef.co/go/tools/cmd/staticcheck@latest
(cd "$root" && "${win[@]}" GOTOOLCHAIN=go1.26.8 "$cache/bin/staticcheck" ./... && echo ok)
echo "== Go 주석 규칙"
(cd "$root" && go run ./cmd/cmtcheck .)
echo "== JS 주석 규칙"
node "$qa/jscheck.mjs" "$root/frontend/src"
echo "== 프론트 빌드"
(cd "$root/frontend" && npm run build --silent >/dev/null)
echo "== 모의 UI 테스트"
fail=0
for v in wide tall; do
  node "$qa/ui-mock.mjs" "$v" > "$cache/ui-$v.txt" 2>&1 || fail=1
  grep -v '^PASS' "$cache/ui-$v.txt" || true
done
exit $fail
