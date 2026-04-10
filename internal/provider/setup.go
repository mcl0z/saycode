package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"saycoding/internal/config"
	"saycoding/internal/model"
	"saycoding/internal/types"
	"saycoding/internal/ui"
)

func RunSetup(ctx context.Context, cfg types.Config) (types.Config, error) {
	ui.ShowBanner("Provider setup")
	baseURL, err := promptValue("base_url", cfg.BaseURL)
	if err != nil {
		return cfg, err
	}
	apiKey, err := promptValue("api_key", cfg.APIKey)
	if err != nil {
		return cfg, err
	}
	next := cfg
	next.BaseURL = strings.TrimRight(baseURL, "/")
	next.APIKey = strings.TrimSpace(apiKey)
	client := model.NewClient(next.BaseURL, next.APIKey, next.ShellTimeoutSec)
	models, err := client.ListModels(ctx)
	if err != nil {
		return cfg, err
	}
	if len(models) == 0 {
		return cfg, fmt.Errorf("provider returned no models")
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	options := make([]string, 0, len(models))
	for _, item := range models {
		options = append(options, item.ID)
	}
	idx, err := ui.Choose("Available models", options)
	if err != nil {
		return cfg, err
	}
	next.Model = options[idx]
	next.ContextWindow = models[idx].ContextLimit()
	if err := config.Save(next); err != nil {
		return cfg, err
	}
	return next, nil
}

func promptValue(label, current string) (string, error) {
	text, err := ui.ReadLineWithPrompt(fmt.Sprintf("%s [%s]: ", label, current))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return current, nil
	}
	return text, nil
}
