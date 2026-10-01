package appdir

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	OutputDir string `json:"outputDir"`
}

func settingsPath() string { return filepath.Join(Base(), "settings.json") }

// 파일이 없거나 JSON이 깨지면 빈 설정을 준다.
func LoadSettings() Settings {
	var s Settings
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

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

// 저장 폴더가 비었거나 사라졌으면 바탕화면을 준다.
func (s Settings) OutputDirOrDesktop() string {
	if s.OutputDir != "" {
		if st, err := os.Stat(s.OutputDir); err == nil && st.IsDir() {
			return s.OutputDir
		}
	}
	return Desktop()
}
