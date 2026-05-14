$ErrorActionPreference = "Stop"

function Write-Stderr {
    param([string]$Message)
    [Console]::Error.WriteLine($Message)
}

function Invoke-StderrCommand {
    param(
        [string]$FilePath,
        [string[]]$Arguments
    )

    & $FilePath @Arguments 2>&1 | ForEach-Object {
        [Console]::Error.WriteLine($_)
    }
    return $LASTEXITCODE
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$pluginRoot = Split-Path -Parent $scriptDir
$projectRoot = Split-Path -Parent $pluginRoot
$binDir = Join-Path $pluginRoot "bin"

$serverBin = Join-Path $binDir "imprint.exe"
$ensureBin = Join-Path $binDir "ensure-server.exe"
$mcpBin = Join-Path $binDir "mcp-server.exe"
$watchBin = Join-Path $binDir "codex-watch.exe"
$hookBin = Join-Path $binDir "codex-hook.exe"

function Test-ImprintBinaries {
    return (Test-Path $serverBin) -and (Test-Path $ensureBin) -and (Test-Path $mcpBin) -and (Test-Path $watchBin) -and (Test-Path $hookBin)
}

function Build-ImprintBinaries {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Imprint Codex plugin requires prebuilt plugin/bin binaries or Go in PATH to build them."
    }

    $installerMain = Join-Path $projectRoot "cmd\install\main.go"
    if (-not (Test-Path $installerMain)) {
        throw "Cannot find Imprint source installer at $installerMain."
    }

    $builder = Join-Path ([IO.Path]::GetTempPath()) ("imprint-builder-{0}.exe" -f $PID)
    try {
        Push-Location $projectRoot
        try {
            Write-Stderr "[imprint] plugin/bin missing; building from source for Codex..."
            $exitCode = Invoke-StderrCommand "go" @("build", "-o", $builder, ".\cmd\install")
            if ($exitCode -ne 0) {
                throw "go build ./cmd/install failed with exit code $exitCode."
            }

            $exitCode = Invoke-StderrCommand $builder @()
            if ($exitCode -ne 0) {
                throw "Imprint installer failed with exit code $exitCode."
            }
        }
        finally {
            Pop-Location
        }
    }
    finally {
        Remove-Item -LiteralPath $builder -Force -ErrorAction SilentlyContinue
    }
}

if (-not (Test-ImprintBinaries)) {
    Build-ImprintBinaries
}

if (-not (Test-ImprintBinaries)) {
    throw "Imprint binaries were not found in $binDir after build."
}

& $ensureBin 1>$null
Start-Process -FilePath $watchBin -WindowStyle Hidden | Out-Null
& $mcpBin
exit $LASTEXITCODE
