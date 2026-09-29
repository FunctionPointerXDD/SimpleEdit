// 대조 테스트는 런타임·모델 경로가 없으면 건너뛴다.

package detect

import (
	"encoding/json"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"simpleEdit/internal/frame"
)

const (
	envORTLib = "SIMPLEEDIT_ORT_LIB"
	envModels = "SIMPLEEDIT_MODELS"
)

type goldenFace struct {
	Box   [4]float32 `json:"box"`
	Score float32    `json:"score"`
}

func TestYuNetMatchesOpenCV(t *testing.T) {
	det := openFaceDetector(t)
	data, err := os.ReadFile("testdata/yunet_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string][]goldenFace
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	for name, want := range golden {
		t.Run(name, func(t *testing.T) {
			got, err := det.Detect(readPNG(t, "testdata/"+name+"_640.png"))
			if err != nil {
				t.Fatal(err)
			}
			sortBoxes(got)
			sort.Slice(want, func(i, j int) bool { return want[i].Box[0] < want[j].Box[0] })
			if len(got) != len(want) {
				t.Fatalf("얼굴 %d개, OpenCV %d개: %+v", len(got), len(want), got)
			}
			for i, w := range want {
				g := got[i]
				d := maxDiff(g.X-w.Box[0], g.Y-w.Box[1], g.W-w.Box[2], g.H-w.Box[3])
				if d > 1 || math.Abs(float64(g.Score-w.Score)) > 0.01 {
					t.Errorf("%d번: got %+v, OpenCV %+v (최대 차 %.3fpx)", i, g, w, d)
				}
				t.Logf("%d번: 좌표 최대 차 %gpx, 점수 차 %g (Go %.6f / OpenCV %.6f)", i, d, g.Score-w.Score, g.Score, w.Score)
			}
		})
	}
}

func openFaceDetector(t *testing.T) *FaceDetector {
	t.Helper()
	lib := os.Getenv(envORTLib)
	if lib == "" {
		lib = defaultORTLib()
	}
	models := os.Getenv(envModels)
	if models == "" {
		models = filepath.Join("..", "..", "tools", "models")
	}
	model := filepath.Join(models, "face_yunet_2023mar.onnx")
	for _, p := range []string{lib, model} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("%s 없음 (scripts/fetch-tools.sh ml)", p)
		}
	}
	if err := Init(lib); err != nil {
		t.Fatal(err)
	}
	det, err := NewFaceDetector(model)
	if err != nil {
		t.Fatal(err)
	}
	det.ScoreThreshold = 0.5
	t.Cleanup(det.Close)
	return det
}

func defaultORTLib() string {
	dir := filepath.Join("..", "..", "tools")
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "windows", "onnxruntime.dll")
	}
	return filepath.Join(dir, "linux", "libonnxruntime.so.1.29.0")
}

func readPNG(t *testing.T, path string) frame.Frame {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return frame.FromImage(img)
}

func sortBoxes(b []Box) {
	sort.Slice(b, func(i, j int) bool { return b[i].X < b[j].X })
}

func maxDiff(vs ...float32) float64 {
	m := 0.0
	for _, v := range vs {
		m = math.Max(m, math.Abs(float64(v)))
	}
	return m
}
