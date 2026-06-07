$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$pluginRoot = Split-Path -Parent $scriptDir
$binDir = Join-Path $pluginRoot "bin"

$ensureBin = Join-Path $binDir "ensure-server.exe"
$hookBin = Join-Path $binDir "codex-hook.exe"

if ((Test-Path $ensureBin) -and (Test-Path $hookBin)) {
    & $ensureBin 1>$null
    $stdin = [Console]::In.ReadToEnd()
    $stdin | & $hookBin
    exit $LASTEXITCODE
}

Write-Error "Imprint Codex hook binaries are missing from $binDir. Run go run ./cmd/install."
exit 0
