package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

type FileConfig struct {
	Port                     string            `json:"port" yaml:"port"`
	TCPPort                  string            `json:"tcpPort" yaml:"tcpPort"`
	UDPPort                  string            `json:"udpPort" yaml:"udpPort"`
	EnableTCP                bool              `json:"enableTcp" yaml:"enableTcp"`
	EnableUDP                bool              `json:"enableUdp" yaml:"enableUdp"`
	DataDir                  string            `json:"dataDir" yaml:"dataDir"`
	RuntimeDir               string            `json:"runtimeDir" yaml:"runtimeDir"`
	NodeBin                  string            `json:"nodeBin" yaml:"nodeBin"`
	NPMBin                   string            `json:"npmBin" yaml:"npmBin"`
	DefaultUsername          string            `json:"defaultUsername" yaml:"defaultUsername"`
	DefaultPassword          string            `json:"defaultPassword" yaml:"defaultPassword"`
	InvocationTimeoutSeconds int               `json:"invocationTimeoutSeconds" yaml:"invocationTimeoutSeconds"`
	Env                      map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

type RuntimeSettings struct {
	InvocationTimeout time.Duration
	NodeBin           string
	NPMBin            string
}

type HotConfig struct {
	path     string
	settings atomic.Value
}

func ConfigFromFileAndEnv() Config {
	cfg := ConfigFromEnv()
	if path := configPath(); path != "" {
		if fileCfg, err := LoadFileConfig(path); err == nil {
			applyFileConfig(&cfg, fileCfg)
		}
	}
	return cfg
}

func LoadFileConfig(path string) (FileConfig, error) {
	var cfg FileConfig
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return cfg, json.Unmarshal(data, &cfg)
	}
	return cfg, yaml.Unmarshal(data, &cfg)
}

func EnsureConfigFile(path string, cfg Config) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	fileCfg := FileConfig{
		Port:                     cfg.Port,
		TCPPort:                  cfg.TCPPort,
		UDPPort:                  cfg.UDPPort,
		EnableTCP:                cfg.EnableTCP,
		EnableUDP:                cfg.EnableUDP,
		DataDir:                  cfg.DataDir,
		RuntimeDir:               cfg.RuntimeDir,
		NodeBin:                  cfg.NodeBin,
		NPMBin:                   cfg.NPMBin,
		DefaultUsername:          cfg.DefaultUsername,
		DefaultPassword:          cfg.DefaultPassword,
		InvocationTimeoutSeconds: int(cfg.InvocationTimeout / time.Second),
	}
	data, err := marshalFileConfig(path, fileCfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func NewHotConfig(path string, cfg Config) *HotConfig {
	hot := &HotConfig{path: path}
	hot.settings.Store(RuntimeSettings{
		InvocationTimeout: cfg.InvocationTimeout,
		NodeBin:           cfg.NodeBin,
		NPMBin:            cfg.NPMBin,
	})
	return hot
}

func (h *HotConfig) Settings() RuntimeSettings {
	if h == nil {
		return RuntimeSettings{InvocationTimeout: 15 * time.Second, NodeBin: "node", NPMBin: npmCommand()}
	}
	return h.settings.Load().(RuntimeSettings)
}

func (h *HotConfig) Watch(stop <-chan struct{}) {
	if h == nil || h.path == "" {
		return
	}
	var lastMod time.Time
	for {
		select {
		case <-stop:
			return
		case <-time.After(2 * time.Second):
			info, err := os.Stat(h.path)
			if err != nil || !info.ModTime().After(lastMod) {
				continue
			}
			lastMod = info.ModTime()
			cfg, err := LoadFileConfig(h.path)
			if err != nil {
				continue
			}
			current := h.Settings()
			next := RuntimeSettings{
				InvocationTimeout: current.InvocationTimeout,
				NodeBin:           valueOrDefault(cfg.NodeBin, current.NodeBin),
				NPMBin:            valueOrDefault(cfg.NPMBin, current.NPMBin),
			}
			if cfg.InvocationTimeoutSeconds > 0 {
				next.InvocationTimeout = time.Duration(cfg.InvocationTimeoutSeconds) * time.Second
			}
			h.settings.Store(next)
		}
	}
}

func applyFileConfig(cfg *Config, fileCfg FileConfig) {
	cfg.Port = valueOrDefault(fileCfg.Port, cfg.Port)
	cfg.TCPPort = valueOrDefault(fileCfg.TCPPort, cfg.TCPPort)
	cfg.UDPPort = valueOrDefault(fileCfg.UDPPort, cfg.UDPPort)
	cfg.EnableTCP = fileCfg.EnableTCP
	cfg.EnableUDP = fileCfg.EnableUDP
	cfg.DataDir = valueOrDefault(fileCfg.DataDir, cfg.DataDir)
	cfg.RuntimeDir = valueOrDefault(fileCfg.RuntimeDir, cfg.RuntimeDir)
	cfg.NodeBin = valueOrDefault(fileCfg.NodeBin, cfg.NodeBin)
	cfg.NPMBin = valueOrDefault(fileCfg.NPMBin, cfg.NPMBin)
	cfg.DefaultUsername = valueOrDefault(fileCfg.DefaultUsername, cfg.DefaultUsername)
	cfg.DefaultPassword = valueOrDefault(fileCfg.DefaultPassword, cfg.DefaultPassword)
	if fileCfg.InvocationTimeoutSeconds > 0 {
		cfg.InvocationTimeout = time.Duration(fileCfg.InvocationTimeoutSeconds) * time.Second
	}
}

func configPath() string {
	if path := os.Getenv("CF233_CONFIG"); path != "" {
		return path
	}
	if path := os.Getenv("CLOUDFUNCTION233_CONFIG"); path != "" {
		return path
	}
	yamlPath := filepath.Join(".", "config.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath
	}
	jsonPath := filepath.Join(".", "config.json")
	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}
	return yamlPath
}

func marshalFileConfig(path string, cfg FileConfig) ([]byte, error) {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return json.MarshalIndent(cfg, "", "  ")
	}
	return yaml.Marshal(cfg)
}
