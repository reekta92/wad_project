import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [tailwindcss()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: `http://${process.env.API_ADDRESS || '127.0.0.1:8080'}` },
    },
  },
});
