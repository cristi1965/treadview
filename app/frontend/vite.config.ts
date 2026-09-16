import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  base: '/',
  server: {
    port: 5173,
    host: true,
    // Keep local verification same-origin while allowing an isolated backend port.
    // Production/Wails remains same-origin and does not use this proxy.
    proxy: {
      '/api': {
        target: process.env.VITE_DEV_API_TARGET || 'http://localhost:8765',
        changeOrigin: true,
      },
      '/ws': {
        target: (process.env.VITE_DEV_API_TARGET || 'http://localhost:8765').replace(/^http/, 'ws'),
        ws: true,
      },
    },
  },
})
