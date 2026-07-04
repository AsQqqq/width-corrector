package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// runtime.json - «визитка» работающей копии: по ней новый запуск понимает,
// что копия уже поднята, и на каком порту её искать.
type runtimeInfo struct {
	PID  int    `json:"pid"`
	Port int    `json:"port"`
	URL  string `json:"url"`
}

func runtimePath(configsDir string) string {
	return filepath.Join(configsDir, "runtime.json")
}

func writeRuntimeInfo(configsDir string, port int) {
	info := runtimeInfo{
		PID:  os.Getpid(),
		Port: port,
		URL:  fmt.Sprintf("http://127.0.0.1:%d", port),
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	os.WriteFile(runtimePath(configsDir), data, 0o644)
}

func removeRuntimeInfo(configsDir string) {
	os.Remove(runtimePath(configsDir))
}

func readRuntimeInfo(configsDir string) (runtimeInfo, bool) {
	data, err := os.ReadFile(runtimePath(configsDir))
	if err != nil {
		return runtimeInfo{}, false
	}
	var info runtimeInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return runtimeInfo{}, false
	}
	return info, true
}

// existingInstanceURL возвращает URL уже работающей копии, если она есть.
// Проверка идёт ПО ПРОЦЕССАМ (жив ли PID и наш ли это исполняемый файл),
// а не по попытке достучаться до URL - так чужая программа на нашем порту
// не будет ошибочно принята за нашу копию.
func existingInstanceURL(configsDir string) (string, bool) {
	info, ok := readRuntimeInfo(configsDir)
	if !ok {
		return "", false
	}
	if info.PID == os.Getpid() {
		return "", false
	}
	if !isOurProcess(info.PID) {
		// копия завершилась (аварийно?) либо PID занят чужим процессом -
		// «визитка» устарела, чистим её и запускаемся сами
		removeRuntimeInfo(configsDir)
		return "", false
	}
	url := info.URL
	if url == "" {
		url = fmt.Sprintf("http://127.0.0.1:%d", info.Port)
	}
	return url, true
}

// isOurProcess проверяет, что процесс с данным PID жив и это именно наш
// исполняемый файл (по имени) - защита от переиспользования PID системой.
func isOurProcess(pid int) bool {
	if pid <= 0 {
		return false
	}
	self, err := os.Executable()
	name := ""
	if err == nil {
		name = filepath.Base(self)
	}

	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH")
		hideConsole(cmd)
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		s := string(out)
		// нет процесса -> tasklist пишет «No tasks are running...» (или INFO:)
		if strings.TrimSpace(s) == "" || strings.Contains(s, "No tasks") || strings.HasPrefix(s, "INFO:") {
			return false
		}
		if name != "" && !strings.Contains(strings.ToLower(s), strings.ToLower(name)) {
			return false
		}
		return true

	default: // darwin, linux
		cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
		hideConsole(cmd)
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		comm := strings.TrimSpace(string(out))
		if comm == "" {
			return false
		}
		if name != "" && filepath.Base(comm) != name {
			return false
		}
		return true
	}
}

// listenFreePort занимает первый свободный порт начиная со start.
// Если весь диапазон занят - просит любой свободный порт у ОС.
func listenFreePort(start int) (net.Listener, int, error) {
	for port := start; port < start+50; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return ln, port, nil
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	return ln, ln.Addr().(*net.TCPAddr).Port, nil
}
