# Fast Install APK

Adiciona ao menu de botão direito de arquivos `.apk`:

- **Instalar via adb**: instala o APK (`adb install -r`) e abre o app no aparelho.
- **Desinstalar via adb**: lê o *package name* do APK e desinstala (`adb uninstall`), **pedindo confirmação antes**.

Funciona no **Windows** (inclusive no menu novo do Windows 11) e no **macOS**.
Não precisa do `aapt`: o package name é lido direto do `AndroidManifest.xml` do APK, mesmo com acentos no caminho.

## Requisitos

- `adb` (Android SDK Platform-Tools). O programa procura no `PATH`, em `ANDROID_HOME` / `ANDROID_SDK_ROOT` e nos locais padrão do Android Studio.
  - macOS: `brew install --cask android-platform-tools`
- Depuração USB ativada e autorizada no aparelho.

## Windows

Rode `FastInstallAPK-Setup-<versão>.exe` (não precisa de administrador). Para remover, vá em *Configurações > Aplicativos > Fast Install APK > Desinstalar*.

- **Windows 11**: as opções aparecem direto no menu novo (como o Notepad++ e o PowerRename). Se não aparecerem logo, feche e abra o Explorador de Arquivos.
- **Windows 10** ou "Mostrar mais opções": as opções também ficam no menu clássico.

Gerar o instalador (precisa de MinGW-w64, Windows SDK e Inno Setup 6):

```bash
powershell -ExecutionPolicy Bypass -File windows/build.ps1 -Version 1.1.0
```

### Como funciona o menu do Windows 11

O menu novo do Windows 11 só aceita extensões vindas de um app com identidade de pacote. Por isso o instalador:

1. instala `FastApkMenu.dll` (um `IExplorerCommand` em C++, em `windows/shellext/`), e
2. registra um pacote MSIX *sparse* (`FastInstallAPK.msix`) que aponta para a pasta de instalação.

O pacote é **sem assinatura** (publisher com o OID de pacote não assinado + `Add-AppxPackage -AllowUnsigned`), então não precisa de certificado nem de administrador.

## macOS

Duas opções:

- **`.pkg`**: `FastInstallAPK-<versão>.pkg` (gerado no Mac com `bash macos/build-pkg.sh 1.1.0` ou pelo GitHub Actions).
- **`.zip`**: descompacte `FastInstallAPK-mac-<versão>.zip` e dê dois cliques em `Instalar.command`.

Como os instaladores não são assinados pela Apple, na primeira vez clique com o **botão direito > Abrir**.

As opções aparecem em **botão direito > Ações Rápidas** no Finder.
Se não aparecerem, ative-as em *Ajustes do Sistema > Privacidade e Segurança > Extensões > Finder*.
Se mesmo assim não aparecerem em `.apk`, rode `bash install.sh --all-files`: o menu passa a aparecer em qualquer arquivo, mas só age em `.apk`.

Remover: `Desinstalar.command` (ou `bash "/Library/Application Support/FastInstallAPK/installer/uninstall.sh"` se instalou pelo `.pkg`).

## Build automático

O workflow `.github/workflows/build.yml` gera o `setup.exe`, o `.pkg` e o `.zip` do Mac. Ao criar uma tag `v*` (ex.: `v1.1.0`), ele publica tudo numa Release do GitHub.

## Como funciona

- **Instalar**: abre uma janela de terminal, roda `adb install -r <apk>` e depois
  `adb shell monkey -p <package> -c android.intent.category.LAUNCHER 1` para abrir o app.
  Se a instalação falhar por assinatura diferente ou downgrade, pergunta se você quer desinstalar a versão atual e instalar de novo.
- **Desinstalar**: mostra uma caixa de confirmação (o padrão é Não/Cancelar) e roda `adb uninstall <package>`.
- **Vários aparelhos conectados**: pergunta em qual instalar/desinstalar (ou em todos).
- Se der certo, a janela fecha sozinha em 3 segundos. Se der erro, fica aberta até você apertar Enter.

Para só ver o package name de um APK:

```bash
powershell -ExecutionPolicy Bypass -File windows/fast-apk.ps1 package app.apk   # Windows
bash macos/fast-apk.sh package app.apk                                          # macOS
```
