package config

import (
	"encoding/json"
	"os"
	"sync"
)

type AppConfig struct {
	GeminiAPIKeys  []string `json:"gemini_api_keys"`
	SystemPrompt   string   `json:"system_prompt"`
	AutoReply      bool     `json:"auto_reply"`
	PreferredModel string   `json:"preferred_model"` // e.g. "gemini-2.5-flash", "gemini-2.0-flash", "auto"
	YtDlpPath      string   `json:"yt_dlp_path"`
	FFmpegPath     string   `json:"ffmpeg_path"`
}

var (
	cfgPath = "config.json"
	mu      sync.RWMutex
	current AppConfig
)

func init() {
	current = AppConfig{
		GeminiAPIKeys:  []string{},
		SystemPrompt:   "Eres un asistente virtual de WhatsApp inteligente, conciso, educado y amigable. Responde siempre con claridad y en el idioma del usuario.",
		AutoReply:      true,
		PreferredModel: "auto", // dynamically discovers cheapest flash model
		YtDlpPath:      "yt-dlp",
		FFmpegPath:     "ffmpeg",
	}
}

func LoadConfig() AppConfig {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(cfgPath)
	if err == nil {
		var loaded AppConfig
		if json.Unmarshal(data, &loaded) == nil {
			current = loaded
		}
	}
	return current
}

func SaveConfig(newCfg AppConfig) error {
	mu.Lock()
	defer mu.Unlock()

	current = newCfg
	data, err := json.MarshalIndent(newCfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0644)
}

func GetConfig() AppConfig {
	mu.RLock()
	defer mu.RUnlock()
	return current
}
