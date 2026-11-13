package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var adbPath string

func findAdb() (string, error) {
	exe := "adb"
	if runtime.GOOS == "windows" {
		exe = "adb.exe"
	}
	if p, err := exec.LookPath(exe); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	var roots []string
	for _, e := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := os.Getenv(e); v != "" {
			roots = append(roots, v)
		}
	}
	switch runtime.GOOS {
	case "windows":
		roots = append(roots, filepath.Join(os.Getenv("LOCALAPPDATA"), "Android", "Sdk"), `C:\Android\Sdk`, `D:\AndroidSDK`)
	case "darwin":
		roots = append(roots, filepath.Join(home, "Library", "Android", "sdk"))
	default:
		roots = append(roots, filepath.Join(home, "Android", "Sdk"))
	}
	for _, r := range roots {
		p := filepath.Join(r, "platform-tools", exe)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	for _, p := range []string{"/opt/homebrew/bin/adb", "/usr/local/bin/adb"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("adb nao encontrado. Instale o Android SDK Platform-Tools e coloque no PATH (ou defina ANDROID_HOME)")
}

func adbCmd(args ...string) *exec.Cmd {
	c := exec.Command(adbPath, args...)
	hideWindow(c)
	return c
}

// adb retorna a saida combinada (stdout+stderr) sem \r
func adb(args ...string) (string, error) {
	out, err := adbCmd(args...).CombinedOutput()
	return strings.ReplaceAll(string(out), "\r", ""), err
}

func shell(d Device, cmd ...string) string {
	out, _ := adb(append([]string{"-s", d.Serial, "shell"}, cmd...)...)
	return strings.TrimSpace(out)
}

// ---------------------------------------------------------------------------
// Dispositivos
// ---------------------------------------------------------------------------

type Device struct {
	Serial string
	State  string
	Model  string
}

func (d Device) Label() string {
	if d.Model != "" {
		return fmt.Sprintf("%s (%s)", d.Model, d.Serial)
	}
	return d.Serial
}

func listDevices() []Device {
	out, _ := adb("devices", "-l")
	var devs []Device
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] == "List" || strings.HasPrefix(f[0], "*") {
			continue
		}
		d := Device{Serial: f[0], State: f[1]}
		for _, kv := range f[2:] {
			if strings.HasPrefix(kv, "model:") {
				d.Model = strings.ReplaceAll(kv[6:], "_", " ")
			}
		}
		devs = append(devs, d)
	}
	return devs
}

type config struct {
	LastDevice string `json:"lastDevice"`
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "FastInstallAPK", "config.json")
}

func loadConfig() config {
	var c config
	if p := configPath(); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			json.Unmarshal(b, &c)
		}
	}
	return c
}

func saveConfig(c config) {
	if p := configPath(); p != "" {
		os.MkdirAll(filepath.Dir(p), 0o755)
		b, _ := json.MarshalIndent(c, "", "  ")
		os.WriteFile(p, b, 0o644)
	}
}

var stdin = bufio.NewReader(os.Stdin)

func prompt(msg string) string {
	fmt.Print(msg)
	s, _ := stdin.ReadString('\n')
	return strings.TrimSpace(s)
}

type devMode int

const (
	devOne      devMode = iota // um aparelho
	devAllowAll                // pode escolher "todos"
	devAll                     // todos, sem perguntar
)

// selectDevices escolhe o(s) aparelho(s). Lembra o ultimo escolhido (Enter = ultimo).
func selectDevices(mode devMode) ([]Device, error) {
	var ready, other []Device
	for _, d := range listDevices() {
		if d.State == "device" {
			ready = append(ready, d)
		} else {
			other = append(other, d)
		}
	}
	if len(ready) == 0 {
		msg := "Nenhum aparelho pronto encontrado."
		if len(other) > 0 {
			var s []string
			for _, d := range other {
				s = append(s, d.Serial+" ("+d.State+")")
			}
			msg += "\n    Com problema: " + strings.Join(s, ", ")
			msg += "\n    Se aparecer \"unauthorized\", aceite a depuracao USB no celular."
		}
		return nil, errors.New(msg)
	}
	if len(ready) == 1 || mode == devAll {
		return ready, nil
	}

	cfg := loadConfig()
	def := -1
	fmt.Println()
	warn("Mais de um aparelho conectado:")
	for i, d := range ready {
		mark := ""
		if d.Serial == cfg.LastDevice {
			def = i
			mark = dim("  <- ultimo usado")
		}
		fmt.Printf("  [%d] %s%s\n", i+1, d.Label(), mark)
	}
	if mode == devAllowAll {
		fmt.Println("  [0] Todos")
	}
	for {
		q := "Escolha o aparelho: "
		if def >= 0 {
			q = fmt.Sprintf("Escolha o aparelho [Enter = %d]: ", def+1)
		}
		s := prompt(q)
		if s == "" && def >= 0 {
			return []Device{ready[def]}, nil
		}
		if s == "0" && mode == devAllowAll {
			return ready, nil
		}
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(ready) {
			cfg.LastDevice = ready[n-1].Serial
			saveConfig(cfg)
			return []Device{ready[n-1]}, nil
		}
	}
}

// ---------------------------------------------------------------------------
// Operacoes no app
// ---------------------------------------------------------------------------

var successRe = regexp.MustCompile(`(?m)^Success`)

func isInstalled(d Device, pkg string) bool {
	out := shell(d, "pm", "list", "packages", pkg)
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == "package:"+pkg {
			return true
		}
	}
	return false
}

func launchApp(d Device, pkg string) {
	step("Abrindo " + pkg + " ...")
	out := shell(d, "monkey", "-p", pkg, "-c", "android.intent.category.LAUNCHER", "1")
	if strings.Contains(out, "No activities found") || strings.Contains(out, "monkey aborted") {
		warn("    App sem activity de launcher para abrir.")
		return
	}
	ok("App aberto.")
}

func pidOf(d Device, pkg string) string {
	out := shell(d, "pidof", pkg)
	if f := strings.Fields(out); len(f) > 0 {
		if _, err := strconv.Atoi(f[0]); err == nil {
			return f[0]
		}
	}
	return ""
}

func waitPid(d Device, pkg string, timeout time.Duration) string {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if p := pidOf(d, pkg); p != "" {
			return p
		}
		time.Sleep(400 * time.Millisecond)
	}
	return ""
}

type installedInfo struct {
	VersionCode string
	VersionName string
	MinSdk      string
	TargetSdk   string
	Paths       []string
}

func getInstalled(d Device, pkg string) *installedInfo {
	if !isInstalled(d, pkg) {
		return nil
	}
	ii := &installedInfo{}
	dump := shell(d, "dumpsys", "package", pkg)
	if m := regexp.MustCompile(`versionCode=(\d+)(?: minSdk=(\d+))?(?: targetSdk=(\d+))?`).FindStringSubmatch(dump); m != nil {
		ii.VersionCode, ii.MinSdk, ii.TargetSdk = m[1], m[2], m[3]
	}
	if m := regexp.MustCompile(`versionName=(\S+)`).FindStringSubmatch(dump); m != nil {
		ii.VersionName = m[1]
	}
	for _, l := range strings.Split(shell(d, "pm", "path", pkg), "\n") {
		if p := strings.TrimPrefix(strings.TrimSpace(l), "package:"); p != l && p != "" {
			ii.Paths = append(ii.Paths, p)
		}
	}
	return ii
}
