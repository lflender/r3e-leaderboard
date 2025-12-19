Param(
    [string[]]$Targets = @(
        "linux-amd64",
        "linux-386",
        "linux-arm64",
        "linux-armv7",
        "linux-armv6"
    ),
    [string]$OutputDir = "bin",
    [string]$AppName = "r3e-leaderboard"
)

# Verify Go is available
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go toolchain not found. Install Go and ensure 'go' is on PATH."; exit 1
}

# Ensure output directory exists
if (-not (Test-Path $OutputDir)) { New-Item -ItemType Directory -Path $OutputDir | Out-Null }

function Build-Target {
    param(
        [string]$Target
    )
    $parts = $Target.Split('-')
    if ($parts.Length -lt 2) { Write-Error "Invalid target: $Target"; return }
    $goos = $parts[0]
    $arch = $parts[1]

    $envVars = @{ GOOS = $goos; CGO_ENABLED = '0' }
    $outName = "$AppName-$goos-$arch"

    switch ($arch) {
        'armv7' {
            $envVars['GOARCH'] = 'arm'; $envVars['GOARM'] = '7'
        }
        'armv6' {
            $envVars['GOARCH'] = 'arm'; $envVars['GOARM'] = '6'
        }
        default {
            $envVars['GOARCH'] = $arch
        }
    }

    Write-Host "Building $Target -> $OutputDir/$outName" -ForegroundColor Cyan
    $ldflags = "-s -w"

    $procEnv = [System.Collections.Hashtable]::Synchronized(@{})
    $envVars.GetEnumerator() | ForEach-Object { $procEnv[$_.Key] = $_.Value }

    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = "go"
    $startInfo.Arguments = "build -trimpath -ldflags \"$ldflags\" -o \"$OutputDir/$outName\""
    $startInfo.RedirectStandardError = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true

    # Apply env vars
    foreach ($k in $procEnv.Keys) { $startInfo.EnvironmentVariables[$k] = $procEnv[$k] }

    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo = $startInfo
    $null = $proc.Start()
    $stdout = $proc.StandardOutput.ReadToEnd()
    $stderr = $proc.StandardError.ReadToEnd()
    $proc.WaitForExit()

    if ($stdout) { Write-Host $stdout }
    if ($stderr) {
        if ($proc.ExitCode -ne 0) { Write-Error $stderr } else { Write-Host $stderr }
    }

    if ($proc.ExitCode -ne 0) {
        Write-Error "Build failed for $Target (exit $($proc.ExitCode))"
    } else {
        Write-Host "Built: $OutputDir/$outName" -ForegroundColor Green
    }
}

$Targets | ForEach-Object { Build-Target -Target $_ }