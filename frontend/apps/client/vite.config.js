import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Configuração Vite para o App Cliente CatBrasil (Porta 3000)
export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 3000,
    strictPort: true,
    host: true,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true, ws: true },
      '/ws': { target: 'http://127.0.0.1:8080', changeOrigin: true, ws: true }
    }
  },
  preview: {
    port: 3000
  }
});
