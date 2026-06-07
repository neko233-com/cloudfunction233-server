//go:build !windows

package cli

import "syscall"

func windowsHideWindow() *syscall.SysProcAttr {
	return nil
}
