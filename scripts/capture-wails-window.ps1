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

    [switch]$KeepOpen
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Drawing
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
    public static extern bool GetWindowRect(IntPtr hWnd, out Rect rect);

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
    throw "ugxlsx exited before its desktop window was ready (exit code $($process.ExitCode))."
}
if ($windowHandle -eq [IntPtr]::Zero) {
    throw "Timed out waiting for the ugxlsx desktop window (PID $($process.Id))."
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

$rect = New-Object UgglsxAcceptanceWindow+Rect
if (-not [UgglsxAcceptanceWindow]::GetWindowRect($windowHandle, [ref]$rect)) {
    throw "Could not read the ugxlsx window bounds (PID $($process.Id))."
}

$captureWidth = $rect.Right - $rect.Left
$captureHeight = $rect.Bottom - $rect.Top
if ($captureWidth -le 0 -or $captureHeight -le 0) {
    throw "The ugxlsx window has invalid bounds: ${captureWidth}x${captureHeight}."
}

$bitmap = New-Object System.Drawing.Bitmap($captureWidth, $captureHeight)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
try {
    $graphics.CopyFromScreen(
        $rect.Left,
        $rect.Top,
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
