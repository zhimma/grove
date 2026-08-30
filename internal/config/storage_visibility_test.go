package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStorageDiskVisibilityFieldsDecodeAndDefaultPrivate(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(`storage:
  disks:
    local:
      driver: local
      root: ./storage
      base_url: /storage
`), &cfg); err != nil {
		t.Fatalf("decode private storage disk: %v", err)
	}
	if cfg.Storage.Disks["local"].Public || cfg.Storage.Disks["local"].ServeStatic {
		t.Fatalf("omitted visibility flags must default to private: %#v", cfg.Storage.Disks["local"])
	}

	if err := yaml.Unmarshal([]byte(`storage:
  disks:
    assets:
      driver: local
      root: ./assets
      base_url: /assets
      public: true
      serve_static: true
`), &cfg); err != nil {
		t.Fatalf("decode public storage disk: %v", err)
	}
	disk := cfg.Storage.Disks["assets"]
	if !disk.Public || !disk.ServeStatic {
		t.Fatalf("explicit visibility flags were not decoded: %#v", disk)
	}
}
