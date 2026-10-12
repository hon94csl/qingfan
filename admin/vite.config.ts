import { defineConfig } from 'vite';
export default defineConfig({ server: { proxy: { '/healthz': 'http://127.0.0.1:8090' } } });
