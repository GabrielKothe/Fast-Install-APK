//go:build windows

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSetConsoleOutputCP  = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP        = kernel32.NewProc("SetConsoleCP")
	procGetConsoleMode      = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode      = kernel32.NewProc("SetConsoleMode")
	procSetConsoleTitleW    = kernel32.NewProc("SetConsoleTitleW")
	procGetConsoleWindow    = kernel32.NewProc("GetConsoleWindow")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

func platformInit() {
	procSetConsoleOutputCP.Call(65001)
	procSetConsoleCP.Call(65001)
	// cores ANSI (ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	h, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); r != 0 {
		procSetConsoleMode.Call(uintptr(h), uintptr(mode|0x0004))
	}
}

func setTitle(t string) {
	p, _ := syscall.UTF16PtrFromString(t)
	procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(p)))
}

func hideWindow(c *exec.Cmd) {}

// detach: o processo continua aberto depois que esta janela fechar
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200} // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
}

func confirm(text string) bool {
	const (
		mbYesNo         = 0x4
		mbIconWarning   = 0x30
		mbDefButton2    = 0x100
		mbSetForeground = 0x10000
		mbTopmost       = 0x40000
		idYes           = 6
	)
	if autoYes() {
		fmt.Println(dim("(FASTAPK_YES) " + strings.ReplaceAll(text, "\n", " ")))
		return true
	}
	hwnd, _, _ := procGetConsoleWindow.Call()
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(appName)
	r, _, _ := procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)),
		mbYesNo|mbIconWarning|mbDefButton2|mbSetForeground|mbTopmost)
	if hwnd != 0 {
		procSetForegroundWindow.Call(hwnd)
	}
	return r == idYes
}

// Notificacao (toast) do Windows via PowerShell, sem janela e sem esperar
func notify(title, msg string) {
	esc := func(s string) string { return strings.ReplaceAll(html.EscapeString(s), "'", "''") }
	script := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null
$x = New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text>` + esc(title) + `</text><text>` + esc(msg) + `</text></binding></visual></toast>')
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show([Windows.UI.Notifications.ToastNotification]::new($x))`
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[i*2], b[i*2+1] = byte(v), byte(v>>8)
	}
	cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), `System32\WindowsPowerShell\v1.0\powershell.exe`),
		"-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	cmd.Start()
}

func reveal(path string) {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	cmd.Start()
}

func platformLaunch(action, apk string) error {
	return errors.New("launch so e usado no macOS")
}

// ---------------------------------------------------------------------------
// Menu classico (Windows 10 e "Mostrar mais opcoes" do Windows 11)
// ---------------------------------------------------------------------------

// onde o menu "Fast Install APK" aparece: arquivos .apk (todas as opcoes) e
// fundo de pasta / Area de Trabalho (so as opcoes que funcionam sem APK)
var classicMenus = []struct {
	sub   string // relativo a HKCU\Software\Classes
	noApk bool
	arg   string
}{
	{`SystemFileAssociations\.apk\shell\FastInstallAPK`, false, "%1"},
	{`Directory\Background\shell\FastInstallAPK`, true, "%V"},
	{`DesktopBackground\shell\FastInstallAPK`, true, "%V"},
}

const classesRoot = `HKCU\Software\Classes\`

func reg(args ...string) error {
	cmd := exec.Command("reg.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("reg %s: %v %s", args[0], err, out)
	}
	return nil
}

func registerClassic() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	unregisterClassic()

	for _, m := range classicMenus {
		key := classesRoot + m.sub
		if err := reg("add", key, "/v", "MUIVerb", "/d", appName, "/f"); err != nil {
			return err
		}
		reg("add", key, "/v", "Icon", "/d", filepath.Join(dir, "logo.ico"), "/f")
		reg("add", key, "/v", "ExtendedSubCommandsKey", "/d", m.sub, "/f")
		n := 0
		for i, a := range actions {
			if m.noApk && !a.NoApk {
				continue
			}
			k := fmt.Sprintf(`%s\shell\%02d-%s`, key, i, a.ID)
			reg("add", k, "/v", "MUIVerb", "/d", a.Title, "/f")
			reg("add", k, "/v", "Icon", "/d", filepath.Join(dir, a.ID+".ico"), "/f")
			if a.Sep && n > 0 {
				reg("add", k, "/v", "CommandFlags", "/t", "REG_DWORD", "/d", "32", "/f") // ECF_SEPARATORBEFORE
			}
			if a.Single {
				reg("add", k, "/v", "MultiSelectModel", "/d", "Single", "/f")
			}
			if err := reg("add", k+`\command`, "/ve", "/d", fmt.Sprintf(`"%s" %s "%s"`, exe, a.ID, m.arg), "/f"); err != nil {
				return err
			}
			n++
		}
	}
	fmt.Println("Menu classico registrado.")
	return nil
}

func unregisterClassic() error {
	for _, m := range classicMenus {
		reg("delete", classesRoot+m.sub, "/f")
	}
	// entradas da versao 1.x
	for _, k := range []string{"FastApk.1Install", "FastApk.2Uninstall"} {
		reg("delete", classesRoot+`SystemFileAssociations\.apk\shell\`+k, "/f")
	}
	return nil
}
