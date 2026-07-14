import path from 'node:path'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig, loadEnv } from 'vite'

const matchesNodeModulePackage = (id: string, packageNames: string[]) => {
  const normalizedId = id.replace(/\\/g, '/')
  return packageNames.some(packageName => normalizedId.includes(`/node_modules/${packageName}/`))
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiMode = env.VITE_API_MODE
  const apiBaseURL = env.VITE_API_BASE_URL
  const enableMock = env.VITE_ENABLE_MOCK
  if (mode === 'production' && enableMock === 'true') {
    throw new Error('Production frontend builds must not set VITE_ENABLE_MOCK=true.')
  }
  if (mode === 'production' && apiMode !== 'real' && !apiBaseURL) {
    throw new Error('Production frontend builds must set VITE_API_MODE=real or VITE_API_BASE_URL to avoid mock/demo fallback.')
  }

  return {
    plugins: [vue(), tailwindcss()],
    build: {
      rolldownOptions: {
        output: {
          codeSplitting: {
            groups: [
              {
                name: 'vendor-framework',
                test: id => matchesNodeModulePackage(id, [
                  'vue',
                  'vue-router',
                  'pinia',
                  '@tanstack/query-core',
                  '@tanstack/vue-query',
                ]),
                priority: 30,
                minSize: 20 * 1024,
              },
              {
                name: 'vendor-ui',
                test: id => matchesNodeModulePackage(id, [
                  '@floating-ui/core',
                  '@floating-ui/dom',
                  '@floating-ui/utils',
                  '@radix-icons/vue',
                  '@vueuse/core',
                  'lucide-vue-next',
                  'reka-ui',
                  'vue-sonner',
                ]),
                priority: 20,
                minSize: 20 * 1024,
              },
              {
                name: 'vendor-content',
                test: id => matchesNodeModulePackage(id, ['dompurify', 'marked']),
                priority: 20,
                minSize: 20 * 1024,
              },
              {
                name: 'vendor-charts',
                test: id => matchesNodeModulePackage(id, ['@unovis/ts', '@unovis/vue']),
                priority: 20,
                minSize: 20 * 1024,
                maxSize: 450 * 1024,
              },
            ],
          },
        },
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      // 本机通过固定 Cloudflare Tunnel 域名预览时，仅允许项目正式域名访问开发服务器。
      allowedHosts: ['c2cmarket.shop'],
      proxy: {
        // 仅代理真实后端 API，避免误拦截 /api-market 这类前端 history 路由。
        '^/api(?:/|\\?|$)': {
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
