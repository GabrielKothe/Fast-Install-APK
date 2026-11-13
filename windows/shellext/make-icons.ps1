# Gera install.ico, uninstall.ico e o logo do pacote (Assets\logo.png) em <OutDir>
param([Parameter(Mandatory = $true)][string]$OutDir)
Add-Type -AssemblyName System.Drawing
New-Item -ItemType Directory -Force -Path (Join-Path $OutDir 'Assets') | Out-Null

function New-Bitmap([int]$size, [string]$kind) {
    $bmp = New-Object System.Drawing.Bitmap $size, $size
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'
    $g.Clear([System.Drawing.Color]::Transparent)
    $s = $size / 32.0
    $green = [System.Drawing.Color]::FromArgb(255, 61, 220, 132)   # verde Android
    $red = [System.Drawing.Color]::FromArgb(255, 232, 72, 85)
    $white = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::White)

    if ($kind -eq 'logo') {
        # cabeca do Android: meia-lua verde com antenas e olhos
        $gb = New-Object System.Drawing.SolidBrush $green
        $pen = New-Object System.Drawing.Pen $green, ([Math]::Max(1.0, 2.2 * $s))
        $pen.StartCap = 'Round'; $pen.EndCap = 'Round'
        $g.DrawLine($pen, 9 * $s, 5 * $s, 12 * $s, 10 * $s)
        $g.DrawLine($pen, 23 * $s, 5 * $s, 20 * $s, 10 * $s)
        $g.FillPie($gb, 2 * $s, 8 * $s, 28 * $s, 30 * $s, 180, 180)
        $dark = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(255, 32, 33, 36))
        $g.FillEllipse($dark, 9 * $s, 15 * $s, 3.6 * $s, 3.6 * $s)
        $g.FillEllipse($dark, 19.4 * $s, 15 * $s, 3.6 * $s, 3.6 * $s)
        $g.Dispose()
        return $bmp
    }

    $bg = if ($kind -eq 'uninstall') { $red } else { $green }
    $g.FillEllipse((New-Object System.Drawing.SolidBrush $bg), 1 * $s, 1 * $s, 30 * $s, 30 * $s)

    if ($kind -eq 'uninstall') {
        # lixeira
        $g.FillRectangle($white, 9 * $s, 9 * $s, 14 * $s, 2.5 * $s)
        $g.FillRectangle($white, 13.5 * $s, 6.5 * $s, 5 * $s, 2.5 * $s)
        $pts = @(
            (New-Object System.Drawing.PointF (10.5 * $s), (12.5 * $s)),
            (New-Object System.Drawing.PointF (21.5 * $s), (12.5 * $s)),
            (New-Object System.Drawing.PointF (20.3 * $s), (25 * $s)),
            (New-Object System.Drawing.PointF (11.7 * $s), (25 * $s)))
        $g.FillPolygon($white, $pts)
    } else {
        # seta para baixo + base
        $g.FillRectangle($white, 13.5 * $s, 6 * $s, 5 * $s, 10 * $s)
        $pts = @(
            (New-Object System.Drawing.PointF (8.5 * $s), (15 * $s)),
            (New-Object System.Drawing.PointF (23.5 * $s), (15 * $s)),
            (New-Object System.Drawing.PointF (16 * $s), (22.5 * $s)))
        $g.FillPolygon($white, $pts)
        $g.FillRectangle($white, 8.5 * $s, 23.5 * $s, 15 * $s, 2.5 * $s)
    }
    $g.Dispose()
    return $bmp
}

function Save-Ico([string]$kind, [string]$path) {
    $sizes = 16, 20, 24, 32, 48, 64, 256
    $pngs = foreach ($sz in $sizes) {
        $bmp = New-Bitmap $sz $kind
        $ms = New-Object System.IO.MemoryStream
        $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png); $bmp.Dispose()
        , $ms.ToArray()
    }
    $fs = [System.IO.File]::Create($path)
    $w = New-Object System.IO.BinaryWriter $fs
    $w.Write([UInt16]0); $w.Write([UInt16]1); $w.Write([UInt16]$sizes.Count)
    $offset = 6 + 16 * $sizes.Count
    for ($i = 0; $i -lt $sizes.Count; $i++) {
        $d = if ($sizes[$i] -ge 256) { 0 } else { $sizes[$i] }
        $w.Write([byte]$d); $w.Write([byte]$d); $w.Write([byte]0); $w.Write([byte]0)
        $w.Write([UInt16]1); $w.Write([UInt16]32)
        $w.Write([UInt32]$pngs[$i].Length); $w.Write([UInt32]$offset)
        $offset += $pngs[$i].Length
    }
    foreach ($p in $pngs) { $w.Write($p) }
    $w.Close()
}

Save-Ico 'install' (Join-Path $OutDir 'install.ico')
Save-Ico 'uninstall' (Join-Path $OutDir 'uninstall.ico')
foreach ($sz in 150, 44) {
    $logo = New-Bitmap $sz 'logo'
    $logo.Save((Join-Path $OutDir "Assets\logo$sz.png"), [System.Drawing.Imaging.ImageFormat]::Png)
    $logo.Dispose()
}
Save-Ico 'logo' (Join-Path $OutDir 'logo.ico')
