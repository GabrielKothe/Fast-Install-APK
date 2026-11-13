package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Instalar
// ---------------------------------------------------------------------------

func installOn(c *ctx, d Device, flags ...string) error {
	pkg := c.info.Package
	step("Instalando em " + d.Label() + " ...")
	args := append([]string{"-s", d.Serial, "install", "-r"}, flags...)
	args = append(args, c.apk)
	out, _ := adb(args...)
	indent(out)

	if strings.Contains(out, "INSTALL_FAILED_UPDATE_INCOMPATIBLE") || strings.Contains(out, "INSTALL_FAILED_VERSION_DOWNGRADE") {
		why := "a assinatura é diferente da versão instalada"
		if strings.Contains(out, "DOWNGRADE") {
			why = "a versão instalada é mais nova"
		}
		if confirm(fmt.Sprintf("Não foi possível atualizar %s porque %s.\n\nDesinstalar a versão atual (os dados do app serão apagados) e instalar de novo?", pkg, why)) {
			o, _ := adb("-s", d.Serial, "uninstall", pkg)
			indent(o)
			out, _ = adb(args...)
			indent(out)
		}
	}
	if !successRe.MatchString(out) {
		return fmt.Errorf("falha ao instalar em %s", d.Label())
	}
	ok("Instalado em " + d.Label() + ".")
	launchApp(d, pkg)
	return nil
}

func installMany(c *ctx, mode devMode, flags ...string) error {
	devs, err := selectDevices(mode)
	if err != nil {
		return err
	}
	var failed []string
	for _, d := range devs {
		if err := installOn(c, d, flags...); err != nil {
			fail(err.Error())
			failed = append(failed, d.Label())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("falhou em: %s", strings.Join(failed, ", "))
	}
	c.note = fmt.Sprintf("Instalado %s em %s", c.info.Package, devNames(devs))
	return nil
}

func devNames(devs []Device) string {
	if len(devs) == 1 {
		if devs[0].Model != "" {
			return devs[0].Model
		}
		return devs[0].Serial
	}
	return fmt.Sprintf("%d aparelhos", len(devs))
}

func actInstall(c *ctx) error          { return installMany(c, devAllowAll) }
func actInstallGrant(c *ctx) error     { return installMany(c, devAllowAll, "-g") }
func actInstallDowngrade(c *ctx) error { return installMany(c, devAllowAll, "-d") }
func actInstallAll(c *ctx) error       { return installMany(c, devAll) }

func actReinstall(c *ctx) error {
	devs, err := selectDevices(devAllowAll)
	if err != nil {
		return err
	}
	pkg := c.info.Package
	if !confirm(fmt.Sprintf("Reinstalar limpo?\n\n%s\n\nAparelho: %s\n\nO app será desinstalado (dados apagados) e instalado de novo.", pkg, devNames(devs))) {
		return errCanceled(c)
	}
	for _, d := range devs {
		if isInstalled(d, pkg) {
			step("Desinstalando de " + d.Label() + " ...")
			out, _ := adb("-s", d.Serial, "uninstall", pkg)
			indent(out)
		}
		if err := installOn(c, d); err != nil {
			return err
		}
	}
	c.note = "Reinstalado " + pkg + " em " + devNames(devs)
	return nil
}

func errCanceled(c *ctx) error {
	warn("Cancelado.")
	c.note = "Cancelado"
	return nil
}

// ---------------------------------------------------------------------------
// App instalado
// ---------------------------------------------------------------------------

func actUninstall(c *ctx) error {
	devs, err := selectDevices(devAllowAll)
	if err != nil {
		return err
	}
	pkg := c.info.Package
	if !confirm(fmt.Sprintf("Desinstalar o app abaixo?\n\n%s\n\nAparelho: %s\n\nOs dados do app serão apagados.", pkg, devNames(devs))) {
		return errCanceled(c)
	}
	for _, d := range devs {
		if !isInstalled(d, pkg) {
			warn("    " + pkg + " não está instalado em " + d.Label() + ".")
			continue
		}
		step("Desinstalando de " + d.Label() + " ...")
		out, _ := adb("-s", d.Serial, "uninstall", pkg)
		indent(out)
		if !successRe.MatchString(out) {
			return fmt.Errorf("falha ao desinstalar de %s", d.Label())
		}
		ok("Desinstalado de " + d.Label() + ".")
	}
	c.note = "Desinstalado " + pkg + " de " + devNames(devs)
	return nil
}

func actClear(c *ctx) error {
	devs, err := selectDevices(devAllowAll)
	if err != nil {
		return err
	}
	pkg := c.info.Package
	if !confirm(fmt.Sprintf("Apagar todos os dados do app?\n\n%s\n\nAparelho: %s\n\nLogin, cache e configurações do app serão apagados.", pkg, devNames(devs))) {
		return errCanceled(c)
	}
	for _, d := range devs {
		if !isInstalled(d, pkg) {
			warn("    " + pkg + " não está instalado em " + d.Label() + ".")
			continue
		}
		step("Limpando dados em " + d.Label() + " ...")
		out := shell(d, "pm", "clear", pkg)
		indent(out)
		if !strings.Contains(out, "Success") {
			return fmt.Errorf("falha ao limpar dados em %s", d.Label())
		}
		ok("Dados apagados.")
	}
	c.note = "Dados de " + pkg + " apagados"
	return nil
}

func actRestart(c *ctx) error {
	devs, err := selectDevices(devAllowAll)
	if err != nil {
		return err
	}
	pkg := c.info.Package
	for _, d := range devs {
		if !isInstalled(d, pkg) {
			return fmt.Errorf("%s não está instalado em %s", pkg, d.Label())
		}
		step("Forçando parada em " + d.Label() + " ...")
		shell(d, "am", "force-stop", pkg)
		launchApp(d, pkg)
	}
	c.note = pkg + " reiniciado"
	return nil
}

// ---------------------------------------------------------------------------
// Diagnostico
// ---------------------------------------------------------------------------

func actLogcat(c *ctx) error {
	devs, err := selectDevices(devOne)
	if err != nil {
		return err
	}
	d, pkg := devs[0], c.info.Package
	if !isInstalled(d, pkg) {
		return fmt.Errorf("%s não está instalado em %s", pkg, d.Label())
	}
	pid := pidOf(d, pkg)
	if pid == "" {
		warn("App não está rodando, abrindo...")
		launchApp(d, pkg)
		pid = waitPid(d, pkg, 10*time.Second)
	}
	if pid == "" {
		return errors.New("não foi possível encontrar o processo do app")
	}

	file := outputPath(c, "logcat-"+pkg, d, "txt")
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	ok(fmt.Sprintf("Logcat de %s (PID %s) em %s", pkg, pid, d.Label()))
	fmt.Println(dim("Salvando em: " + file))
	fmt.Println(dim("Pressione Ctrl+C para parar.\n"))

	// Ctrl+C para so o adb, nao este programa
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	cmd := adbCmd("-s", d.Serial, "logcat", "-v", "threadtime", "--pid="+pid)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { <-sig; cmd.Process.Kill() }()

	colors := map[string]string{"V": "90", "D": "36", "I": "32", "W": "33", "E": "31", "F": "35"}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	lines := 0
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		f.WriteString(line + "\n")
		lines++
		if fl := strings.Fields(line); len(fl) >= 5 {
			if col, ok := colors[fl[4]]; ok {
				fmt.Println(color(col, line))
				continue
			}
		}
		fmt.Println(line)
	}
	cmd.Wait()
	fmt.Println()
	ok(fmt.Sprintf("%d linhas salvas em %s", lines, file))
	reveal(file)
	return nil
}

func actInfo(c *ctx) error {
	i := c.info
	fmt.Println(color("1", "Aplicativo"))
	kv("Package", i.Package)
	kv("Nome", i.Label)
	kv("Versão", versionLabel(i.VersionName, i.VersionCode))
	kv("Activity inicial", i.LaunchActivity)
	kv("Debuggable", map[bool]string{true: "sim", false: "não"}[i.Debuggable])

	section("Android")
	kv("SDK mínimo", sdkLabel(i.MinSdk))
	kv("SDK alvo", sdkLabel(i.TargetSdk))
	kv("Compilado com", sdkLabel(i.CompileSdk))
	abis := strings.Join(i.ABIs, ", ")
	if abis == "" {
		abis = "todas (sem código nativo)"
	}
	kv("Arquiteturas", abis)

	section("Arquivo")
	kv("Tamanho", humanSize(i.Size))
	kv("SHA-256", strings.ToLower(i.FileSHA256))

	section("Assinatura")
	if i.Cert == nil {
		warn("  APK sem assinatura (ou formato não reconhecido).")
	} else {
		kv("Esquema", i.Cert.Scheme)
		kv("Certificado", i.Cert.Subject)
		kv("Validade", i.Cert.Expires)
		kv("SHA-1", i.Cert.SHA1)
		kv("SHA-256", i.Cert.SHA256)
	}

	section(fmt.Sprintf("Permissões (%d)", len(i.Permissions)))
	for _, p := range i.Permissions {
		fmt.Println("  " + strings.TrimPrefix(p, "android.permission."))
	}
	return nil
}

var sdkNames = map[int]string{
	21: "5.0", 22: "5.1", 23: "6", 24: "7.0", 25: "7.1", 26: "8.0", 27: "8.1", 28: "9", 29: "10",
	30: "11", 31: "12", 32: "12L", 33: "13", 34: "14", 35: "15", 36: "16", 37: "17",
}

func sdkLabel(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	if v, ok := sdkNames[n]; ok {
		return fmt.Sprintf("%d (Android %s)", n, v)
	}
	return s
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

func actCompare(c *ctx) error {
	devs, err := selectDevices(devAllowAll)
	if err != nil {
		return err
	}
	i := c.info
	for _, d := range devs {
		section(d.Label())
		ii := getInstalled(d, i.Package)
		if ii == nil {
			warn("  Não instalado. Pode instalar normalmente.")
			continue
		}
		var devCert *CertInfo
		if len(ii.Paths) > 0 {
			tmp, err := os.MkdirTemp("", "fastapk")
			if err == nil {
				local := filepath.Join(tmp, "base.apk")
				step("Lendo assinatura do app instalado ...")
				if _, err := adb("-s", d.Serial, "pull", ii.Paths[0], local); err == nil {
					devCert = readCertFile(local)
				}
				os.RemoveAll(tmp)
			}
		}
		fmt.Printf("  %-12s %-28s %s\n", "", "APK", "Instalado")
		fmt.Printf("  %-12s %-28s %s\n", "Versão", i.VersionName, ii.VersionName)
		fmt.Printf("  %-12s %-28s %s\n", "Código", i.VersionCode, ii.VersionCode)
		fmt.Printf("  %-12s %-28s %s\n", "SDK alvo", i.TargetSdk, ii.TargetSdk)
		apkSig, devSig := "-", "-"
		if i.Cert != nil {
			apkSig = i.Cert.SHA256[:23] + "..."
		}
		if devCert != nil {
			devSig = devCert.SHA256[:23] + "..."
		}
		fmt.Printf("  %-12s %-28s %s\n\n", "Assinatura", apkSig, devSig)

		a, _ := strconv.ParseInt(i.VersionCode, 10, 64)
		b, _ := strconv.ParseInt(ii.VersionCode, 10, 64)
		switch {
		case a > b:
			ok("O APK é mais novo que o instalado (atualização).")
		case a == b:
			ok("Mesma versão do instalado.")
		default:
			warn("O APK é MAIS ANTIGO que o instalado: use \"Instalar permitindo downgrade\" ou \"Reinstalar limpo\".")
		}
		switch {
		case i.Cert == nil || devCert == nil:
			warn("Não foi possível comparar as assinaturas.")
		case i.Cert.SHA256 == devCert.SHA256:
			ok("Mesma assinatura: dá para atualizar mantendo os dados.")
		default:
			fail("Assinatura DIFERENTE: a instalação vai falhar. Use \"Reinstalar limpo\" (apaga os dados).")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Arquivos e evidencias
// ---------------------------------------------------------------------------

func actScreenshot(c *ctx) error {
	devs, err := selectDevices(devOne)
	if err != nil {
		return err
	}
	d := devs[0]
	step("Capturando a tela de " + d.Label() + " ...")
	png, err := adbCmd("-s", d.Serial, "exec-out", "screencap", "-p").Output()
	if err != nil || !bytes.HasPrefix(png, []byte("\x89PNG")) {
		return errors.New("falha ao capturar a tela (o aparelho está desbloqueado?)")
	}
	file := outputPath(c, "print", d, "png")
	if err := os.WriteFile(file, png, 0o644); err != nil {
		return err
	}
	ok("Salvo em " + file)
	reveal(file)
	c.note = "Print salvo: " + filepath.Base(file)
	return nil
}

func actRecord(c *ctx) error {
	devs, err := selectDevices(devOne)
	if err != nil {
		return err
	}
	d := devs[0]
	remote := "/sdcard/fastapk-gravacao.mp4"
	cmd := adbCmd("-s", d.Serial, "shell", "screenrecord", "--time-limit", "180", remote)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	ok("Gravando a tela de " + d.Label() + " (máximo 3 minutos).")
	fmt.Println(color("1", "Pressione Enter para parar."))
	enter := make(chan struct{})
	go func() {
		if _, err := stdin.ReadString('\n'); err == nil {
			close(enter)
		} // sem console (EOF): espera Ctrl+C ou o limite de 3 minutos
	}()
	start := time.Now()
	select {
	case <-enter:
	case <-sig:
	case <-done:
	}
	step(fmt.Sprintf("Parando (%ds gravados) ...", int(time.Since(start).Seconds())))
	shell(d, "pkill -INT screenrecord || kill -2 $(pidof screenrecord)")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
	}
	time.Sleep(1500 * time.Millisecond) // o arquivo termina de ser escrito

	file := outputPath(c, "gravacao", d, "mp4")
	out, err := adb("-s", d.Serial, "pull", remote, file)
	shell(d, "rm", "-f", remote)
	if err != nil {
		indent(out)
		return errors.New("falha ao copiar a gravação do aparelho")
	}
	ok("Salvo em " + file)
	reveal(file)
	c.note = "Gravação salva: " + filepath.Base(file)
	return nil
}

func actPull(c *ctx) error {
	devs, err := selectDevices(devOne)
	if err != nil {
		return err
	}
	d, pkg := devs[0], c.info.Package
	ii := getInstalled(d, pkg)
	if ii == nil || len(ii.Paths) == 0 {
		return fmt.Errorf("%s não está instalado em %s", pkg, d.Label())
	}
	base := fmt.Sprintf("%s-v%s-instalado", pkg, ii.VersionName)
	dir := c.outDir
	var target string
	if len(ii.Paths) == 1 {
		target = filepath.Join(dir, base+".apk")
		step("Copiando " + ii.Paths[0] + " ...")
		out, err := adb("-s", d.Serial, "pull", ii.Paths[0], target)
		indent(out)
		if err != nil {
			return errors.New("falha ao copiar o APK")
		}
	} else {
		target = filepath.Join(dir, base)
		os.MkdirAll(target, 0o755)
		warn(fmt.Sprintf("App com %d APKs (split). Copiando todos para a pasta %s", len(ii.Paths), base))
		for _, p := range ii.Paths {
			out, err := adb("-s", d.Serial, "pull", p, filepath.Join(target, filepath.Base(p)))
			indent(out)
			if err != nil {
				return errors.New("falha ao copiar " + p)
			}
		}
	}
	ok("Salvo em " + target)
	reveal(target)
	c.note = "APK copiado: " + filepath.Base(target)
	return nil
}

// ---------------------------------------------------------------------------
// Wi-Fi
// ---------------------------------------------------------------------------

var ipRe = regexp.MustCompile(`inet (\d+\.\d+\.\d+\.\d+)`)

func actWifi(c *ctx) error {
	fmt.Println(color("1", "Conectar via Wi-Fi"))
	fmt.Println("  [1] Ativar Wi-Fi no aparelho conectado por USB")
	fmt.Println("  [2] Parear aparelho novo (Android 11+, código de pareamento)")
	fmt.Println("  [3] Conectar em IP:porta")
	fmt.Println("  [4] Desconectar todos os aparelhos Wi-Fi")
	var target string
	switch prompt("Escolha: ") {
	case "1":
		var usb []Device
		for _, d := range listDevices() {
			if d.State == "device" && !strings.Contains(d.Serial, ":") && !strings.Contains(d.Serial, "._adb-tls") {
				usb = append(usb, d)
			}
		}
		if len(usb) == 0 {
			return errors.New("nenhum aparelho conectado por USB")
		}
		d := usb[0]
		if len(usb) > 1 {
			for i, u := range usb {
				fmt.Printf("  [%d] %s\n", i+1, u.Label())
			}
			n, _ := strconv.Atoi(prompt("Aparelho: "))
			if n < 1 || n > len(usb) {
				return errors.New("opção inválida")
			}
			d = usb[n-1]
		}
		m := ipRe.FindStringSubmatch(shell(d, "ip", "-f", "inet", "addr", "show", "wlan0"))
		if m == nil {
			return errors.New("o aparelho não está conectado em uma rede Wi-Fi")
		}
		step("Ativando adb via Wi-Fi em " + d.Label() + " ...")
		out, _ := adb("-s", d.Serial, "tcpip", "5555")
		indent(out)
		time.Sleep(2 * time.Second)
		target = m[1] + ":5555"
	case "2":
		fmt.Println(dim("No celular: Opções do desenvolvedor > Depuração por Wi-Fi > Parear o dispositivo com um código."))
		hp := prompt("IP:porta de pareamento: ")
		code := prompt("Código de pareamento: ")
		out, _ := adb("pair", hp, code)
		indent(out)
		if !strings.Contains(out, "Successfully paired") {
			return errors.New("falha no pareamento")
		}
		fmt.Println(dim("Agora use o IP:porta mostrado na tela \"Depuração por Wi-Fi\" (não o de pareamento)."))
		target = prompt("IP:porta de conexão: ")
	case "3":
		target = prompt("IP:porta (ex.: 192.168.0.20:5555): ")
		if target != "" && !strings.Contains(target, ":") {
			target += ":5555"
		}
	case "4":
		out, _ := adb("disconnect")
		indent(out)
		ok("Desconectado.")
		return nil
	default:
		return errors.New("opção inválida")
	}
	if target == "" {
		return errors.New("endereço vazio")
	}
	step("Conectando em " + target + " ...")
	out, _ := adb("connect", target)
	indent(out)
	if !strings.Contains(out, "connected to") {
		return errors.New("falha ao conectar")
	}
	ok("Conectado em " + target + ".")
	if c.info == nil {
		return nil
	}
	if a := strings.ToLower(prompt("\nInstalar o APK agora nesse aparelho? [S/n] ")); a == "" || a == "s" || a == "sim" {
		model := strings.TrimSpace(shell(Device{Serial: target}, "getprop", "ro.product.model"))
		return installOn(c, Device{Serial: target, Model: model})
	}
	return nil
}
