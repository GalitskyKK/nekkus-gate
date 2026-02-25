import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: process.env.VITE_OUT_DIR || '../ui/frontend/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5174,
    proxy: {
      '/api': { target: 'http://127.0.0.1:9003', changeOrigin: true },
      '/ws': { target: 'http://127.0.0.1:9003', ws: true },
    },
  },
})
