package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ncruces/zenity"
)

// appVersion - текущая версия сборки. Сравнивается с тегом последнего релиза
// на GitHub (releases/latest). Поднимай её перед каждым релизом.
const appVersion = "1.0.4"

const (
	githubOwner = "AsQqqq"
	githubRepo  = "width-corrector"
)

// ghRelease - нужные поля ответа GitHub API (releases/latest).
type ghRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// checkForUpdate смотрит последний релиз на GitHub и, если версия новее,
// предлагает обновиться. manual=true - вызвано вручную из трея (тогда
// показываем ответ «у вас последняя версия» / ошибки), false - тихая
// проверка при старте.
func (s *server) checkForUpdate(manual bool) {
	// Самозамена бинарника реализована только под Windows.
	if runtime.GOOS != "windows" {
		if manual {
			zenity.Info(s.tr(
				"Auto-update is available only on Windows.",
				"Автообновление доступно только на Windows."),
				zenity.Title("WidthCorrector"))
		}
		return
	}

	rel, err := latestRelease()
	if err != nil {
		if manual {
			zenity.Error(s.tr(
				"Failed to check for updates:\n"+err.Error(),
				"Не удалось проверить обновления:\n"+err.Error()),
				zenity.Title("WidthCorrector"))
		}
		return
	}

	remote := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if remote == "" || compareVersions(remote, appVersion) <= 0 {
		if manual {
			zenity.Info(s.tr(
				"You have the latest version ("+appVersion+").",
				"У вас последняя версия ("+appVersion+")."),
				zenity.Title("WidthCorrector"))
		}
		return
	}

	assetURL, assetSize := pickExeAsset(rel)
	if assetURL == "" {
		if manual {
			zenity.Error(s.tr(
				"The release has no .exe file attached.",
				"К релизу не прикреплён .exe файл."),
				zenity.Title("WidthCorrector"))
		}
		return
	}

	// Спрашиваем пользователя (с кратким списком изменений).
	changelog := strings.TrimSpace(rel.Body)
	if len(changelog) > 600 {
		changelog = changelog[:600] + "…"
	}
	msg := s.tr(
		fmt.Sprintf("A new version %s is available (you have %s).", remote, appVersion),
		fmt.Sprintf("Доступна новая версия %s (у вас %s).", remote, appVersion))
	if changelog != "" {
		msg += "\n\n" + changelog
	}
	msg += s.tr(
		"\n\nUpdate now? The program will restart.",
		"\n\nОбновить сейчас? Программа перезапустится.")

	err = zenity.Question(msg,
		zenity.Title("WidthCorrector"),
		zenity.OKLabel(s.tr("Update", "Обновить")),
		zenity.CancelLabel(s.tr("Later", "Позже")))
	if err != nil {
		return // пользователь отказался (ErrCanceled) или диалог закрыт
	}

	if err := s.applyUpdate(assetURL, assetSize); err != nil {
		zenity.Error(s.tr(
			"Update failed:\n"+err.Error(),
			"Не удалось обновиться:\n"+err.Error()),
			zenity.Title("WidthCorrector"))
	}
}

// applyUpdate скачивает новый бинарник, подменяет текущий .exe и
// перезапускает программу. На Windows работающий .exe нельзя удалить, но
// можно переименовать - поэтому: качаем в .new, текущий переименовываем в
// .old, .new ставим на его место, запускаем и выходим. Хвост .old подчищает
// уже новый процесс при следующем старте (cleanupOldBinary).
func (s *server) applyUpdate(url string, size int64) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	newPath := self + ".new"
	oldPath := self + ".old"

	dlg, derr := zenity.Progress(
		zenity.Title("WidthCorrector"),
		zenity.MaxValue(100),
		zenity.NoCancel())
	if derr == nil && dlg != nil {
		dlg.Text(s.tr("Downloading update…", "Скачивание обновления…"))
		defer dlg.Close()
	}

	os.Remove(newPath) // на случай хвоста от прошлой неудачной попытки
	if err := downloadFile(url, newPath, size, dlg); err != nil {
		os.Remove(newPath)
		return err
	}

	if dlg != nil {
		dlg.Text(s.tr("Installing…", "Установка…"))
		dlg.Complete()
	}

	// Подменяем файл.
	os.Remove(oldPath)
	if err := os.Rename(self, oldPath); err != nil {
		os.Remove(newPath)
		return err
	}
	if err := os.Rename(newPath, self); err != nil {
		os.Rename(oldPath, self) // откат: возвращаем рабочий .exe на место
		os.Remove(newPath)
		return err
	}

	// Перезапуск: снимаем «визитку», стартуем новый бинарник, выходим.
	// Флаг --after-update просит новый процесс не принять нас (ещё живых)
	// за «уже запущенную копию» и дождаться освобождения порта.
	removeRuntimeInfo(s.configsDir)
	if dlg != nil {
		dlg.Close()
	}
	if err := exec.Command(self, "--after-update").Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}

// downloadFile качает url в dest, обновляя прогресс-диалог (если он есть).
func downloadFile(url, dest string, size int64, dlg zenity.ProgressDialog) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "WidthCorrector")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := size
	if total <= 0 {
		total = resp.ContentLength
	}

	buf := make([]byte, 64*1024)
	var written int64
	lastPct := -1
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if dlg != nil && total > 0 {
				if pct := int(written * 100 / total); pct != lastPct {
					dlg.Value(pct)
					lastPct = pct
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	// Защита от «пустого»/битого ответа: настоящий бинарник заметно больше.
	if written < 1024*1024 {
		return fmt.Errorf("downloaded file is too small (%d bytes)", written)
	}
	return nil
}

// latestRelease тянет последний релиз через GitHub API.
func latestRelease() (ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOwner, githubRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ghRelease{}, err
	}
	req.Header.Set("User-Agent", "WidthCorrector")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ghRelease{}, fmt.Errorf("GitHub API HTTP %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ghRelease{}, err
	}
	return rel, nil
}

// pickExeAsset выбирает из релиза .exe: сперва ровно WidthCorrector.exe,
// иначе первый попавшийся .exe.
func pickExeAsset(rel ghRelease) (url string, size int64) {
	for _, a := range rel.Assets {
		if strings.EqualFold(a.Name, "WidthCorrector.exe") {
			return a.URL, a.Size
		}
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			return a.URL, a.Size
		}
	}
	return "", 0
}

// compareVersions сравнивает версии вида "1.2.0": >0 если a новее b, 0 равны, <0 старее.
func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(strings.TrimSpace(pa[i]))
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(strings.TrimSpace(pb[i]))
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

// cleanupOldBinary подчищает хвосты прошлого обновления (.old/.new).
func cleanupOldBinary() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	os.Remove(self + ".old")
	os.Remove(self + ".new")
}
