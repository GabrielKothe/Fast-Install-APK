# Gera os icones do menu (<id>.ico), o logo (logo.ico) e os logos do pacote (Assets\logo*.png) em <OutDir>
# Cada icone: circulo colorido (cor do grupo) + simbolo branco da fonte de icones do Windows.
param(
    [Parameter(Mandatory = $true)][string]$OutDir,
    [string]$Preview   # opcional: gera uma folha PNG com todos os icones para conferir
)
Add-Type -AssemblyName System.Drawing
New-Item -ItemType Directory -Force -Path (Join-Path $OutDir 'Assets') | Out-Null

function C([int]$r, [int]$g, [int]$b) { [System.Drawing.Color]::FromArgb(255, $r, $g, $b) }
$green  = C 61 220 132   # instalar
$amber  = C 245 166 35   # app (parar/limpar)
$red    = C 232 72 85    # desinstalar
$purple = C 142 110 235  # diagnostico
$teal   = C 0 168 168    # evidencias
$blue   = C 45 127 249   # conexao

# id -> simbolo (Segoe MDL2 Assets / Segoe Fluent Icons) e cor
$icons = [ordered]@{
    'install'           = @(0xE896, $green)   # Download
    'install-grant'     = @(0xE8D7, $green)   # Permissions
    'install-downgrade' = @(0xE7A7, $green)   # Undo
    'install-all'       = @(0xE975, $green)   # Devices (varios)
    'reinstall'         = @(0xE72C, $green)   # Refresh
    'restart'           = @(0xE768, $amber)   # Play
    'clear'             = @(0xE75C, $amber)   # EraseTool
    'uninstall'         = @(0xE74D, $red)     # Delete
    'logcat'            = @(0xE756, $purple)  # CommandPrompt
    'info'              = @(0xE946, $purple)  # Info
    'compare'           = @(0xE8AB, $purple)  # Switch
    'mirror'            = @(0xE7F4, $teal)    # Monitor (espelhar)
    'screenshot'        = @(0xE722, $teal)    # Camera
    'record'            = @(0xE714, $teal)    # Video
    'pull'              = @(0xE8B7, $teal)    # Folder
    'wifi'              = @(0xE701, $blue)    # Wifi
}

$fontName = 'Segoe MDL2 Assets'
if ((New-Object System.Drawing.Text.InstalledFontCollection).Families.Name -contains 'Segoe Fluent Icons') { $fontName = 'Segoe Fluent Icons' }

function New-GlyphBitmap([int]$size, [int]$glyph, [System.Drawing.Color]$bg) {
    $bmp = New-Object System.Drawing.Bitmap $size, $size
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'
    $g.TextRenderingHint = 'AntiAliasGridFit'
    $g.Clear([System.Drawing.Color]::Transparent)
    $g.FillEllipse((New-Object System.Drawing.SolidBrush $bg), 0.5, 0.5, $size - 1, $size - 1)
    $font = New-Object System.Drawing.Font $fontName, ($size * 0.54), ([System.Drawing.FontStyle]::Regular), ([System.Drawing.GraphicsUnit]::Pixel)
    $fmt = New-Object System.Drawing.StringFormat
    $fmt.Alignment = 'Center'; $fmt.LineAlignment = 'Center'
    $rect = New-Object System.Drawing.RectangleF 0, ($size * 0.02), $size, $size
    $g.DrawString([string][char]$glyph, $font, [System.Drawing.Brushes]::White, $rect, $fmt)
    $font.Dispose(); $g.Dispose()
    return $bmp
}

function New-LogoBitmap([int]$size) {
    $bmp = New-Object System.Drawing.Bitmap $size, $size
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'
    $g.Clear([System.Drawing.Color]::Transparent)
    $s = $size / 32.0
    # cabeca do Android: meia-lua verde com antenas e olhos
    $gb = New-Object System.Drawing.SolidBrush $green
    $pen = New-Object System.Drawing.Pen $green, ([Math]::Max(1.0, 2.2 * $s))
    $pen.StartCap = 'Round'; $pen.EndCap = 'Round'
    $g.DrawLine($pen, 9 * $s, 5 * $s, 12 * $s, 10 * $s)
    $g.DrawLine($pen, 23 * $s, 5 * $s, 20 * $s, 10 * $s)
    $g.FillPie($gb, 2 * $s, 8 * $s, 28 * $s, 30 * $s, 180, 180)
    $dark = New-Object System.Drawing.SolidBrush (C 32 33 36)
    $g.FillEllipse($dark, 9 * $s, 15 * $s, 3.6 * $s, 3.6 * $s)
    $g.FillEllipse($dark, 19.4 * $s, 15 * $s, 3.6 * $s, 3.6 * $s)
    $g.Dispose()
    return $bmp
}

# $glyph = 0 gera o logo do Android
function Save-Ico([string]$path, [int]$glyph, [System.Drawing.Color]$col) {
    $sizes = 16, 20, 24, 32, 40, 48, 64, 256
    $pngs = foreach ($sz in $sizes) {
        $bmp = if ($glyph) { New-GlyphBitmap $sz $glyph $col } else { New-LogoBitmap $sz }
        $ms = New-Object System.IO.MemoryStream
        $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png); $bmp.Dispose()
        , $ms.ToArray()
    }
    $w = New-Object System.IO.BinaryWriter ([System.IO.File]::Create($path))
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

foreach ($id in $icons.Keys) {
    $glyph, $col = $icons[$id]
    Save-Ico (Join-Path $OutDir "$id.ico") $glyph $col
}
Save-Ico (Join-Path $OutDir 'logo.ico') 0 $green
foreach ($sz in 150, 44) {
    $logo = New-LogoBitmap $sz
    $logo.Save((Join-Path $OutDir "Assets\logo$sz.png"), [System.Drawing.Imaging.ImageFormat]::Png)
    $logo.Dispose()
}

if ($Preview) {
    # folha de conferencia: cada icone em 16, 24 e 48 px, fundo escuro como o menu do Windows 11
    $rowH = 56; $sheet = New-Object System.Drawing.Bitmap 420, ($rowH * $icons.Count + 10)
    $g = [System.Drawing.Graphics]::FromImage($sheet)
    $g.Clear((C 44 44 44)); $g.TextRenderingHint = 'AntiAliasGridFit'
    $label = New-Object System.Drawing.Font 'Segoe UI', 13
    $y = 6
    foreach ($id in $icons.Keys) {
        $glyph, $col = $icons[$id]
        $x = 8
        foreach ($sz in 16, 24, 48) {
            $b = New-GlyphBitmap $sz $glyph $col
            $g.DrawImage($b, $x, $y + (48 - $sz) / 2, $sz, $sz); $b.Dispose()
            $x += $sz + 12
        }
        $g.DrawString($id, $label, [System.Drawing.Brushes]::White, $x + 8, $y + 14)
        $y += $rowH
    }
    $g.Dispose(); $sheet.Save($Preview, [System.Drawing.Imaging.ImageFormat]::Png); $sheet.Dispose()
}
Write-Host "Fonte de icones: $fontName"
