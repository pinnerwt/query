import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [preact(), tailwindcss()],
  base: '/app/',
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/r': 'http://localhost:8080',
    },
  },
})
