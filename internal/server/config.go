package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port              string
	TCPPort           string
	UDPPort           string
	EnableTCP         bool
	EnableUDP         bool
	DataDir           string
	RuntimeDir        string
	NodeBin           string
	NPMBin            string
	DefaultUsername   string
	DefaultPassword   string
	InvocationTimeout time.Duration
}

func ConfigFromEnv() Config {
	timeout := 15 * time.Second
	if raw := os.Getenv("CF233_INVOCATION_TIMEOUT_SECONDS"); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	return Config{
		Port:              valueOrDefault(os.Getenv("PORT"), "6988"),
		TCPPort:           valueOrDefault(os.Getenv("CF233_TCP_PORT"), "6989"),
		UDPPort:           valueOrDefault(os.Getenv("CF233_UDP_PORT"), "6990"),
		EnableTCP:         boolFromEnv("CF233_ENABLE_TCP", false),
		EnableUDP:         boolFromEnv("CF233_ENABLE_UDP", false),
		DataDir:           valueOrDefault(os.Getenv("CF233_DATA_DIR"), filepath.Join(".", "data")),
		RuntimeDir:        valueOrDefault(os.Getenv("CF233_RUNTIME_DIR"), filepath.Join(".", "runtimes")),
		NodeBin:           valueOrDefault(os.Getenv("CF233_NODE_BIN"), "node"),
		NPMBin:            valueOrDefault(os.Getenv("CF233_NPM_BIN"), npmCommand()),
		DefaultUsername:   valueOrDefault(os.Getenv("CF233_DEFAULT_USERNAME"), "root"),
		DefaultPassword:   valueOrDefault(os.Getenv("CF233_DEFAULT_PASSWORD"), "root"),
		InvocationTimeout: timeout,
	}
}

func boolFromEnv(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func npmCommand() string {
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		return "npm.cmd"
	}
	return "npm"
}
