import type { Config } from 'tailwindcss'

export default {
  content: [
    './index.html',
    './src/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        obs: {
          bg:       '#080c10',
          surface:  '#0d1117',
          surface2: '#111820',
          border:   '#1e2d3d',
          border2:  '#243447',
          accent:   '#00d4ff',
          accent2:  '#7b61ff',
          green:    '#00e676',
          orange:   '#ffab40',
          red:      '#ff4f6a',
          yellow:   '#ffd740',
          text:     '#e8f1ff',
          muted:    '#4a6080',
          muted2:   '#6b8ba8',
        },
      },
      fontFamily: {
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
        syne: ['Syne', 'ui-sans-serif', 'sans-serif'],
      },
      fontSize: {
        '2xs': ['10px', { lineHeight: '14px' }],
      },
      keyframes: {
        'pulse-dot': {
          '0%, 100%': { opacity: '1', boxShadow: '0 0 4px #00e676' },
          '50%':       { opacity: '0.4', boxShadow: 'none' },
        },
        'fade-in-up': {
          from: { opacity: '0', transform: 'translateY(12px)' },
          to:   { opacity: '1', transform: 'translateY(0)' },
        },
      },
      animation: {
        'pulse-dot':  'pulse-dot 2s infinite',
        'fade-in-up': 'fade-in-up 0.4s ease both',
      },
      borderColor: {
        DEFAULT: '#1e2d3d',
      },
    },
  },
  plugins: [],
} satisfies Config
