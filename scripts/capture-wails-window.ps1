[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ExePath,

    [string[]]$ArgumentList = @(),

    [Parameter(Mandatory = $true)]
    [string]$OutputPath,

    [ValidateRange(800, 4096)]
    [int]$WindowWidth = 2400,

    [ValidateRange(600, 2160)]
    [int]$WindowHeight = 1050,

    [ValidateRange(0, 30)]
    [int]$WarmupSeconds = 5,

    [ValidateRange(1, 30)]
    [int]$WindowTimeoutSeconds = 15,

    [string[]]$Keys = @(),

    [ValidateRange(50, 5000)]
    [int]$KeyDelayMilliseconds = 500,

    [string[]]$Clicks = @(),

    [string[]]$RightClicks = @(),

    [switch]$ClientAreaOnly,

    [ValidateRange(0, 4096)]
    [int]$CaptureX = 0,

    [ValidateRange(0, 2160)]
    [int]$CaptureY = 0,

    [ValidateRange(0, 4096)]
    [int]$CaptureWidth = 0,

    [ValidateRange(0, 2160)]
    [int]$CaptureHeight = 0,

    [switch]$KeepOpen
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type @'
using System;
using System.Runtime.InteropServices;

public static class UgglsxAcceptanceWindow {
    private delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

    [StructLayout(LayoutKind.Sequential)]
    public struct Rect {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct Point {
        public int X;
        public int Y;
    }

    [DllImport("user32.dll")]
    public static extern bool SetProcessDPIAware();

    [DllImport("user32.dll", SetLastError = true)]
    public static extern bool SetWindowPos(
        IntPtr hWnd,
        IntPtr hWndInsertAfter,
        int x,
        int y,
        int cx,
        int cy,
        uint flags
    );

    [DllImport("user32.dll")]
    public static extern bool ShowWindow(IntPtr hWnd, int command);

    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);

    [DllImport("user32.dll")]
    public static extern bool SetCursorPos(int x, int y);

    [DllImport("user32.dll")]
    public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extraInfo);

    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hWnd, out Rect rect);

    [DllImport("user32.dll")]
    public static extern bool GetClientRect(IntPtr hWnd, out Rect rect);

    [DllImport("user32.dll")]
    public static extern bool ClientToScreen(IntPtr hWnd, ref Point point);

    [DllImport("user32.dll")]
    public static extern bool PostMessage(IntPtr hWnd, uint message, IntPtr wParam, IntPtr lParam);

    [DllImport("user32.dll")]
    private static extern bool EnumWindows(EnumWindowsProc callback, IntPtr lParam);

    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);

    [DllImport("user32.dll")]
    private static extern bool IsWindowVisible(IntPtr hWnd);

    public static IntPtr FindTopLevelWindow(int processId) {
        IntPtr result = IntPtr.Zero;
        EnumWindows(delegate(IntPtr hWnd, IntPtr lParam) {
            uint windowProcessId;
            GetWindowThreadProcessId(hWnd, out windowProcessId);
            if (windowProcessId == processId && IsWindowVisible(hWnd)) {
                result = hWnd;
                return false;
            }
            return true;
        }, IntPtr.Zero);
        return result;
    }
}
'@

[void][UgglsxAcceptanceWindow]::SetProcessDPIAware()

$resolvedExe = (Resolve-Path -LiteralPath $ExePath).Path
$resolvedOutput = [System.IO.Path]::GetFullPath($OutputPath)
$outputDirectory = Split-Path -Parent $resolvedOutput
if (-not [string]::IsNullOrWhiteSpace($outputDirectory)) {
    [System.IO.Directory]::CreateDirectory($outputDirectory) | Out-Null
}

$process = Start-Process -FilePath $resolvedExe -ArgumentList $ArgumentList -PassThru
$deadline = [DateTime]::UtcNow.AddSeconds($WindowTimeoutSeconds)
$windowHandle = [IntPtr]::Zero

do {
    Start-Sleep -Milliseconds 100
    $process.Refresh()
    $windowHandle = [UgglsxAcceptanceWindow]::FindTopLevelWindow($process.Id)
} while ($windowHandle -eq [IntPtr]::Zero -and -not $process.HasExited -and [DateTime]::UtcNow -lt $deadline)

if ($process.HasExited) {
    throw "SheetProof exited before its desktop window was ready (exit code $($process.ExitCode))."
}
if ($windowHandle -eq [IntPtr]::Zero) {
    throw "Timed out waiting for the SheetProof desktop window (PID $($process.Id))."
}

[void][UgglsxAcceptanceWindow]::ShowWindow($windowHandle, 9)
[void][UgglsxAcceptanceWindow]::SetWindowPos(
    $windowHandle,
    [IntPtr](-1),
    20,
    20,
    $WindowWidth,
    $WindowHeight,
    0x0040
)
[void][UgglsxAcceptanceWindow]::SetForegroundWindow($windowHandle)

if ($WarmupSeconds -gt 0) {
    Start-Sleep -Seconds $WarmupSeconds
}

foreach ($key in $Keys) {
    [void][UgglsxAcceptanceWindow]::SetForegroundWindow($windowHandle)
    [System.Windows.Forms.SendKeys]::SendWait($key)
    Start-Sleep -Milliseconds $KeyDelayMilliseconds
}

foreach ($click in $RightClicks) {
    $parts = $click.Split(',')
    if ($parts.Count -ne 2) {
        throw "Right-click coordinates must use x,y window-relative format: $click"
    }
    $clickX = 0
    $clickY = 0
    if (-not [int]::TryParse($parts[0], [ref]$clickX) -or -not [int]::TryParse($parts[1], [ref]$clickY)) {
        throw "Right-click coordinates must be integers: $click"
    }
    $clickRect = New-Object UgglsxAcceptanceWindow+Rect
    if (-not [UgglsxAcceptanceWindow]::GetWindowRect($windowHandle, [ref]$clickRect)) {
        throw "Could not read the SheetProof window bounds before right-click: $click"
    }
    [void][UgglsxAcceptanceWindow]::SetForegroundWindow($windowHandle)
    [void][UgglsxAcceptanceWindow]::SetCursorPos($clickRect.Left + $clickX, $clickRect.Top + $clickY)
    try {
        [UgglsxAcceptanceWindow]::mouse_event(0x0008, 0, 0, 0, [UIntPtr]::Zero)
    } finally {
        [UgglsxAcceptanceWindow]::mouse_event(0x0010, 0, 0, 0, [UIntPtr]::Zero)
    }
    Start-Sleep -Milliseconds $KeyDelayMilliseconds
}

foreach ($click in $Clicks) {
    $parts = $click.Split(',')
    if ($parts.Count -ne 2) {
        throw "Click coordinates must use x,y window-relative format: $click"
    }
    $clickX = 0
    $clickY = 0
    if (-not [int]::TryParse($parts[0], [ref]$clickX) -or -not [int]::TryParse($parts[1], [ref]$clickY)) {
        throw "Click coordinates must be integers: $click"
    }
    $clickRect = New-Object UgglsxAcceptanceWindow+Rect
    if (-not [UgglsxAcceptanceWindow]::GetWindowRect($windowHandle, [ref]$clickRect)) {
        throw "Could not read the SheetProof window bounds before click: $click"
    }
    [void][UgglsxAcceptanceWindow]::SetForegroundWindow($windowHandle)
    [void][UgglsxAcceptanceWindow]::SetCursorPos($clickRect.Left + $clickX, $clickRect.Top + $clickY)
    try {
        [UgglsxAcceptanceWindow]::mouse_event(0x0002, 0, 0, 0, [UIntPtr]::Zero)
    } finally {
        [UgglsxAcceptanceWindow]::mouse_event(0x0004, 0, 0, 0, [UIntPtr]::Zero)
    }
    Start-Sleep -Milliseconds $KeyDelayMilliseconds
}

$rect = New-Object UgglsxAcceptanceWindow+Rect
if ($ClientAreaOnly) {
    if (-not [UgglsxAcceptanceWindow]::GetClientRect($windowHandle, [ref]$rect)) {
        throw "Could not read the SheetProof client bounds (PID $($process.Id))."
    }
    $origin = New-Object UgglsxAcceptanceWindow+Point
    if (-not [UgglsxAcceptanceWindow]::ClientToScreen($windowHandle, [ref]$origin)) {
        throw "Could not locate the SheetProof client area (PID $($process.Id))."
    }
    $captureLeft = $origin.X
    $captureTop = $origin.Y
} else {
    if (-not [UgglsxAcceptanceWindow]::GetWindowRect($windowHandle, [ref]$rect)) {
        throw "Could not read the SheetProof window bounds (PID $($process.Id))."
    }
    $captureLeft = $rect.Left
    $captureTop = $rect.Top
}

$availableWidth = $rect.Right - $rect.Left
$availableHeight = $rect.Bottom - $rect.Top
$captureLeft += $CaptureX
$captureTop += $CaptureY
$captureWidth = if ($CaptureWidth -gt 0) { $CaptureWidth } else { $availableWidth - $CaptureX }
$captureHeight = if ($CaptureHeight -gt 0) { $CaptureHeight } else { $availableHeight - $CaptureY }
if ($CaptureX + $captureWidth -gt $availableWidth -or $CaptureY + $captureHeight -gt $availableHeight) {
    throw "The requested capture region exceeds the available ${availableWidth}x${availableHeight} area."
}
if ($captureWidth -le 0 -or $captureHeight -le 0) {
    throw "The SheetProof window has invalid bounds: ${captureWidth}x${captureHeight}."
}

$bitmap = New-Object System.Drawing.Bitmap($captureWidth, $captureHeight)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
try {
    $graphics.CopyFromScreen(
        $captureLeft,
        $captureTop,
        0,
        0,
        (New-Object System.Drawing.Size($captureWidth, $captureHeight))
    )
    $bitmap.Save($resolvedOutput, [System.Drawing.Imaging.ImageFormat]::Png)
} finally {
    $graphics.Dispose()
    $bitmap.Dispose()
}

[void][UgglsxAcceptanceWindow]::SetWindowPos(
    $windowHandle,
    [IntPtr](-2),
    0,
    0,
    0,
    0,
    0x0003
)

$closeRequested = $false
$exited = $false
if (-not $KeepOpen) {
    $closeRequested = [UgglsxAcceptanceWindow]::PostMessage(
        $windowHandle,
        0x0010,
        [IntPtr]::Zero,
        [IntPtr]::Zero
    )
    $exited = $process.WaitForExit(3000)
}

[pscustomobject]@{
    ProcessId = $process.Id
    Screenshot = $resolvedOutput
    Width = $captureWidth
    Height = $captureHeight
    CloseRequested = $closeRequested
    Exited = $exited
}
