/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // StockGod.xyz theme variables
        base: '#08090b',
        surface: '#111317',
        'surface-2': '#181b21',
        'surface-3': '#21242c',
        line: '#ebeef512',
        'line-2': '#ebeef521',
        accent: '#d98a6a',
        ink: '#ecedf0',
        muted: '#8e919b',
        faint: '#585b66',
        up: '#2ebd85',
        down: '#f6465d',
        
        // Original Tailwind-compatible mapping
        background: '#08090b',
        foreground: '#ecedf0',
        primary: {
          DEFAULT: '#d98a6a',
          foreground: '#ffffff',
        },
        secondary: {
          DEFAULT: '#181b21',
          foreground: '#ecedf0',
        },
      },
      fontFamily: {
        sans: ["-apple-system", "\"SF Pro Text\"", "\"PingFang SC\"", "system-ui", "sans-serif"],
        mono: ["\"Spline Sans Mono\"", "\"SF Mono\"", "\"JetBrains Mono\"", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
}
