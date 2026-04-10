package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"saycoding/internal/types"
)

const dirName = ".saycoding"

func RootDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dirName), nil
}

func Default() types.Config {
	return types.Config{
		BaseURL:              "https://api.openai.com/v1",
		APIKeyEnv:            "OPENAI_API_KEY",
		Model:                "gpt-4o-mini",
		MaxSteps:             48,
		ShellTimeoutSec:      30,
		AutoApproveWorkspace: true,
		Theme:                "default",
	}
}

func Load() (types.Config, error) {
	root, err := RootDir()
	if err != nil {
		return types.Config{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return types.Config{}, err
	}
	path := filepath.Join(root, "config.json")
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg.APIKey = os.Getenv(cfg.APIKeyEnv)
		return cfg, Save(cfg)
	}
	if err != nil {
		return types.Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return types.Config{}, err
	}
	if cfg.APIKey == "" && cfg.APIKeyEnv != "" {
		cfg.APIKey = os.Getenv(cfg.APIKeyEnv)
	}
	return cfg, nil
}

func Save(cfg types.Config) error {
	root, err := RootDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	path := filepath.Join(root, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
