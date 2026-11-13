# Compila a extensao do menu do Windows 11, gera o pacote MSIX e o instalador dist\FastInstallAPK-Setup-<versao>.exe
# Requisitos: MinGW-w64 (g++/gcc no PATH), Windows SDK (makeappx.exe) e Inno Setup 6
param([string]$Version = '1.0.0')
$ErrorActionPreference = 'Stop'

$here = $PSScriptRoot
$root = Split-Path $here -Parent
$build = Join-Path $root 'build'
$pkg = Join-Path $build 'pkg'
Remove-Item $build -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $pkg | Out-Null

Write-Host '==> Compilando FastApkMenu.dll'
& g++ -O2 -shared -static -static-libgcc -static-libstdc++ -municode -o "$build\FastApkMenu.dll" `
    "$here\shellext\FastApkMenu.cpp" "$here\shellext\FastApkMenu.def" -lole32 -luuid -lshlwapi
if ($LASTEXITCODE) { throw 'Falha ao compilar a DLL' }
& gcc -O2 -mwindows -o "$build\FastInstallAPK.exe" "$here\shellext\stub.c"
if ($LASTEXITCODE) { throw 'Falha ao compilar o stub' }

Write-Host '==> Gerando icones'
& "$here\shellext\make-icons.ps1" -OutDir $build

Write-Host '==> Gerando pacote MSIX'
Copy-Item "$build\Assets" $pkg -Recurse
# Publisher com este OID permite instalar o pacote sem assinatura (Add-AppxPackage -AllowUnsigned)
$publisher = 'CN=FastInstallAPK, OID.2.25.311729368913984317654407730594956997722=1'
$manifest = (Get-Content "$here\shellext\AppxManifest.xml" -Raw).Replace('__PUBLISHER__', $publisher).Replace('__VERSION__', "$Version.0")
[System.IO.File]::WriteAllText("$pkg\AppxManifest.xml", $manifest, (New-Object System.Text.UTF8Encoding $false))

$makeappx = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\makeappx.exe" | Sort-Object FullName | Select-Object -Last 1
if (-not $makeappx) { throw 'makeappx.exe nao encontrado (instale o Windows SDK)' }
& $makeappx.FullName pack /o /nv /d $pkg /p "$build\FastInstallAPK.msix" | Out-Null
if ($LASTEXITCODE) { throw 'Falha no makeappx' }

Write-Host '==> Compilando instalador'
$iscc = @("${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe", "$env:ProgramFiles\Inno Setup 6\ISCC.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $iscc) { throw 'Inno Setup 6 nao encontrado' }
& $iscc /Q "/DAppVersion=$Version" "$here\installer.iss"
if ($LASTEXITCODE) { throw 'Falha no ISCC' }
Write-Host "OK: $(Join-Path $root "dist\FastInstallAPK-Setup-$Version.exe")" -ForegroundColor Green
