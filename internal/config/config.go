package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const DefaultURL = "https://jenkins.example.com/"

type Config struct {
	URL    string `json:"url"`
	CAFile string `json:"ca_file,omitempty"`
}

type Paths struct{ ConfigFile, StateDir string }

func DefaultPaths() (Paths, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	stateRoot := os.Getenv("XDG_STATE_HOME")
	if stateRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
		if runtime.GOOS == "darwin" {
			stateRoot = filepath.Join(home, "Library", "Application Support")
		} else {
			stateRoot = filepath.Join(home, ".local", "state")
		}
	}
	return Paths{ConfigFile: filepath.Join(configRoot, "jkins", "config.json"), StateDir: filepath.Join(stateRoot, "jkins")}, nil
}

func Load(path string) (Config, error) {
	config := Config{URL: DefaultURL}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, errors.New("invalid config JSON")
	}
	parsed, err := url.Parse(config.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Config{}, errors.New("config url must be an HTTPS controller URL")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return Config{}, errors.New("config url must not contain a path")
	}
	if strings.Contains(config.CAFile, "\x00") || (config.CAFile != "" && !filepath.IsAbs(config.CAFile)) {
		return Config{}, errors.New("config ca_file must be an absolute path")
	}
	return config, nil
}
