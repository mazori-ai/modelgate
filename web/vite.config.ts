import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      // GraphQL API (served from unified backend on port 8080)
      '/graphql': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        headers: {
          'X-Tenant': 'default',  // Always send default tenant header
        },
      },
    },
  },
})
