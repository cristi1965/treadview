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
        
        // Apple dark-mode system palette (calendar + flash surfaces stay pure dark)
        'ios-bg': '#000000',
        'ios-card': '#1C1C1E',
        'ios-card-2': '#2C2C2E',
        'ios-fill': 'rgba(118,118,128,0.24)',
        'ios-fill-2': '#636366',
        'ios-hairline': 'rgba(255,255,255,0.08)',
        'ios-sep': 'rgba(84,84,88,0.65)',
        'ios-label': '#FFFFFF',
        'ios-label-2': 'rgba(235,235,245,0.60)',
        'ios-label-3': 'rgba(235,235,245,0.30)',
        'ios-green': '#30D158',
        'ios-red': '#FF453A',
        'ios-orange': '#FF9F0A',
        'ios-blue': '#0A84FF',

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
        sans: ["-apple-system", "BlinkMacSystemFont", "\"SF Pro Text\"", "\"PingFang SC\"", "system-ui", "sans-serif"],
        mono: ["\"SF Mono\"", "\"Spline Sans Mono\"", "\"JetBrains Mono\"", "Menlo", "monospace"],
      },
      borderRadius: {
        ios: '14px',
        'ios-lg': '16px',
        'ios-sm': '8px',
      },
    },
  },
  plugins: [],
}
