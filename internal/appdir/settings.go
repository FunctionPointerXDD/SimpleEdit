package appdir

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings는 다음 실행 때도 기억할 설정이다.
type Settings struct {
	OutputDir string `json:"outputDir"`
}

func settingsPath() string { return filepath.Join(Base(), "settings.json") }

// LoadSettings는 설정을 읽는다. 없거나 깨지면 빈 값.
func LoadSettings() Settings {
	var s Settings
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

// SaveSettings는 설정을 파일에 쓴다.
func SaveSettings(s Settings) error {
	if err := os.MkdirAll(Base(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), data, 0o644)
}

// OutputDirOrDesktop은 저장 폴더. 없으면 바탕화면.
func (s Settings) OutputDirOrDesktop() string {
	if s.OutputDir != "" {
		if st, err := os.Stat(s.OutputDir); err == nil && st.IsDir() {
			return s.OutputDir
		}
	}
	return Desktop()
}
