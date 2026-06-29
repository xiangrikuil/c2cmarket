import path from 'node:path'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiMode = env.VITE_API_MODE
  const apiBaseURL = env.VITE_API_BASE_URL
  if (mode === 'production' && apiMode !== 'real' && !apiBaseURL) {
    throw new Error('Production frontend builds must set VITE_API_MODE=real or VITE_API_BASE_URL to avoid mock/demo fallback.')
  }

  return {
    plugins: [vue(), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      proxy: {
        '/api': {
          target: process.env.VITE_DEV_API_PROXY_TARGET ?? 'http://127.0.0.1:8080',
          changeOrigin: true,
        },
        '/health': {
          target: process.env.VITE_DEV_API_PROXY_TARGET ?? 'http://127.0.0.1:8080',
          changeOrigin: true,
        },
        '/readyz': {
          target: process.env.VITE_DEV_API_PROXY_TARGET ?? 'http://127.0.0.1:8080',
          changeOrigin: true,
        },
      },
    },
  }
})
