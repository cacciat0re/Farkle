import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: '::',
    proxy: {
      '/api': 'http://localhost:8086',
      '/ws': {
        target: 'ws://localhost:8086',
        ws: true,
      },
    },
  },
})
