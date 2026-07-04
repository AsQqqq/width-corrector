//go:build !windows

package main

import "os/exec"

// hideConsole - заглушка для не-Windows систем (там консоль не всплывает).
func hideConsole(cmd *exec.Cmd) {}
