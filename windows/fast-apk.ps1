# Fast Install APK - Windows
# Uso: fast-apk.ps1 install|uninstall <caminho-do-apk>
param(
    [Parameter(Mandatory = $true, Position = 0)][ValidateSet('install', 'uninstall', 'package')][string]$Action,
    [Parameter(Mandatory = $true, Position = 1)][string]$Apk
)

$ErrorActionPreference = 'Stop'
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}
Add-Type -AssemblyName System.IO.Compression, System.IO.Compression.FileSystem
Add-Type -AssemblyName System.Windows.Forms

function Write-Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "OK  $msg" -ForegroundColor Green }
function Write-Err($msg)  { Write-Host "ERRO $msg" -ForegroundColor Red }

function Exit-Script([int]$code) {
    if ($Action -eq 'package') { exit $code }
    if ($code -eq 0) {
        Write-Host ''
        Write-Host 'Fechando em 3 segundos...' -ForegroundColor DarkGray
        Start-Sleep -Seconds 3
    } else {
        Write-Host ''
        Read-Host 'Pressione Enter para fechar' | Out-Null
    }
    exit $code
}

function Find-Adb {
    $cmd = Get-Command adb.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $roots = @($env:ANDROID_HOME, $env:ANDROID_SDK_ROOT, "$env:LOCALAPPDATA\Android\Sdk", "$env:USERPROFILE\AppData\Local\Android\Sdk", 'C:\Android\Sdk', 'D:\AndroidSDK')
    foreach ($r in $roots) {
        if ($r) {
            $p = Join-Path $r 'platform-tools\adb.exe'
            if (Test-Path -LiteralPath $p) { return $p }
        }
    }
    return $null
}

# ---------------------------------------------------------------------------
# Le o package name direto do AndroidManifest.xml binario (sem precisar do aapt)
# ---------------------------------------------------------------------------
function Get-ApkPackageName([string]$path) {
    $zip = [System.IO.Compression.ZipFile]::OpenRead($path)
    try {
        $entry = $zip.GetEntry('AndroidManifest.xml')
        if (-not $entry) { throw 'AndroidManifest.xml nao encontrado no APK.' }
        $ms = New-Object System.IO.MemoryStream
        $s = $entry.Open(); $s.CopyTo($ms); $s.Close()
        $b = $ms.ToArray()
    } finally { $zip.Dispose() }

    $u16 = { param($o) [int]$b[$o] -bor ([int]$b[$o + 1] -shl 8) }
    $u32 = { param($o) [BitConverter]::ToUInt32($b, $o) }

    $pool = $null
    $getStr = {
        param([uint32]$i)
        if ($null -eq $pool -or $i -ge $pool.Count) { return $null }
        $o = $pool.Start + $pool.StringsStart + (& $u32 ($pool.Start + $pool.HeaderSize + $i * 4))
        if ($pool.Utf8) {
            if ($b[$o] -band 0x80) { $o += 2 } else { $o += 1 }              # tamanho em chars
            $len = [int]$b[$o]
            if ($len -band 0x80) { $len = (($len -band 0x7F) -shl 8) -bor $b[$o + 1]; $o += 2 } else { $o += 1 }
            return [System.Text.Encoding]::UTF8.GetString($b, $o, $len)
        } else {
            $len = & $u16 $o
            if ($len -band 0x8000) { $len = (($len -band 0x7FFF) -shl 16) -bor (& $u16 ($o + 2)); $o += 4 } else { $o += 2 }
            return [System.Text.Encoding]::Unicode.GetString($b, $o, $len * 2)
        }
    }

    $pos = & $u16 2
    while ($pos -lt $b.Length) {
        $type = & $u16 $pos
        $hsize = & $u16 ($pos + 2)
        $size = & $u32 ($pos + 4)
        if ($size -le 0) { break }
        if ($type -eq 0x0001) {
            $pool = @{
                Start = $pos; HeaderSize = $hsize
                Count = & $u32 ($pos + 8)
                Utf8 = (((& $u32 ($pos + 16)) -band 0x100) -ne 0)
                StringsStart = & $u32 ($pos + 20)
            }
        } elseif ($type -eq 0x0102) {
            $ext = $pos + $hsize
            if ((& $getStr (& $u32 ($ext + 4))) -ne 'manifest') { break }
            $attrStart = & $u16 ($ext + 8)
            $attrSize = & $u16 ($ext + 10)
            $attrCount = & $u16 ($ext + 12)
            for ($a = 0; $a -lt $attrCount; $a++) {
                $ao = $ext + $attrStart + $a * $attrSize
                if ((& $getStr (& $u32 ($ao + 4))) -eq 'package') {
                    $raw = & $u32 ($ao + 8)
                    if ($raw -ne [uint32]::MaxValue) { return & $getStr $raw }
                    if ($b[$ao + 15] -eq 3) { return & $getStr (& $u32 ($ao + 16)) }
                }
            }
            break
        }
        $pos += $size
    }
    throw 'Nao foi possivel ler o package name do APK.'
}

function Invoke-Adb([string[]]$adbArgs) {
    $prev = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    $lines = & $script:Adb @adbArgs 2>&1 | ForEach-Object { "$_" }
    $script:AdbExit = $LASTEXITCODE
    $ErrorActionPreference = $prev
    return , @($lines)
}

function Select-Devices {
    $lines = Invoke-Adb @('devices')
    $ready = @(); $other = @()
    foreach ($l in $lines) {
        if ($l -match '^(\S+)\s+(device|unauthorized|offline|recovery|sideload)\s*$') {
            if ($Matches[2] -eq 'device') { $ready += $Matches[1] } else { $other += "$($Matches[1]) ($($Matches[2]))" }
        }
    }
    if ($ready.Count -eq 0) {
        Write-Err 'Nenhum dispositivo pronto encontrado.'
        if ($other.Count -gt 0) {
            Write-Host "Dispositivos com problema: $($other -join ', ')" -ForegroundColor Yellow
            Write-Host 'Se aparecer "unauthorized", aceite a depuracao USB no celular.' -ForegroundColor Yellow
        }
        Exit-Script 1
    }
    if ($ready.Count -eq 1) { return , $ready }

    Write-Host ''
    Write-Host 'Mais de um dispositivo conectado:' -ForegroundColor Yellow
    for ($i = 0; $i -lt $ready.Count; $i++) {
        $model = (Invoke-Adb @('-s', $ready[$i], 'shell', 'getprop', 'ro.product.model')) -join ''
        Write-Host ("  [{0}] {1}  {2}" -f ($i + 1), $ready[$i], $model.Trim())
    }
    Write-Host '  [0] Todos'
    while ($true) {
        $sel = Read-Host 'Escolha o dispositivo'
        if ($sel -eq '0') { return , $ready }
        $n = 0
        if ([int]::TryParse($sel, [ref]$n) -and $n -ge 1 -and $n -le $ready.Count) { return , @($ready[$n - 1]) }
    }
}

function Confirm-Box([string]$text, [string]$title) {
    $r = [System.Windows.Forms.MessageBox]::Show($text, $title,
        [System.Windows.Forms.MessageBoxButtons]::YesNo,
        [System.Windows.Forms.MessageBoxIcon]::Warning,
        [System.Windows.Forms.MessageBoxDefaultButton]::Button2)
    return $r -eq [System.Windows.Forms.DialogResult]::Yes
}

function Test-Installed([string]$serial, [string]$pkg) {
    $out = Invoke-Adb @('-s', $serial, 'shell', 'pm', 'list', 'packages', $pkg)
    return [bool]($out | Where-Object { $_.Trim() -eq "package:$pkg" })
}

function Install-On([string]$serial, [string]$pkg) {
    Write-Step "Instalando em $serial ..."
    $out = Invoke-Adb @('-s', $serial, 'install', '-r', $Apk)
    $out | ForEach-Object { Write-Host "    $_" }
    $text = $out -join "`n"

    if ($text -match 'INSTALL_FAILED_UPDATE_INCOMPATIBLE|INSTALL_FAILED_VERSION_DOWNGRADE') {
        $why = if ($text -match 'DOWNGRADE') { 'a versao instalada e mais nova' } else { 'a assinatura e diferente da versao instalada' }
        if (Confirm-Box "Nao foi possivel atualizar $pkg porque $why.`n`nDesinstalar a versao atual (os dados do app serao apagados) e instalar de novo?" 'Fast Install APK') {
            Invoke-Adb @('-s', $serial, 'uninstall', $pkg) | ForEach-Object { Write-Host "    $_" }
            $out = Invoke-Adb @('-s', $serial, 'install', '-r', $Apk)
            $out | ForEach-Object { Write-Host "    $_" }
            $text = $out -join "`n"
        }
    }

    if ($text -notmatch '(?m)^Success') { Write-Err "Falha ao instalar em $serial."; return $false }
    Write-Ok "Instalado em $serial."

    Write-Step "Abrindo $pkg ..."
    $out = Invoke-Adb @('-s', $serial, 'shell', 'monkey', '-p', $pkg, '-c', 'android.intent.category.LAUNCHER', '1')
    if (($out -join "`n") -match 'No activities found|monkey aborted') {
        Write-Host '    App instalado, mas nao tem activity de launcher para abrir.' -ForegroundColor Yellow
    } else { Write-Ok 'App aberto.' }
    return $true
}

function Uninstall-From([string]$serial, [string]$pkg) {
    if (-not (Test-Installed $serial $pkg)) {
        Write-Host "    $pkg nao esta instalado em $serial." -ForegroundColor Yellow
        return $true
    }
    Write-Step "Desinstalando $pkg de $serial ..."
    $out = Invoke-Adb @('-s', $serial, 'uninstall', $pkg)
    $out | ForEach-Object { Write-Host "    $_" }
    if (($out -join "`n") -notmatch '(?m)^Success') { Write-Err "Falha ao desinstalar de $serial."; return $false }
    Write-Ok "Desinstalado de $serial."
    return $true
}

# ---------------------------------------------------------------------------
try {
    $Apk = (Resolve-Path -LiteralPath $Apk).ProviderPath
    $pkg = Get-ApkPackageName $Apk
} catch {
    if ($Action -eq 'package') { [Console]::Error.WriteLine($_.Exception.Message); exit 1 }
    $Host.UI.RawUI.WindowTitle = 'Fast Install APK'
    Write-Err $_.Exception.Message
    Exit-Script 1
}

if ($Action -eq 'package') { Write-Output $pkg; exit 0 }

$Host.UI.RawUI.WindowTitle = "Fast Install APK - $pkg"
Write-Host "APK:     $Apk"
Write-Host "Package: $pkg" -ForegroundColor White

$script:Adb = Find-Adb
if (-not $script:Adb) {
    Write-Err 'adb nao encontrado. Instale o Android SDK Platform-Tools e coloque no PATH (ou defina ANDROID_HOME).'
    Exit-Script 1
}

$devices = Select-Devices
$ok = $true

if ($Action -eq 'install') {
    foreach ($d in $devices) { if (-not (Install-On $d $pkg)) { $ok = $false } }
} else {
    $target = if ($devices.Count -gt 1) { "$($devices.Count) dispositivos" } else { $devices[0] }
    if (-not (Confirm-Box "Desinstalar o app abaixo?`n`n$pkg`n`nDispositivo: $target`n`nOs dados do app serao apagados." 'Fast Install APK - Desinstalar')) {
        Write-Host 'Cancelado pelo usuario.' -ForegroundColor Yellow
        Exit-Script 0
    }
    foreach ($d in $devices) { if (-not (Uninstall-From $d $pkg)) { $ok = $false } }
}

if ($ok) { Exit-Script 0 } else { Exit-Script 1 }
