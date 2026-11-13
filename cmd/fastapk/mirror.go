package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Espelhar a tela com o scrcpy (https://github.com/Genymobile/scrcpy)

func findScrcpy() string {
	exe := "scrcpy"
	if runtime.GOOS == "windows" {
		exe = "scrcpy.exe"
	}
	if p, err := exec.LookPath(exe); err == nil {
		return p
	}
	var cands []string
	switch runtime.GOOS {
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		cands = append(cands, filepath.Join(local, "Microsoft", "WinGet", "Links", exe))
		// pasta do pacote do winget (quando o link nao foi criado)
		if m, _ := filepath.Glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", "Genymobile.scrcpy*", "*", exe)); len(m) > 0 {
			cands = append(cands, m...)
		}
		cands = append(cands, `C:\scrcpy\`+exe, filepath.Join(os.Getenv("ProgramFiles"), "scrcpy", exe))
	case "darwin":
		cands = append(cands, "/opt/homebrew/bin/scrcpy", "/usr/local/bin/scrcpy")
	default:
		cands = append(cands, "/usr/bin/scrcpy", "/usr/local/bin/scrcpy", "/snap/bin/scrcpy")
	}
	for _, p := range cands {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func installScrcpy() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if !confirm("O scrcpy (programa gratuito para espelhar a tela do Android) não está instalado.\n\nInstalar agora pelo winget?") {
			return errors.New("scrcpy não instalado. Instale com: winget install Genymobile.scrcpy")
		}
		cmd = exec.Command("winget", "install", "--id", "Genymobile.scrcpy", "-e")
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return errors.New("scrcpy não instalado. Instale o Homebrew e rode: brew install scrcpy")
		}
		if !confirm("O scrcpy (programa gratuito para espelhar a tela do Android) não está instalado.\n\nInstalar agora pelo Homebrew?") {
			return errors.New("scrcpy não instalado. Instale com: brew install scrcpy")
		}
		cmd = exec.Command("brew", "install", "scrcpy")
	default:
		return errors.New("scrcpy não instalado. Instale pelo gerenciador de pacotes (ex.: apt install scrcpy)")
	}
	step("Instalando o scrcpy ...")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("falha ao instalar o scrcpy: %v", err)
	}
	return nil
}

func actMirror(c *ctx) error {
	devs, err := selectDevices(devOne)
	if err != nil {
		return err
	}
	d := devs[0]
	scrcpy := findScrcpy()
	if scrcpy == "" {
		if err := installScrcpy(); err != nil {
			return err
		}
		if scrcpy = findScrcpy(); scrcpy == "" {
			return errors.New("scrcpy instalado, mas não encontrado. Feche esta janela e tente de novo")
		}
	}
	title := d.Label()
	step("Abrindo o espelhamento de " + title + " ...")
	cmd := exec.Command(scrcpy, "-s", d.Serial, "--window-title", title, "--stay-awake")
	// usa o mesmo adb deste programa (evita conflito de versao do servidor adb)
	cmd.Env = append(os.Environ(), "ADB="+adbPath)
	cmd.Dir = filepath.Dir(scrcpy)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao abrir o scrcpy: %v", err)
	}
	ok("Espelhamento aberto. Feche a janela do scrcpy para encerrar.")
	c.note = "Espelhando " + devNames(devs)
	return nil
}
