package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"saycoding/internal/app"
	"saycoding/internal/config"
	"saycoding/internal/provider"
	"saycoding/internal/session"
	"saycoding/internal/tui"
	"saycoding/internal/types"
)

func Run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := session.NewStore()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		sess, err := app.NewSession(store)
		if err != nil {
			return err
		}
		return tui.Run(context.Background(), app.New(cfg, store, sess))
	}
	switch args[0] {
	case "run":
		if len(args) < 2 {
			return fmt.Errorf("usage: saycoding run \"prompt\"")
		}
		sess, err := app.NewSession(store)
		if err != nil {
			return err
		}
		return app.New(cfg, store, sess).RunPrompt(context.Background(), strings.Join(args[1:], " "))
	case "resume":
		if len(args) < 2 {
			return fmt.Errorf("usage: saycoding resume <session-id>")
		}
		sessionID := args[1]
		if sessionID == "latest" {
			latest, err := store.Latest()
			if err != nil {
				return err
			}
			if latest == nil {
				return fmt.Errorf("no saved sessions")
			}
			sessionID = latest.ID
		}
		sess, err := store.Load(sessionID)
		if err != nil {
			return err
		}
		return tui.Run(context.Background(), app.New(cfg, store, sess))
	case "config":
		return runConfig(cfg, args[1:])
	case "doctor":
		return runDoctor(cfg)
	case "status":
		return runStatus(cfg, store)
	case "sessions":
		return runSessions(store)
	case "provider":
		next, err := provider.RunSetup(context.Background(), cfg)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "provider saved: %s\n", next.Model)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runConfig(cfg any, args []string) error {
	if len(args) == 0 {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: saycoding config <key> <value>")
	}
	loaded, err := config.Load()
	if err != nil {
		return err
	}
	switch args[0] {
	case "base_url":
		loaded.BaseURL = args[1]
	case "model":
		loaded.Model = args[1]
		loaded.ContextWindow = 0
	case "api_key_env":
		loaded.APIKeyEnv = args[1]
	case "api_key":
		loaded.APIKey = args[1]
	case "context_window":
		value, err := parseCount(args[1])
		if err != nil {
			return fmt.Errorf("invalid context_window: %s", args[1])
		}
		loaded.ContextWindow = value
	default:
		return fmt.Errorf("unknown config key: %s", args[0])
	}
	if err := config.Save(loaded); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "saved")
	return nil
}

func runDoctor(cfg any) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println("config ok")
	fmt.Println(string(data))
	return nil
}

func runStatus(cfg types.Config, store *session.Store) error {
	latest, err := store.Latest()
	if err != nil {
		return err
	}
	idx, err := store.List()
	if err != nil {
		return err
	}
	fmt.Printf("model: %s\n", cfg.Model)
	if cfg.ContextWindow > 0 {
		fmt.Printf("context_window: %d\n", cfg.ContextWindow)
	}
	fmt.Printf("base_url: %s\n", cfg.BaseURL)
	fmt.Printf("sessions: %d\n", len(idx))
	if latest != nil {
		fmt.Printf("latest_session: %s\n", latest.ID)
		fmt.Printf("latest_cwd: %s\n", latest.CWD)
		fmt.Printf("latest_updated: %s\n", latest.UpdatedAt.Format(time.RFC3339))
	}
	return nil
}

func parseCount(text string) (int, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	multiplier := 1
	switch {
	case strings.HasSuffix(text, "k"):
		multiplier = 1000
		text = strings.TrimSuffix(text, "k")
	case strings.HasSuffix(text, "m"):
		multiplier = 1000000
		text = strings.TrimSuffix(text, "m")
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}
	return value * multiplier, nil
}

func runSessions(store *session.Store) error {
	idx, err := store.List()
	if err != nil {
		return err
	}
	if len(idx) == 0 {
		fmt.Println("no saved sessions")
		return nil
	}
	for _, item := range idx {
		fmt.Printf("%s  %s  %s\n", item.ID, item.UpdatedAt.Format(time.RFC3339), item.CWD)
	}
	return nil
}
