import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const API_TARGET = process.env.VITE_DEV_API_TARGET || 'http://localhost:8443'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: API_TARGET,
        changeOrigin: true,
        secure: false,
        // Long-running /files/stream downloads must not time out mid-transfer.
        timeout: 0,
        proxyTimeout: 0,
      },
      '/ws': {
        target: API_TARGET.replace(/^http/, 'ws'),
        ws: true,
        changeOrigin: true,
        secure: false,
      },
    },
  },
})
