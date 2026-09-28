package download

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"simpleEdit/internal/applog"
	"simpleEdit/internal/tools"
)

// Stage는 다운로드의 현재 단계다.
type Stage string

const (
	StageStart    Stage = "start"    // 영상 정보 확인 중
	StageDownload Stage = "download" // 받는 중
	StageMerge    Stage = "merge"    // 영상·소리 합치는 중
)

// Progress는 전체 기준 진행 비율(0~1)과 단계다.
type Progress struct {
	Frac  float64
	Stage Stage
}

// 다운로드 실패 원인. app 쪽에서 사용자 문장으로 바꾼다.
var (
	ErrPrivate     = errors.New("비공개 영상")
	ErrUnavailable = errors.New("볼 수 없는 영상")
	ErrAgeLimit    = errors.New("연령 제한 영상")
	ErrLive        = errors.New("생방송 영상")
	ErrBlocked     = errors.New("유튜브가 일시적으로 차단")
	ErrNetwork     = errors.New("네트워크 오류")
	ErrNoSpace     = errors.New("디스크 공간 부족")
	ErrFailed      = errors.New("다운로드 실패")
)

// 출력 줄 앞에 붙는 표시. 사용자 제목 등과 섞이지 않도록 고유한 접두어를 쓴다.
const (
	tagInfo = "SEINFO"
	tagProg = "SEPROG"
	tagFile = "SEFILE"
)

// Args는 yt-dlp 인자. --progress는 --print 대응
func Args(videoURL, workDir string) []string {
	return []string{
		"--ignore-config", "--no-playlist", "--newline", "--no-mtime", "--no-part",
		"--encoding", "utf-8",
		"-f", "bv*[height<=1080][vcodec^=avc1]+ba[ext=m4a]/" +
			"bv*[height<=1080][ext=mp4]+ba[ext=m4a]/" +
			"b[height<=1080][ext=mp4]/b[ext=mp4]/b",
		"--merge-output-format", "mp4",
		"--ffmpeg-location", tools.BinDir(),
		"--match-filter", "!is_live",
		"--no-simulate", "--progress",
		"--print", "before_dl:" + tagInfo + " %(requested_formats.1.format_id|-)s " +
			"%(requested_formats.0.filesize,requested_formats.0.filesize_approx|0)s " +
			"%(requested_formats.1.filesize,requested_formats.1.filesize_approx|0)s",
		"--progress-template", "download:" + tagProg + " %(progress.downloaded_bytes|0)s " +
			"%(progress.total_bytes,progress.total_bytes_estimate|0)s",
		"--print", "after_move:" + tagFile + " %(filepath)s",
		"-o", filepath.Join(workDir, "source.%(ext)s"),
		videoURL,
	}
}

// Download는 영상을 workDir에 받아 경로를 반환한다.
func Download(ctx context.Context, videoURL, workDir string, onProgress func(Progress)) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		path, stderr, err := downloadOnce(ctx, videoURL, workDir, onProgress)
		if err == nil || ctx.Err() != nil || !isForbidden(stderr) {
			return path, err
		}
		lastErr = err
		if attempt == maxAttempts {
			break
		}
		applog.Printf("403 응답, 다시 시도 (%d/%d)", attempt+1, maxAttempts)
		if err := resetDir(workDir); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("%w: %v", ErrBlocked, lastErr)
}

// maxAttempts는 403일 때 시도할 총 횟수다.
const maxAttempts = 2

// downloadOnce는 yt-dlp를 한 번 실행한다.
func downloadOnce(ctx context.Context, videoURL, workDir string, onProgress func(Progress)) (string, string, error) {
	cmd := tools.Command(ctx, tools.YtDlp, Args(videoURL, workDir)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	if err := cmd.Start(); err != nil {
		return "", "", err
	}
	p := newParser(onProgress)
	p.read(stdout)
	err = cmd.Wait()
	stderr := cmd.Stderr()
	switch {
	case ctx.Err() != nil:
		return "", stderr, ctx.Err()
	case err == nil && p.file == "":
		return "", stderr, ErrLive
	case err != nil:
		if c := Classify(stderr); c != nil {
			return "", stderr, c
		}
		return "", stderr, fmt.Errorf("%w: %v", ErrFailed, err)
	}
	return p.file, stderr, nil
}

// isForbidden은 일시적 403 차단인지 본다.
func isForbidden(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "http error 403")
}

// resetDir는 반쯤 받은 파일을 지우고 폴더를 비운다.
func resetDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// Update는 yt-dlp -U로 도구를 최신으로 바꾼다.
func Update(ctx context.Context) error {
	return tools.Command(ctx, tools.YtDlp, "-U").Run()
}

// parser는 영상·소리 진행률을 크기 비율로 합친다.
type parser struct {
	onProgress func(Progress)
	weights    []float64 // 각 스트림이 전체에서 차지하는 비율
	part       int       // 지금 받는 스트림 번호
	lastBytes  int64
	lastFrac   float64
	file       string
}

func newParser(on func(Progress)) *parser {
	return &parser{onProgress: on, weights: []float64{1}}
}

// 합치기 단계에 남겨두는 비율
const mergeShare = 0.03

// 크기를 모를 때 영상·소리 비중
var unknownWeights = []float64{0.9, 0.1}

func (p *parser) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	p.emit(0, StageStart)
	for sc.Scan() {
		p.line(sc.Text())
	}
	_, _ = io.Copy(io.Discard, r)
}

func (p *parser) line(s string) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, tagInfo+" "):
		f := strings.Fields(s)
		if len(f) < 4 {
			return
		}
		if f[1] == "-" || f[1] == "NA" {
			p.weights = []float64{1}
			return
		}
		a, _ := strconv.ParseFloat(f[2], 64)
		b, _ := strconv.ParseFloat(f[3], 64)
		if a > 0 && b > 0 {
			p.weights = []float64{a / (a + b), b / (a + b)}
		} else {
			p.weights = unknownWeights
		}
	case strings.HasPrefix(s, tagProg+" "):
		f := strings.Fields(s)
		if len(f) < 3 {
			return
		}
		done, _ := strconv.ParseFloat(f[1], 64)
		total, _ := strconv.ParseFloat(f[2], 64)
		if int64(done) < p.lastBytes && p.part < len(p.weights)-1 {
			p.part++
		}
		p.lastBytes = int64(done)
		var partFrac float64
		if total > 0 {
			partFrac = min(1, done/total)
		}
		before := 0.0
		for _, w := range p.weights[:p.part] {
			before += w
		}
		frac := (before + p.weights[p.part]*partFrac) * (1 - mergeShare)
		if p.part == len(p.weights)-1 && partFrac >= 1 {
			p.emit(frac, StageMerge)
			return
		}
		p.emit(frac, StageDownload)
	case strings.HasPrefix(s, tagFile+" "):
		p.file = strings.TrimSpace(strings.TrimPrefix(s, tagFile+" "))
	}
}

// emit은 진행률이 뒤로 가지 않게 알린다.
func (p *parser) emit(frac float64, st Stage) {
	if frac < p.lastFrac {
		frac = p.lastFrac
	}
	p.lastFrac = frac
	if p.onProgress != nil {
		p.onProgress(Progress{Frac: frac, Stage: st})
	}
}

// Classify는 yt-dlp stderr에서 실패 원인을 찾는다. 모르면 nil.
func Classify(stderr string) error {
	s := strings.ToLower(stderr)
	has := func(subs ...string) bool {
		for _, x := range subs {
			if strings.Contains(s, x) {
				return true
			}
		}
		return false
	}
	switch {
	case has("no space left", "errno 28", "not enough space"):
		return ErrNoSpace
	case has("private video"):
		return ErrPrivate
	case has("sign in to confirm your age", "age-restricted", "inappropriate for some users"):
		return ErrAgeLimit
	case has("sign in to confirm you", "not a bot", "http error 429"):
		return ErrBlocked
	case has("is_live", "live event", "this live stream", "premieres in"):
		return ErrLive
	case has("video unavailable", "has been removed", "account associated with this video has been terminated",
		"not available in your country", "this video is not available", "members-only", "join this channel"):
		return ErrUnavailable
	case has("getaddrinfo", "failed to resolve", "name resolution", "timed out", "unable to connect",
		"connection refused", "connection reset", "network is unreachable", "unable to download webpage",
		"urlopen error", "remote end closed", "http error 5"):
		return ErrNetwork
	}
	return nil
}
