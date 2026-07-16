#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'
BASE_URL="https://router.flatkey.ai"
KEY_URL="https://console.flatkey.ai/keys"

echo ""
echo "==========================================="
echo "  Flatkey — coding agent setup"
echo "==========================================="
echo ""
echo "Which coding agent do you want to install?"
echo "  1) Claude Code"
echo "  2) Codex CLI"
echo ""
AGENT=""
while [ -z "$AGENT" ]; do
  read -r -p "Enter 1 or 2 (default: 1): " CHOICE < /dev/tty
  CHOICE="${CHOICE:-1}"
  case "$CHOICE" in
    1) AGENT="claude" ;;
    2) AGENT="codex" ;;
    *) echo -e "${YELLOW}Please enter 1 or 2.${NC}" ;;
  esac
done
echo ""

echo -n "Checking Node.js... "
if command -v node >/dev/null 2>&1; then
  echo -e "${GREEN}ok${NC} $(node --version)"
else
  echo -e "${YELLOW}not found${NC}"
  echo "Install Node.js LTS from https://nodejs.org/ and re-run this script."
  exit 1
fi

if [ "$AGENT" = "claude" ]; then
  echo -n "Checking Claude Code... "
  if command -v claude >/dev/null 2>&1; then
    echo -e "${GREEN}ok${NC}"
  else
    echo -e "${YELLOW}installing${NC}"
    npm install -g @anthropic-ai/claude-code
  fi
else
  echo -n "Checking Codex CLI... "
  if command -v codex >/dev/null 2>&1; then
    echo -e "${GREEN}ok${NC}"
  else
    echo -e "${YELLOW}installing${NC}"
    npm install -g @openai/codex
  fi
fi

RC_FILE=""
if [ -n "${ZSH_VERSION:-}" ] || [ -f "$HOME/.zshrc" ]; then
  RC_FILE="$HOME/.zshrc"
elif [ -f "$HOME/.bashrc" ]; then
  RC_FILE="$HOME/.bashrc"
else
  RC_FILE="$HOME/.profile"
fi

echo "Create or copy your Flatkey API key:"
echo "  $KEY_URL"
echo ""
read -r -p "Paste Flatkey API key: " API_KEY < /dev/tty
if [ -z "$API_KEY" ]; then
  echo -e "${RED}API key required.${NC}"
  exit 1
fi

if [ -f "$RC_FILE" ]; then
  sed -i.bak '/# Flatkey — Claude Code proxy/,/# End Flatkey — Claude Code proxy/d' "$RC_FILE" 2>/dev/null || true
  rm -f "${RC_FILE}.bak"
fi

if [ "$AGENT" = "claude" ]; then
  {
    echo ""
    echo "# Flatkey — Claude Code proxy"
    echo "export FLATKEY_API_KEY=\"$API_KEY\""
    echo "export ANTHROPIC_BASE_URL=\"https://router.flatkey.ai\""
    echo "export ANTHROPIC_AUTH_TOKEN=\"$API_KEY\""
    echo "export ANTHROPIC_API_KEY=\"\""
    echo "# End Flatkey — Claude Code proxy"
  } >> "$RC_FILE"
else
  {
    echo ""
    echo "# Flatkey — Codex CLI proxy"
    echo "export FLATKEY_API_KEY=\"$API_KEY\""
    echo "# End Flatkey — Codex CLI proxy"
  } >> "$RC_FILE"
fi

if [ "$AGENT" = "claude" ]; then
FLATKEY_API_KEY="$API_KEY" FLATKEY_BASE_URL="$BASE_URL" node <<'NODE'
const fs = require('fs');
const path = require('path');
const dir = path.join(process.env.HOME, '.claude');
const file = path.join(dir, 'settings.json');
fs.mkdirSync(dir, { recursive: true });
let settings = {};
try { settings = JSON.parse(fs.readFileSync(file, 'utf8')); } catch {}
settings.env = {
  ...(settings.env || {}),
  FLATKEY_API_KEY: process.env.FLATKEY_API_KEY,
  ANTHROPIC_BASE_URL: process.env.FLATKEY_BASE_URL,
  ANTHROPIC_AUTH_TOKEN: process.env.FLATKEY_API_KEY,
  ANTHROPIC_API_KEY: ''
};
fs.writeFileSync(file, `${JSON.stringify(settings, null, 2)}\n`);
NODE
else
  mkdir -p "$HOME/.codex"
  cat > "$HOME/.codex/config.toml" <<EOF
model_provider = "flatkey"
model = "gpt-5.5"

[model_providers.flatkey]
name = "Flatkey"
base_url = "https://router.flatkey.ai/v1"
env_key = "FLATKEY_API_KEY"
EOF
fi

echo ""
echo -e "${GREEN}Done.${NC} Restart your terminal or run: source $RC_FILE"
if [ "$AGENT" = "claude" ]; then
  echo "Start Claude Code with: claude"
else
  echo "Start Codex CLI with: codex"
fi
