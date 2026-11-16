$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$root = Split-Path $PSScriptRoot -Parent
$target = Join-Path $root 'desktop\UsbRelay\Assets\usb-relay.ico'
New-Item -ItemType Directory -Force -Path (Split-Path $target) | Out-Null
$bitmap = New-Object System.Drawing.Bitmap 256,256
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$path = New-Object System.Drawing.Drawing2D.GraphicsPath
$fill = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(71,215,181))
$pen = New-Object System.Drawing.Pen ([System.Drawing.Color]::FromArgb(8,45,39)),18
$pen.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
$pen.EndCap = [System.Drawing.Drawing2D.LineCap]::Round
$memory = New-Object System.IO.MemoryStream
try {
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $graphics.Clear([System.Drawing.Color]::Transparent)
    $path.AddArc(4,4,96,96,180,90); $path.AddArc(156,4,96,96,270,90)
    $path.AddArc(156,156,96,96,0,90); $path.AddArc(4,156,96,96,90,90); $path.CloseFigure()
    $graphics.FillPath($fill,$path)
    $graphics.DrawLine($pen,78,178,178,78)
    $graphics.DrawLine($pen,116,78,178,78)
    $graphics.DrawLine($pen,178,78,178,140)
    $bitmap.Save($memory,[System.Drawing.Imaging.ImageFormat]::Png)
    $png = $memory.ToArray()
    $output = [System.IO.File]::Create($target)
    $writer = New-Object System.IO.BinaryWriter $output
    try {
        $writer.Write([uint16]0); $writer.Write([uint16]1); $writer.Write([uint16]1)
        $writer.Write([byte]0); $writer.Write([byte]0); $writer.Write([byte]0); $writer.Write([byte]0)
        $writer.Write([uint16]1); $writer.Write([uint16]32); $writer.Write([uint32]$png.Length); $writer.Write([uint32]22); $writer.Write($png)
    } finally { $writer.Dispose() }
} finally { $memory.Dispose(); $pen.Dispose(); $fill.Dispose(); $path.Dispose(); $graphics.Dispose(); $bitmap.Dispose() }
