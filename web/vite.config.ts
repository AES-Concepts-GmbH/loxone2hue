// vitest/config re-exports Vite's defineConfig with the `test` block typed.
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: './',
  define: {
    __APP_VERSION__: JSON.stringify(process.env.VITE_APP_VERSION || 'dev'),
    __BUILD_DATE__: JSON.stringify(process.env.VITE_BUILD_DATE || 'unknown'),
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
  test: {
    // Pure helper functions only; nothing here needs a DOM.
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
