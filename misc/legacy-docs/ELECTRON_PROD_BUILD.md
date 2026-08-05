# Electron Production Build Guide

## Overview

This guide explains how to run TradingAgents in production mode using Electron. The production build:

- ✅ Uses the built React frontend (from `dist/` folder)
- ✅ Runs the Go backend binary (from `../backend/main`)
- ✅ Automatically starts both backend and frontend in a single Electron window
- ✅ Properly handles process lifecycle (cleanup on exit)

## Prerequisites

1. **Backend Binary**: Build the Go backend first
   ```bash
   cd app/backend
   go build -o main
   ```

2. **Frontend Build**: Build the React frontend
   ```bash
   cd app/frontend
   npm install
   npm run build
   ```

3. **Copy Frontend to Electron**: Copy the built files
   ```bash
   cd app/electron
   rm -rf dist
   cp -r ../frontend/dist ./dist
   ```

## Running Production Build

### Method 1: Using npm script (Recommended)

```bash
cd app/electron
npm run prod
```

This command:
1. Builds the frontend (`npm run build:frontend`)
2. Copies the built files to `app/electron/dist/`
3. Starts Electron in production mode

### Method 2: Using the shell script

```bash
cd app/electron
./start-prod.sh
```

This is a simpler wrapper that:
- Checks if `dist/` and backend binary exist
- Starts Electron with `NODE_ENV=production`

### Method 3: Manual execution

```bash
cd app/electron
NODE_ENV=production npx electron .
```

## How It Works

### 1. Backend Process

The Electron `main.js` automatically spawns the Go backend process:

```javascript
// In main.js
function startBackend() {
  const binaryName = isWindows ? 'main.exe' : 'main'
  
  if (app.isPackaged) {
    backendPath = path.join(process.resourcesPath, binaryName)
  } else {
    backendPath = path.join(__dirname, '..', 'backend', binaryName)
  }
  
  backendProcess = spawn(backendPath, [], {
    cwd: runCwd,
    env: { ...process.env, PORT: '8765' }
  })
}
```

### 2. Frontend Loading

In production mode (`NODE_ENV=production` or `app.isPackaged`):

```javascript
// In main.js
const isDev = process.env.NODE_ENV !== 'production' && !app.isPackaged;

if (isDev) {
  mainWindow.loadURL('http://localhost:5173')  // Development server
} else {
  mainWindow.loadFile(path.join(__dirname, 'dist', 'index.html'))  // Static files
}
```

### 3. Process Cleanup

When Electron exits, it automatically kills the backend process:

```javascript
app.on('will-quit', () => {
  if (backendProcess) {
    backendProcess.kill()
  }
})
```

## Verification

After running `npm run prod`, you should see:

1. **Backend logs**:
   ```
   [Backend STDERR]: 2026/07/04 01:05:10 main.go:16: TradingAgents Go Backend starting...
   [Backend STDERR]: 2026/07/04 01:05:10 db.go:27: SQLite database successfully initialized at: trades.db
   [Backend STDERR]: 2026/07/04 01:05:10 main.go:41: Server listening on :8765
   ```

2. **Electron connection**:
   ```
   [Electron] Connected to Go backend!
   ```

3. **Electron window**: A native desktop window opens showing the TradingAgents UI

## Troubleshooting

### Error: "Port 8765 already in use"

Stop the development server first:
```bash
# Find the process using port 8765
lsof -i :8765

# Kill it (replace PID with actual process ID)
kill <PID>
```

### Error: "dist/ not found"

Build the frontend first:
```bash
cd app/electron
npm run build:frontend
```

### Error: "Backend binary not found"

Build the Go backend:
```bash
cd app/backend
go build -o main
```

### Electron window shows blank screen

Check the developer console (View → Toggle Developer Tools) for errors. Common issues:
- Missing static files in `dist/`
- API requests failing (backend not running)
- CORS issues (should not happen in Electron)

## Development vs Production

| Mode | Frontend | Backend | Electron |
|------|----------|---------|----------|
| **Development** | Vite dev server (`:5173`) | Manual start (`go run .`) | `npm start` or `electron .` |
| **Production** | Static files (`dist/`) | Auto-started binary | `npm run prod` |

## Building Distributable Packages

To create a `.dmg` (macOS) or `.exe` (Windows) installer:

```bash
cd app/electron
npm run package
```

This will:
1. Build the frontend
2. Package Electron with the frontend and backend
3. Create an installer in `app/electron/release/`

**Important**: Ensure the backend binary is built for the target platform:
- macOS: `GOOS=darwin GOARCH=arm64 go build -o main`
- Windows: `GOOS=windows GOARCH=amd64 go build -o main.exe`

## File Structure

```
app/electron/
├── main.js              # Electron main process
├── preload.js           # Preload script (security)
├── package.json         # Electron config and scripts
├── start-prod.sh        # Production startup script
├── dist/                # Built frontend (copied from ../frontend/dist)
│   ├── index.html
│   └── assets/
│       ├── index-*.js
│       └── index-*.css
└── release/             # Built installers (after npm run package)
    ├── TradingAgents-1.0.0-arm64.dmg (macOS)
    └── TradingAgents Setup 1.0.0.exe (Windows)
```

## Next Steps

- ✅ Production build works (`npm run prod`)
- ✅ Frontend loads from static files
- ✅ Backend auto-starts and connects
- 🔄 Create distributable package (`npm run package`)
- 🔄 Test on different platforms (macOS, Windows, Linux)

## Notes

- The production build uses the same database file (`trades.db`) as development
- Environment variables from `.env` are still loaded by the Go backend
- Electron's `NODE_ENV` only affects the frontend loading logic, not the backend
- The backend always runs on port 8765 (hardcoded in `main.js`)
