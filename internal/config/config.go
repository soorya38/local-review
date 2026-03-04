// Package config manages persistent user configuration for lr.
// Settings are stored in a simple key=value file at $XDG_CONFIG_HOME/lr/config
// (falling back to ~/.config/lr/config on all platforms).
//
// Priority for every setting (highest → lowest):
//
//	CLI flag  >  environment variable  >  config file  >  built-in default
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// KeyGroqKey is the config file key for the Groq API key.
	KeyGroqKey = "groq-key"
	// KeyStandards is the config file key for the coding-standards file path.
	KeyStandards = "standards"
)

// Config holds all persisted lr settings.
type Config struct {
	// GroqKey is the Groq API key read from the config file.
	GroqKey string
	// Standards is the path to the coding-standards file read from the config file.
	Standards string
}

// Load reads the config file and returns the parsed Config.
// Missing keys are silently left at their zero value.
// If the file does not exist, an empty Config is returned without error.
func Load() (*Config, error) {
	path, err := filePath()
	if err != nil {
		return &Config{}, nil //nolint:nilerr // config file is optional
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to open config file %q: %w", path, err)
	}
	defer f.Close()

	cfg := &Config{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case KeyGroqKey:
			cfg.GroqKey = strings.TrimSpace(value)
		case KeyStandards:
			cfg.Standards = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return cfg, nil
}

// Set writes a single key=value pair to the config file, creating the file and
// its parent directory if they do not already exist.
// Existing values for the same key are overwritten; all other keys are preserved.
func Set(key, value string) error {
	path, err := filePath()
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing lines so we can update in place.
	existing := map[string]string{}
	order := []string{}

	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			k = strings.TrimSpace(k)
			if _, seen := existing[k]; !seen {
				order = append(order, k)
			}
			existing[k] = strings.TrimSpace(v)
		}
		f.Close()
	}

	// Upsert the target key.
	if _, seen := existing[key]; !seen {
		order = append(order, key)
	}
	existing[key] = value

	// Write back.
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "# lr configuration — managed by `lr config set`")
	for _, k := range order {
		fmt.Fprintf(w, "%s = %s\n", k, existing[k])
	}
	return w.Flush()
}

// List returns all key=value pairs currently stored in the config file.
// Keys whose values are empty are omitted.
func List() (map[string]string, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if cfg.GroqKey != "" {
		out[KeyGroqKey] = cfg.GroqKey
	}
	if cfg.Standards != "" {
		out[KeyStandards] = cfg.Standards
	}
	return out, nil
}

// FilePath returns the resolved path to the config file (exposed for display).
func FilePath() (string, error) {
	return filePath()
}

// filePath returns $XDG_CONFIG_HOME/lr/config or ~/.config/lr/config.
func filePath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "lr", "config"), nil
}
