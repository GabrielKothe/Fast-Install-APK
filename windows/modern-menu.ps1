# Registra/remove o menu moderno do Windows 11 (pacote MSIX sparse, sem assinatura)
param(
    [Parameter(Mandatory = $true)][ValidateSet('register', 'unregister')][string]$Action,
    [string]$AppDir = (Join-Path $env:LOCALAPPDATA 'FastInstallAPK')
)
$name = 'FastInstallAPK.ContextMenu'

Get-AppxPackage -Name $name -ErrorAction SilentlyContinue | ForEach-Object {
    try { Remove-AppxPackage -Package $_.PackageFullName -ErrorAction Stop } catch { Write-Warning $_.Exception.Message }
}
# Libera a DLL caso o Explorer/dllhost ainda esteja com ela carregada
Get-Process dllhost -ErrorAction SilentlyContinue | Where-Object {
    try { $_.Modules.ModuleName -contains 'FastApkMenu.dll' } catch { $false }
} | Stop-Process -Force -ErrorAction SilentlyContinue

if ($Action -eq 'register') {
    if ([Environment]::OSVersion.Version.Build -lt 22000) { Write-Host 'Windows 10: menu moderno nao se aplica.'; exit 0 }
    try {
        Add-AppxPackage -Path (Join-Path $AppDir 'FastInstallAPK.msix') -ExternalLocation $AppDir -AllowUnsigned -ErrorAction Stop
        Write-Host 'Menu do Windows 11 registrado.'
    } catch {
        Write-Warning "Nao foi possivel registrar o menu do Windows 11: $($_.Exception.Message)"
        exit 1
    }
}
exit 0
