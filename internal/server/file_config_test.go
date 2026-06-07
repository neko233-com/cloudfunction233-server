package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureAndLoadYAMLConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Config{
		Port:              "6988",
		TCPPort:           "6989",
		UDPPort:           "6990",
		DataDir:           "./data",
		RuntimeDir:        "./runtimes",
		NodeBin:           "node",
		NPMBin:            "npm",
		DefaultUsername:   "root",
		DefaultPassword:   "root",
		InvocationTimeout: 15 * time.Second,
	}
	if err := EnsureConfigFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "port: \"6988\"") && !strings.Contains(text, "port: 6988") {
		t.Fatalf("config does not look like yaml: %s", text)
	}
	if strings.Contains(text, "{") {
		t.Fatalf("config should be yaml, got json-like content: %s", text)
	}

	fileCfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileCfg.Port != "6988" || fileCfg.RuntimeDir != "./runtimes" {
		t.Fatalf("loaded yaml config = %#v", fileCfg)
	}
}
