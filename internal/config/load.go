package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load() (*Config, error) {
	return LoadWithOptions(LoadOptions{})
}

func LoadWithOptions(opts LoadOptions) (*Config, error) {
	cfg := defaultConfig()

	configFile := strings.TrimSpace(opts.ConfigFile)
	if configFile == "" {
		configFile = "config.yaml"
	}

	debugConfigured := false
	if raw, err := os.ReadFile(filepath.Clean(configFile)); err == nil {
		rawText := string(raw)
		debugConfigured = configHasAppDebug(rawText)
		expanded := expandEnv(rawText)
		decoder := yaml.NewDecoder(strings.NewReader(expanded))
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		var extra yaml.Node
		if err := decoder.Decode(&extra); err == nil {
			return nil, fmt.Errorf("unmarshal config: multiple YAML documents are not allowed")
		} else if err != io.EOF {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}

	applyEnvironmentOverrides(&cfg)
	cfg.normalize(debugConfigured || strings.TrimSpace(os.Getenv("APP_DEBUG")) != "")
	if err := cfg.Validate(opts.Service); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func configHasAppDebug(rawConfig string) bool {
	var root yaml.Node
	expanded := expandEnv(rawConfig)
	if err := yaml.Unmarshal([]byte(expanded), &root); err != nil || len(root.Content) == 0 {
		return false
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value != "app" || doc.Content[i+1].Kind != yaml.MappingNode {
			continue
		}
		app := doc.Content[i+1]
		for j := 0; j+1 < len(app.Content); j += 2 {
			if app.Content[j].Value == "debug" {
				value := app.Content[j+1]
				return strings.TrimSpace(value.Value) != "" && value.Tag != "!!null"
			}
		}
	}
	return false
}
