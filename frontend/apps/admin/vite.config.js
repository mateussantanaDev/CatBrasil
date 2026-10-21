import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Configuração Vite para o App Gestão / ERP CatBrasil (Porta 3002)
export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 3002,
    strictPort: true,
    host: true,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true, ws: true },
      '/ws': { target: 'http://127.0.0.1:8080', changeOrigin: true, ws: true }
    }
  },
  preview: {
    port: 3002
  }
});
