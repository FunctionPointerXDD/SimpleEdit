# 운전영상편집기 (simpleEdit)

운전 강사가 강의용 도로 주행·사고 영상을 **유튜브 주소 입력 → 구간 자르기 → 가리기 → MP4 저장** 네 단계로 만드는 Windows 데스크톱 앱입니다.
영상 편집 경험이 없는 70대 사용자가 도움 없이 끝낼 수 있도록, 한 화면에 한 단계만 보여 주는 마법사 방식으로 만들었습니다.

- 대상 OS: Windows 10/11 64비트
- 기술: Go + Wails v2, 바닐라 JS, yt-dlp, FFmpeg, ONNX Runtime
- 자세한 요구사항과 설계 결정: [docs/PRD.md](docs/PRD.md), 진행 현황: [docs/status_board.html](docs/status_board.html)

## 받기와 실행

1. [Releases](https://github.com/FunctionPointerXDD/SimpleEdit/releases)에서 `운전영상편집기-<버전>.zip`을 받습니다.
2. 원하는 곳에 압축을 풀고 `운전영상편집기.exe`를 실행합니다. 설치는 필요 없습니다.
3. 바탕화면 바로가기가 필요하면 `운전영상편집기.exe`를 마우스 오른쪽 단추로 눌러 직접 만듭니다.

`bin\` 폴더(yt-dlp, FFmpeg, ONNX Runtime, VC++ 런타임, 모델)는 실행 파일 옆에 그대로 두어야 합니다.
WebView2 런타임이 없는 PC에서는 처음 실행할 때 Wails 기본 안내가 나옵니다.

## 사용 방법

| 단계 | 화면 | 하는 일 |
| --- | --- | --- |
| 1 | 영상 불러오기 | 유튜브 주소를 복사한 뒤 **붙여넣기**(최대 1080p로 받음), 또는 **내 컴퓨터 영상 열기**. 받은 영상은 7일 동안 다시 받지 않고 씁니다 |
| 2 | 구간 자르기 | 재생하면서 **여기서 시작**·**여기서 끝**으로 쓸 부분을 정합니다 |
| 3 | 가릴 곳 정하기 | 자동 가리기와 직접 가리기(아래). 가릴 것이 없으면 건너뜁니다 |
| 4 | 저장 | H.264 + AAC MP4로 저장합니다. 진행률·남은 시간·취소, 저장 위치 바꾸기, 저장 폴더 열기 |

### 3단계 가리기

- **사람 얼굴 가리기**: 자른 구간에서 얼굴을 찾아 프레임마다 따라가며 타원으로 흐리게 합니다. 머리카락까지 덮도록 조금 넓게 가립니다.
- **번호판 가리기**: 번호판 위치를 찾아 사각형으로 흐리게 합니다(번호는 읽지 않습니다). 모델이 무거워 자른 구간 길이만큼 시간이 걸립니다.
- **직접 가리기**: 미리보기 위에 마우스로 네모를 그립니다.
  - 기본은 **항상 가리기**(구간 내내). **여기서부터**·**여기까지**로 일부 시간만 가릴 수 있고, 가리는 시간은 시간 막대 아래에 표시됩니다.
  - **움직임 따라가기**: 그린 네모 속 물체(예: 움직이는 번호판)를 앞뒤로 따라가며 가립니다. 놓치면 그 지점에서 멈추고 몇 초에서 멈췄는지 알려 주므로, 그 장면에서 다시 그리면 됩니다.
- 장면이 바뀌는 곳(화면 전환)을 찾아, 다음 장면의 가림이 미리 나타나지 않게 합니다.
- 블러 세기는 모든 가림에서 화면 긴 변의 8%입니다.

### 알아 둘 점

- 자동 가리기는 놓치거나 잘못 가리는 곳이 있을 수 있습니다. 저장 전에 미리보기로 확인하고, 놓친 곳은 직접 가리기로 보충해 주세요.
- 번호판 모델은 공개 데이터(해외·한국 번호판)로 학습했습니다. 밤·비 장면, 아주 먼 번호판, 번호판만 한 크기의 표지판·건물에서 틀릴 수 있습니다. 강사님 블랙박스 영상으로 다시 학습할 예정입니다.
- 번호판 검출 중 높이가 화면의 10%를 넘는 상자는 표지판·건물로 보고 버립니다. 그보다 크게 찍힌 번호판은 직접 가려 주세요.
- 움직임 따라가기는 밤 장면에서 빨리 놓치는 편입니다.

### 파일 위치

| 무엇 | 위치 |
| --- | --- |
| 저장한 영상 | 기본은 바탕화면 `운전강의_YYYYMMDD_HHMM.mp4` (4단계에서 바꿀 수 있음) |
| 받은 원본 | `%LOCALAPPDATA%\운전영상편집기\work\` (앱 시작 시 7일 지난 파일 삭제) |
| 로그 | `%LOCALAPPDATA%\운전영상편집기\logs\app.log` (문제가 생기면 이 파일을 보내 주세요) |

## 개발

WSL(Linux)에서 개발하고 Windows용으로 크로스 빌드합니다.

### 준비

- Go 1.25 이상, Node.js(npm), [Wails v2](https://wails.io) CLI
- mingw-w64: `sudo apt install gcc-mingw-w64-x86-64` (ONNX Runtime 바인딩이 cgo라 필요)
- 외부 도구 받기: `scripts/fetch-tools.sh windows`, `ml`, `vcrt`, `linux` → `tools/<종류>/`
- 번호판 모델 `tools/models/plate_yolox_tiny.onnx`는 직접 학습한 파일이라 git에 없습니다. `scripts/plate`로 학습·변환해 넣습니다.

### 빌드와 배포 파일

```bash
scripts/package.sh            # dist/운전영상편집기/ 폴더와 dist/운전영상편집기.zip
scripts/package.sh --no-zip   # 폴더만
```

### 검사

```bash
go test ./...
scripts/qa/run.sh   # Go 테스트·vet·staticcheck, API 명세, 주석 규칙, 프론트 타입, 모의 UI 테스트
```

Go App 메서드·요청/응답 구조체·이벤트·안내 문장을 바꾸면 `go run ./cmd/apispec`으로 [docs/api.json](docs/api.json)과 `frontend/src/apiSpec.js`를 다시 만듭니다(직접 고치지 않음).

### 개발용 명령(devcli)

WSL에서 쓸 때는 `SIMPLEEDIT_BIN=$PWD/tools/linux`를 앞에 붙입니다.

```bash
go run ./cmd/devcli analyze <원본> <출력> <시작초> <끝초> [face|plate]   # 자동 가리기 결과 영상 + JSON
go run ./cmd/devcli track <원본> <출력> <시작초> <끝초> <그린초> x,y,w,h  # 움직임 따라가기
```

### 구조

```
main.go, app.go           Wails 설정, 바인딩 메서드(불러오기·자르기·저장)
automask_app.go           얼굴·번호판 자동 가리기 바인딩
track_app.go              움직임 따라가기 바인딩
video_serve.go            미리보기 영상 서빙(/video/current)
internal/download/        yt-dlp 실행·진행률
internal/media/           ffprobe, ffmpeg 프레임 파이프(디코드 → Go 블러 → 인코드)
internal/detect/          ONNX Runtime 추론: YuNet(얼굴), YOLOX(번호판), VitTrack(추적)
internal/automask/        분석·트랙 연결·보간·장면 전환·따라가기·블러
internal/frame/           프레임 형식(BGR, YUV420)과 블러
internal/job/             취소 가능한 단일 작업 관리
internal/tools/           bin/ 경로, 숨김 프로세스
frontend/src/             마법사 화면(steps/), 미리보기 위 그리기(mask/)
scripts/plate/            번호판 YOLOX 학습·ONNX 내보내기·INT8 변환(Colab)
```

## 모델과 외부 도구

| 구성 요소 | 출처 | 라이선스 |
| --- | --- | --- |
| 얼굴 `face_yunet_2026may.onnx` | YuNet, OpenCV Zoo | MIT |
| 추적 `object_tracking_vittrack_2023sep.onnx` | VitTrack, OpenCV Zoo | Apache-2.0 |
| 번호판 `plate_yolox_tiny.onnx` | YOLOX-Tiny를 번호판 1종류로 이어 학습, INT8 변환 | 코드 Apache-2.0, 학습 데이터 CC BY 4.0 |
| ONNX Runtime 1.29.0 | Microsoft | MIT |
| FFmpeg | BtbN/FFmpeg-Builds (win64 gpl) | GPL |
| yt-dlp | yt-dlp | Unlicense |
| Deno | denoland (yt-dlp 유튜브 처리용) | MIT |
| VC++ 런타임 DLL 4개 | Microsoft 재배포 가능 패키지 | Microsoft 재배포 조건 |

번호판 학습 데이터 출처(CC BY 4.0 표시)는 [docs/model_notice.txt](docs/model_notice.txt)에 있고, 배포 zip에는 `bin\models\출처.txt`로 들어갑니다.
