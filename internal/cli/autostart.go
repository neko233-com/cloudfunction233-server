package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func EnableAutostart(exe string) error {
	if isWindows() {
		taskName := appName
		cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/SC", "ONSTART", "/TR", fmt.Sprintf(`"%s" start`, exe), "/RL", "HIGHEST", "/F")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, string(out))
		}
		return nil
	}
	unit := fmt.Sprintf(`[Unit]
Description=cloudfunction233-server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=CF233_CONFIG=%s
ExecStart=%s serve
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, configFile(), exe)
	path := "/etc/systemd/system/cloudfunction233-server.service"
	if err := os.WriteFile(path, []byte(unit), 0644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", "cloudfunction233-server"}, {"restart", "cloudfunction233-server"}} {
		if out, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, string(out))
		}
	}
	return nil
}

func DisableAutostart() error {
	if isWindows() {
		out, err := exec.Command("schtasks", "/Delete", "/TN", appName, "/F").CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, string(out))
		}
		return nil
	}
	_ = exec.Command("systemctl", "disable", "--now", "cloudfunction233-server").Run()
	_ = os.Remove(filepath.Join("/etc/systemd/system", "cloudfunction233-server.service"))
	_ = exec.Command("systemctl", "daemon-reload").Run()
	return nil
}
