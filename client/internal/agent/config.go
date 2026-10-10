package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL                string `json:"server_url"`
	AgentToken               string `json:"agent_token"`
	ComputerID               string `json:"computer_id"`
	RoomID                   string `json:"room_id"`
	Language                 string `json:"language"`
	HeartbeatIntervalSeconds int    `json:"heartbeat_interval_seconds"`
	Version                  string `json:"version"`
}

func DefaultConfigPath() string {
	return `C:\Program Files\SchoolAgent\config.json`
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	if cfg.HeartbeatIntervalSeconds <= 0 {
		cfg.HeartbeatIntervalSeconds = 30
	}
	if cfg.ServerURL == "" || cfg.ComputerID == "" || cfg.AgentToken == "" {
		return Config{}, fmt.Errorf("missing required config fields")
	}
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
