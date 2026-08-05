#!/usr/bin/env bash
set -euo pipefail

VITE_API_BASE_URL=http://127.0.0.1:8765 npm run build

dest="../backend/cmd/desktop/frontend/dist"
mkdir -p "$dest"
find "$dest" -mindepth 1 ! -name '.gitkeep' -exec rm -rf {} +
cp -R dist/. "$dest"/
