import { defineConfig } from 'vite';

export default defineConfig({
  appType: 'mpa',
  build: {
    outDir: '../web-dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        admin: new URL('./admin/index.html', import.meta.url).pathname,
        agent: new URL('./agent/index.html', import.meta.url).pathname
      }
    }
  }
});
