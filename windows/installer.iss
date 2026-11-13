; Fast Install APK - instalador Windows (Inno Setup 6)
; Gerar com: powershell -File windows\build.ps1 -Version x.y.z

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
Source: "..\build\fastapk.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\menu.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "modern-menu.ps1"; DestDir: "{app}"; Flags: ignoreversion
; Menu moderno do Windows 11 (gerados por build.ps1)
Source: "..\build\FastApkMenu.dll"; DestDir: "{app}"; Flags: ignoreversion restartreplace uninsrestartdelete
Source: "..\build\FastInstallAPK.msix"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\install.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\uninstall.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\logo.ico"; DestDir: "{app}"; Flags: ignoreversion
; O pacote sparse le os logos da pasta externa
Source: "..\build\Assets\*"; DestDir: "{app}\Assets"; Flags: ignoreversion

[InstallDelete]
; arquivos da versao 1.x
Type: files; Name: "{app}\fast-apk.ps1"
Type: files; Name: "{app}\FastInstallAPK.exe"

[Run]
Filename: "{app}\fastapk.exe"; Parameters: "register-classic"; Flags: runhidden waituntilterminated; StatusMsg: "Registrando o menu..."
Filename: "{win}\System32\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\modern-menu.ps1"" register ""{app}"""; Flags: runhidden waituntilterminated; StatusMsg: "Registrando o menu do Windows 11..."

[UninstallRun]
Filename: "{app}\fastapk.exe"; Parameters: "unregister-classic"; Flags: runhidden waituntilterminated; RunOnceId: "UnregisterClassicMenu"
Filename: "{win}\System32\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\modern-menu.ps1"" unregister ""{app}"""; Flags: runhidden waituntilterminated; RunOnceId: "UnregisterModernMenu"

[UninstallDelete]
Type: filesandordirs; Name: "{userappdata}\FastInstallAPK"

[Messages]
ptbr.FinishedLabel=O {#AppName} foi instalado.%n%nClique com o botao direito em um arquivo .apk e abra o submenu "Fast Install APK".%n%nSe nao aparecer de imediato, feche e abra o Explorador de Arquivos.

[Code]
function AdbFound(): Boolean;
var
  Paths: TArrayOfString;
  I: Integer;
  PathEnv: String;
begin
  Result := False;
  SetArrayLength(Paths, 3);
  Paths[0] := GetEnv('ANDROID_HOME') + '\platform-tools\adb.exe';
  Paths[1] := GetEnv('ANDROID_SDK_ROOT') + '\platform-tools\adb.exe';
  Paths[2] := ExpandConstant('{localappdata}') + '\Android\Sdk\platform-tools\adb.exe';
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
