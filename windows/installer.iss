; Fast Install APK - instalador Windows (Inno Setup 6)
; Compilar: "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" windows\installer.iss

#define AppName "Fast Install APK"
#ifndef AppVersion
  #define AppVersion "1.0.0"
#endif

[Setup]
AppId={{7E0C5B8A-3F2D-4C71-9A5E-FA57A9C0A1B2}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=Gabriel Kothe
DefaultDirName={localappdata}\FastInstallAPK
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableReadyPage=yes
PrivilegesRequired=lowest
OutputDir=..\dist
OutputBaseFilename=FastInstallAPK-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayName={#AppName}
UninstallDisplayIcon={app}\logo.ico
SetupIconFile=..\build\logo.ico
ChangesAssociations=yes

[Languages]
Name: "ptbr"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"
Name: "en"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "fast-apk.ps1"; DestDir: "{app}"; Flags: ignoreversion
Source: "modern-menu.ps1"; DestDir: "{app}"; Flags: ignoreversion
; Menu moderno do Windows 11 (gerados por build.ps1)
Source: "..\build\FastApkMenu.dll"; DestDir: "{app}"; Flags: ignoreversion restartreplace uninsrestartdelete
Source: "..\build\FastInstallAPK.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\FastInstallAPK.msix"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\install.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\uninstall.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\logo.ico"; DestDir: "{app}"; Flags: ignoreversion
; Pacote sparse le os logos (icone do submenu "Fast Install APK") da pasta externa
Source: "..\build\Assets\*"; DestDir: "{app}\Assets"; Flags: ignoreversion

[Run]
Filename: "{win}\System32\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\modern-menu.ps1"" register ""{app}"""; Flags: runhidden waituntilterminated; StatusMsg: "Registrando o menu do Windows 11..."

[UninstallRun]
Filename: "{win}\System32\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\modern-menu.ps1"" unregister ""{app}"""; Flags: runhidden waituntilterminated; RunOnceId: "UnregisterModernMenu"

[Registry]
Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.1Install"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.1Install"; ValueType: string; ValueName: "MUIVerb"; ValueData: "Instalar via adb"
Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.1Install"; ValueType: string; ValueName: "Icon"; ValueData: "{win}\System32\imageres.dll,-1028"
Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.1Install\command"; ValueType: string; ValueName: ""; ValueData: """{win}\System32\WindowsPowerShell\v1.0\powershell.exe"" -NoProfile -ExecutionPolicy Bypass -File ""{app}\fast-apk.ps1"" install ""%1"""

Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.2Uninstall"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.2Uninstall"; ValueType: string; ValueName: "MUIVerb"; ValueData: "Desinstalar via adb"
Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.2Uninstall"; ValueType: string; ValueName: "Icon"; ValueData: "{win}\System32\imageres.dll,-5305"
Root: HKCU; Subkey: "Software\Classes\SystemFileAssociations\.apk\shell\FastApk.2Uninstall\command"; ValueType: string; ValueName: ""; ValueData: """{win}\System32\WindowsPowerShell\v1.0\powershell.exe"" -NoProfile -ExecutionPolicy Bypass -File ""{app}\fast-apk.ps1"" uninstall ""%1"""

[Messages]
ptbr.FinishedLabel=O {#AppName} foi instalado.%n%nClique com o botao direito em um arquivo .apk e escolha "Instalar via adb" ou "Desinstalar via adb".%n%nSe as opcoes nao aparecerem de imediato, feche e abra o Explorador de Arquivos.

[Code]
function AdbFound(): Boolean;
var
  Paths: TArrayOfString;
  I: Integer;
  PathEnv: String;
begin
  Result := False;
  SetArrayLength(Paths, 4);
  Paths[0] := GetEnv('ANDROID_HOME') + '\platform-tools\adb.exe';
  Paths[1] := GetEnv('ANDROID_SDK_ROOT') + '\platform-tools\adb.exe';
  Paths[2] := ExpandConstant('{localappdata}') + '\Android\Sdk\platform-tools\adb.exe';
  Paths[3] := 'D:\AndroidSDK\platform-tools\adb.exe';
  for I := 0 to GetArrayLength(Paths) - 1 do
    if FileExists(Paths[I]) then begin Result := True; Exit; end;
  PathEnv := GetEnv('PATH');
  while PathEnv <> '' do
  begin
    I := Pos(';', PathEnv);
    if I = 0 then I := Length(PathEnv) + 1;
    if FileExists(AddBackslash(Copy(PathEnv, 1, I - 1)) + 'adb.exe') then begin Result := True; Exit; end;
    Delete(PathEnv, 1, I);
  end;
end;

// Antes de copiar os arquivos: remove o pacote antigo e libera a DLL (atualizacao)
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Code: Integer;
begin
  Result := '';
  ExtractTemporaryFile('modern-menu.ps1');
  Exec(ExpandConstant('{win}\System32\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -ExecutionPolicy Bypass -File "' + ExpandConstant('{tmp}\modern-menu.ps1') + '" unregister',
    '', SW_HIDE, ewWaitUntilTerminated, Code);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssDone) and (not WizardSilent()) and (not AdbFound()) then
    MsgBox('Aviso: o adb nao foi encontrado neste computador.' + #13#10#13#10 +
      'Instale o Android SDK Platform-Tools e adicione a pasta platform-tools ao PATH (ou defina ANDROID_HOME).',
      mbInformation, MB_OK);
end;
