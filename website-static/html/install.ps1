# Flatkey — Claude Code setup for Windows PowerShell
$ErrorActionPreference = "Stop"
$BaseUrl = "https://router.flatkey.ai"
$KeyUrl = "https://console.flatkey.ai/keys"

Write-Host ""
Write-Host "===========================================" -ForegroundColor DarkGray
Write-Host "  Flatkey — coding agent setup" -ForegroundColor DarkGray
Write-Host "===========================================" -ForegroundColor DarkGray
Write-Host ""
Write-Host "Which coding agent do you want to install?" -ForegroundColor Gray
Write-Host "  1) Claude Code"
Write-Host "  2) Codex CLI"
$Agent = ""
while (-not $Agent) {
  $Choice = Read-Host -Prompt "Enter 1 or 2 (default: 1)"
  if (-not $Choice) { $Choice = "1" }
  if ($Choice -eq "1") { $Agent = "claude" }
  elseif ($Choice -eq "2") { $Agent = "codex" }
  else { Write-Host "Please enter 1 or 2." -ForegroundColor Yellow }
}
Write-Host ""

Write-Host -NoNewline "Checking Node.js... "
try {
  $NodeVersion = node --version 2>$null
  if (-not $NodeVersion) { throw "Node.js missing" }
  Write-Host "ok $NodeVersion" -ForegroundColor Green
} catch {
  Write-Host "not found" -ForegroundColor Yellow
  $Winget = Get-Command winget -ErrorAction SilentlyContinue
  if (-not $Winget) {
    Write-Host "Install Node.js LTS from https://nodejs.org/ and re-run this script." -ForegroundColor Red
    exit 1
  }
  winget install OpenJS.NodeJS.LTS --silent --accept-package-agreements --accept-source-agreements
  $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
}

if ($Agent -eq "claude") {
  Write-Host -NoNewline "Checking Claude Code... "
  $Claude = Get-Command claude -ErrorAction SilentlyContinue
  if ($Claude) {
    Write-Host "ok" -ForegroundColor Green
  } else {
    Write-Host "installing" -ForegroundColor Yellow
    npm install -g @anthropic-ai/claude-code
  }
} else {
  Write-Host -NoNewline "Checking Codex CLI... "
  $Codex = Get-Command codex -ErrorAction SilentlyContinue
  if ($Codex) {
    Write-Host "ok" -ForegroundColor Green
  } else {
    Write-Host "installing" -ForegroundColor Yellow
    npm install -g @openai/codex
  }
}

Write-Host "Create or copy your Flatkey API key:" -ForegroundColor DarkGray
Write-Host "  $KeyUrl" -ForegroundColor Yellow
$ApiKey = Read-Host -Prompt "Paste Flatkey API key"
if (-not $ApiKey) {
  Write-Host "API key required." -ForegroundColor Red
  exit 1
}

[Environment]::SetEnvironmentVariable("FLATKEY_API_KEY", $ApiKey, "User")
$env:FLATKEY_API_KEY = $ApiKey
if ($Agent -eq "claude") {
  [Environment]::SetEnvironmentVariable("ANTHROPIC_BASE_URL", $BaseUrl, "User")
  [Environment]::SetEnvironmentVariable("ANTHROPIC_AUTH_TOKEN", $ApiKey, "User")
  [Environment]::SetEnvironmentVariable("ANTHROPIC_API_KEY", "", "User")
  $env:ANTHROPIC_BASE_URL = $BaseUrl
  $env:ANTHROPIC_AUTH_TOKEN = $ApiKey
  $env:ANTHROPIC_API_KEY = ""

  $ClaudeDir = Join-Path $env:USERPROFILE ".claude"
  $SettingsFile = Join-Path $ClaudeDir "settings.json"
  New-Item -ItemType Directory -Path $ClaudeDir -Force | Out-Null
  try {
    $Settings = Get-Content -Path $SettingsFile -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
  } catch {
    $Settings = [PSCustomObject]@{}
  }
  if (-not $Settings.PSObject.Properties["env"]) {
    $Settings | Add-Member -NotePropertyName env -NotePropertyValue ([PSCustomObject]@{}) -Force
  }
  $Settings.env | Add-Member -NotePropertyName FLATKEY_API_KEY -NotePropertyValue $ApiKey -Force
  $Settings.env | Add-Member -NotePropertyName ANTHROPIC_BASE_URL -NotePropertyValue $BaseUrl -Force
  $Settings.env | Add-Member -NotePropertyName ANTHROPIC_AUTH_TOKEN -NotePropertyValue $ApiKey -Force
  $Settings.env | Add-Member -NotePropertyName ANTHROPIC_API_KEY -NotePropertyValue "" -Force
  $Settings | ConvertTo-Json -Depth 10 | Set-Content -Path $SettingsFile -Encoding UTF8
} else {
  $CodexDir = Join-Path $env:USERPROFILE ".codex"
  New-Item -ItemType Directory -Path $CodexDir -Force | Out-Null
  $ConfigToml = @"
model_provider = ""flatkey""
model = ""gpt-5.5""

[model_providers.flatkey]
name = ""Flatkey""
base_url = ""https://router.flatkey.ai/v1""
env_key = ""FLATKEY_API_KEY""
"@
  Set-Content -Path (Join-Path $CodexDir "config.toml") -Value $ConfigToml -Encoding UTF8
}

Write-Host ""
if ($Agent -eq "claude") {
  Write-Host "Done. Open a new PowerShell window, then run: claude" -ForegroundColor Green
} else {
  Write-Host "Done. Open a new PowerShell window, then run: codex" -ForegroundColor Green
}
