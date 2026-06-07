package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Start(exe string, args []string) error {
	if running, _ := Status(); running {
		return errors.New("server is already running")
	}
	if err := os.MkdirAll(runDir(), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(installDir(), 0755); err != nil {
		return err
	}

	log, err := os.OpenFile(logFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer log.Close()

	cmdArgs := append([]string{"serve"}, args...)
	cmd := exec.Command(exe, cmdArgs...)
	cmd.Env = append(os.Environ(), "CF233_CONFIG="+configFile())
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Dir = filepath.Dir(exe)
	if isWindows() {
		cmd.SysProcAttr = windowsHideWindow()
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return os.WriteFile(pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
}

func Stop() error {
	pid, err := readPID()
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if isWindows() {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	} else {
		_ = proc.Signal(os.Interrupt)
	}
	_ = os.Remove(pidFile())
	return nil
}

func Restart(exe string, args []string) error {
	_ = Stop()
	return Start(exe, args)
}

func Status() (bool, int) {
	pid, err := readPID()
	if err != nil {
		return false, 0
	}
	if isWindows() {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid)).CombinedOutput()
		return err == nil && strings.Contains(string(out), strconv.Itoa(pid)), pid
	}
	err = exec.Command("kill", "-0", strconv.Itoa(pid)).Run()
	return err == nil, pid
}

func readPID() (int, error) {
	data, err := os.ReadFile(pidFile())
	if err != nil {
		return 0, errors.New("server is not running")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}
