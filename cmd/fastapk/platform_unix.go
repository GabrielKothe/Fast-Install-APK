//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func platformInit()            {}
func setTitle(t string)        { fmt.Printf("\x1b]0;%s\x07", t) }
func hideWindow(c *exec.Cmd)   {}
func registerClassic() error   { return errors.New("so no Windows") }
func unregisterClassic() error { return errors.New("so no Windows") }

func osaString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func osascript(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).Output()
	return strings.TrimSpace(string(out)), err
}

func confirm(text string) bool {
	if autoYes() {
		return true
	}
	if runtime.GOOS != "darwin" {
		a := strings.ToLower(prompt(text + "\n[s/N] "))
		return a == "s" || a == "sim"
	}
	out, _ := osascript(fmt.Sprintf(`display dialog %s with title %s buttons {"Cancelar", "Sim"} default button "Cancelar" cancel button "Cancelar" with icon caution`,
		osaString(text), osaString(appName)))
	return strings.Contains(out, "Sim")
}

func notify(title, msg string) {
	if runtime.GOOS == "darwin" {
		exec.Command("osascript", "-e", fmt.Sprintf("display notification %s with title %s", osaString(msg), osaString(title))).Start()
	}
}

func reveal(path string) {
	if runtime.GOOS == "darwin" {
		exec.Command("open", "-R", path).Start()
	} else {
		exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Chamado pela Acao Rapida do Finder: "menu" mostra a lista de opcoes; depois abre o Terminal com a acao
func platformLaunch(id, apk string) error {
	if id == "menu" {
		var titles []string
		for _, a := range actions {
			titles = append(titles, osaString(a.Title))
		}
		out, err := osascript(fmt.Sprintf(`choose from list {%s} with title %s with prompt %s default items {%s}`,
			strings.Join(titles, ", "), osaString(appName), osaString(filepath.Base(apk)), titles[0]))
		if err != nil || out == "" || out == "false" {
			return nil
		}
		id = ""
		for _, a := range actions {
			if a.Title == out {
				id = a.ID
			}
		}
		if id == "" {
			return nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "fastapk-*.command")
	if err != nil {
		return err
	}
	fmt.Fprintf(f, "#!/bin/bash\nrm -f \"$0\"\nclear\n%s %s %s\n", shQuote(exe), shQuote(id), shQuote(apk))
	f.Close()
	os.Chmod(f.Name(), 0o755)
	return exec.Command("open", "-a", "Terminal", f.Name()).Run()
}
