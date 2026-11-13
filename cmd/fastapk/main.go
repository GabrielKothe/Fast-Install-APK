// Fast Install APK - instala/desinstala e mais, via adb, a partir do menu de contexto de arquivos .apk
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var version = "dev"

const appName = "Fast Install APK"

// ---------------------------------------------------------------------------
// Console
// ---------------------------------------------------------------------------

func color(code, s string) string { return "\x1b[" + code + "m" + s + "\x1b[0m" }
func dim(s string) string          { return color("90", s) }
func step(s string)                { fmt.Println(color("36", "==> "+s)) }
func ok(s string)                  { fmt.Println(color("32", "OK  "+s)) }
func warn(s string)                { fmt.Println(color("33", s)) }
func fail(s string)                { fmt.Println(color("31", "ERRO "+s)) }
func indent(s string) {
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			fmt.Println("    " + l)
		}
	}
}
func kv(k, v string) {
	if v == "" {
		v = dim("-")
	}
	fmt.Printf("  %-18s %s\n", k+":", v)
}
func section(s string) { fmt.Println("\n" + color("1", s)) }

// ---------------------------------------------------------------------------
// Acoes
// ---------------------------------------------------------------------------

type ctx struct {
	apk  string
	info *ApkInfo
	note string // texto da notificacao final
}

type action struct {
	ID     string
	Title  string
	Icon   string // icone do menu no Windows (arquivo .ico na pasta do app)
	Sep    bool   // separador antes deste item
	Single bool   // com varios arquivos selecionados, roda so para o primeiro
	Device bool   // precisa de adb/aparelho
	Wait   bool   // sempre espera Enter no final (mostra informacao)
	Full   bool   // le hash/assinatura do APK
	Run    func(c *ctx) error
}

var actions []action

func init() {
	actions = []action{
		{ID: "install", Title: "Instalar via adb", Icon: "install.ico", Device: true, Run: actInstall},
		{ID: "install-grant", Title: "Instalar concedendo permissões", Icon: "install.ico", Device: true, Run: actInstallGrant},
		{ID: "install-downgrade", Title: "Instalar permitindo downgrade", Icon: "install.ico", Device: true, Run: actInstallDowngrade},
		{ID: "install-all", Title: "Instalar em todos os aparelhos", Icon: "install.ico", Device: true, Run: actInstallAll},
		{ID: "reinstall", Title: "Reinstalar limpo", Icon: "install.ico", Device: true, Run: actReinstall},

		{ID: "restart", Title: "Forçar parada e abrir", Sep: true, Device: true, Run: actRestart},
		{ID: "clear", Title: "Limpar dados do app", Device: true, Run: actClear},
		{ID: "uninstall", Title: "Desinstalar via adb", Icon: "uninstall.ico", Device: true, Run: actUninstall},

		{ID: "logcat", Title: "Logcat do app", Sep: true, Single: true, Device: true, Wait: true, Run: actLogcat},
		{ID: "info", Title: "Informações do APK", Wait: true, Full: true, Run: actInfo},
		{ID: "compare", Title: "Comparar com o instalado", Device: true, Wait: true, Full: true, Run: actCompare},

		{ID: "screenshot", Title: "Print da tela", Sep: true, Single: true, Device: true, Run: actScreenshot},
		{ID: "record", Title: "Gravar a tela", Single: true, Device: true, Run: actRecord},
		{ID: "pull", Title: "Puxar APK instalado", Single: true, Device: true, Run: actPull},

		{ID: "wifi", Title: "Conectar via Wi-Fi", Sep: true, Single: true, Device: true, Wait: true, Run: actWifi},
	}
}

func findAction(id string) *action {
	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------

func usage() {
	fmt.Printf("%s %s\n\nUso: fastapk <acao> <arquivo.apk>\n\nAcoes:\n", appName, version)
	for _, a := range actions {
		fmt.Printf("  %-18s %s\n", a.ID, a.Title)
	}
	fmt.Println("\nOutros: menu | register-classic | unregister-classic | launch <acao> <apk> | version")
	os.Exit(2)
}

func main() {
	platformInit()
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version)
		return
	case "menu":
		// usado pela DLL do Windows 11: id|titulo|icone|flags (s = separador antes, 1 = so o primeiro arquivo)
		for _, a := range actions {
			flags := ""
			if a.Sep {
				flags += "s"
			}
			if a.Single {
				flags += "1"
			}
			fmt.Printf("%s|%s|%s|%s\n", a.ID, a.Title, a.Icon, flags)
		}
		return
	case "register-classic":
		exitOn(registerClassic())
		return
	case "unregister-classic":
		exitOn(unregisterClassic())
		return
	case "launch":
		if len(os.Args) < 4 {
			usage()
		}
		exitOn(platformLaunch(os.Args[2], os.Args[3]))
		return
	}

	a := findAction(os.Args[1])
	if a == nil || len(os.Args) < 3 {
		usage()
	}
	apk, err := filepath.Abs(os.Args[2])
	if err != nil {
		apk = os.Args[2]
	}
	setTitle(appName + " - " + a.Title)
	fmt.Println(color("1", appName+" - "+a.Title))
	fmt.Println(dim(apk))

	c := &ctx{apk: apk}
	run := func() error {
		info, err := readApk(apk, a.Full)
		if err != nil {
			return err
		}
		c.info = info
		fmt.Printf("Package: %s  %s\n\n", color("1", info.Package), dim(versionLabel(info.VersionName, info.VersionCode)))
		if a.Device {
			if adbPath, err = findAdb(); err != nil {
				return err
			}
		}
		return a.Run(c)
	}
	err = run()
	finish(a, c, err)
}

func versionLabel(name, code string) string {
	switch {
	case name != "" && code != "":
		return fmt.Sprintf("v%s (%s)", name, code)
	case name != "":
		return "v" + name
	case code != "":
		return "(" + code + ")"
	}
	return ""
}

func exitOn(err error) {
	if err != nil {
		fail(err.Error())
		os.Exit(1)
	}
}

func pause() { prompt("\nPressione Enter para fechar ") }

// FASTAPK_YES=1 responde "Sim" para todas as confirmacoes (automacao)
func autoYes() bool { return os.Getenv("FASTAPK_YES") == "1" }

func finish(a *action, c *ctx, err error) {
	pkg := ""
	if c.info != nil {
		pkg = c.info.Package
	}
	if err != nil {
		fmt.Println()
		fail(err.Error())
		notify(appName+": erro", a.Title+" - "+firstLine(err.Error()))
		pause()
		os.Exit(1)
	}
	if c.note != "" {
		notify(appName, c.note)
	} else if !a.Wait {
		notify(appName, a.Title+": concluído ("+pkg+")")
	}
	if a.Wait {
		pause()
		return
	}
	fmt.Println(dim("\nFechando em 3 segundos..."))
	time.Sleep(3 * time.Second)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Arquivo de saida ao lado do APK: <prefixo>-<modelo>-<data>.<ext>
func outputPath(c *ctx, prefix string, d Device, ext string) string {
	name := d.Model
	if name == "" {
		name = d.Serial
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>| `, r) {
			return '_'
		}
		return r
	}, name)
	return filepath.Join(filepath.Dir(c.apk), fmt.Sprintf("%s-%s-%s.%s", prefix, name, time.Now().Format("20060102-150405"), ext))
}
