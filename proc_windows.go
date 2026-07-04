//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideConsole прячет окно консоли у вспомогательных команд (tasklist).
// Без этого GUI-приложение (собранное с -H windowsgui) моргает чёрным
// окном консоли при каждом вызове.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
