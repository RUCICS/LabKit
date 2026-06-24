import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

const apiProxyTarget = process.env.LABKIT_WEB_API_PROXY ?? 'http://127.0.0.1:8083';

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: apiProxyTarget,
        changeOrigin: true
      },
      // Browser-only 微人大 login start endpoint lives on the API under /auth.
      '/auth/login': {
        target: apiProxyTarget,
        changeOrigin: true
      }
    }
  },
  test: {
    environment: 'jsdom',
    globals: true
  }
});
