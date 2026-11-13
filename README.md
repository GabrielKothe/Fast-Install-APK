# Fast Install APK

Clique com o botão direito em um `.apk` e use o submenu **Fast Install APK**:

| Grupo | Opção | O que faz |
|---|---|---|
| Instalar | **Instalar via adb** | `adb install -r` e abre o app |
| | Instalar concedendo permissões | `install -g`: já libera câmera, localização etc. |
| | Instalar permitindo downgrade | `install -d`: instala versão mais antiga (APK debug) |
| | Instalar em todos os aparelhos | instala em todos os conectados, sem perguntar |
| | Reinstalar limpo | desinstala, instala e abre (pede confirmação) |
| App | Forçar parada e abrir | `am force-stop` e abre de novo |
| | Limpar dados do app | `pm clear` (pede confirmação) |
| | **Desinstalar via adb** | `adb uninstall` (pede confirmação) |
| Diagnóstico | Logcat do app | log só do processo do app, colorido e salvo em `.txt` ao lado do APK |
| | Informações do APK | package, versão, SDK mín./alvo, arquiteturas, permissões e assinatura (SHA-1/SHA-256). Não precisa de aparelho |
| | Comparar com o instalado | versão e assinatura do APK x app instalado: avisa antes do erro de "assinatura diferente" |
| | Puxar APK instalado | copia o APK que está no aparelho (inclusive split APKs) |
| Tela ✱ | Espelhar a tela | mostra e controla a tela do aparelho no PC com o [scrcpy](https://github.com/Genymobile/scrcpy) (se não estiver instalado, oferece instalar pelo `winget`/`brew`) |
| ✱ | Print da tela | salva `.png` na pasta |
| ✱ | Gravar a tela | grava até apertar Enter (máx. 3 min) e salva `.mp4` na pasta |
| ✱ | Conectar via Wi-Fi | ativa adb por Wi-Fi a partir do USB, pareia (Android 11+) ou conecta em IP:porta |

✱ **Não precisam de APK**: além do `.apk`, aparecem no botão direito do **fundo de qualquer pasta** e da **Área de Trabalho**, e salvam os arquivos nessa pasta.

Também:

- **Vários aparelhos**: mostra o modelo (ex.: "L400 (4AF02KF0B)") e lembra o último escolhido (Enter = último).
- **Notificação** do sistema ao terminar.
- Se der certo, a janela fecha sozinha em 3 s. Se der erro, fica aberta até apertar Enter.
- Se a instalação falhar por assinatura diferente ou downgrade, pergunta se quer desinstalar e instalar de novo.
- O APK é lido direto (manifesto binário + bloco de assinatura v1/v2/v3), **sem precisar do `aapt`**, e funciona com acentos no caminho.

## Requisitos

- `adb` (Android SDK Platform-Tools). O programa procura no `PATH`, em `ANDROID_HOME` / `ANDROID_SDK_ROOT` e nos locais padrão do Android Studio.
- Depuração USB ativada e autorizada no aparelho.

## Windows

Rode `FastInstallAPK-Setup-<versão>.exe` (não precisa de administrador). Para remover: *Configurações > Aplicativos > Fast Install APK*.

- **Windows 11**: o submenu aparece direto no menu novo, com um ícone por opção. Se não aparecer logo, feche e abra o Explorador de Arquivos. O menu novo do Windows 11 não mostra separadores nos submenus, por isso as cores dos ícones indicam os grupos.
- **Windows 10** ou "Mostrar mais opções": o mesmo submenu também fica no menu clássico.

## macOS

- **`.pkg`**: `FastInstallAPK-<versão>.pkg`
- **`.zip`**: descompacte e dê dois cliques em `Instalar.command`

Como não são assinados pela Apple, na primeira vez use **botão direito > Abrir**.
No Finder: **botão direito > Ações Rápidas > "Fast Install APK…"** abre a lista com todas as opções. "Instalar via adb" e "Desinstalar via adb" também ficam como atalhos diretos.
Se não aparecer, ative em *Ajustes do Sistema > Privacidade e Segurança > Extensões > Finder*.

## Linha de comando

```bash
fastapk <ação> <arquivo.apk>      # ex.: fastapk info app.apk
fastapk                           # lista as ações
```

`FASTAPK_YES=1` responde "Sim" em todas as confirmações (para automação).

## Estrutura

- `cmd/fastapk/`: o programa (Go, um código só para Windows e macOS)
- `windows/shellext/`: extensão do menu do Windows 11 (`IExplorerCommand` em C++). Lê os itens de `menu.txt`, que é gerado por `fastapk menu`
- `windows/installer.iss`: instalador (Inno Setup). Registra o pacote MSIX *sparse* sem assinatura (sem certificado nem administrador) e o menu clássico
- `macos/`: Ações Rápidas do Finder, `.pkg` e `.zip`

## Build

Windows (precisa de Go, MinGW-w64, Windows SDK e Inno Setup 6):

```bash
powershell -ExecutionPolicy Bypass -File windows/build.ps1 -Version 2.0.0
```

O workflow `.github/workflows/build.yml` gera o `setup.exe`, o `.pkg` e o `.zip` do Mac. Ao criar uma tag `v*`, ele publica tudo numa Release do GitHub.
