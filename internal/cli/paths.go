package cli

import (
	"os"
	"path/filepath"
)

const appName = "cloudfunction233-server"

func installDir() string {
	if dir := os.Getenv("CF233_HOME"); dir != "" {
		return dir
	}
	if isWindows() {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = os.TempDir()
		}
		return filepath.Join(base, appName)
	}
	return filepath.Join("/opt", appName)
}

func runDir() string {
	if isWindows() {
		return installDir()
	}
	return filepath.Join("/var", "run", appName)
}

func pidFile() string {
	return filepath.Join(runDir(), appName+".pid")
}

func logFile() string {
	return filepath.Join(installDir(), appName+".log")
}

func configFile() string {
	if path := os.Getenv("CF233_CONFIG"); path != "" {
		return path
	}
	return filepath.Join(installDir(), "config.yaml")
}

func isWindows() bool {
	return filepath.Separator == '\\'
}
