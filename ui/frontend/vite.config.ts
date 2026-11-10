import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

const here = (p: string) => fileURLToPath(new URL(p, import.meta.url));

// `npm run dev` proxies the API to the web demo (go run ./cmd/pinky-web-demo
// -token devtoken), or to PINKY_API with PINKY_TOKEN.
const api = process.env.PINKY_API ?? 'http://127.0.0.1:7466';
const token = process.env.PINKY_TOKEN ?? 'devtoken';

// The catalogs carry descriptions for translators ("@key" entries) that
// the client never reads: leave them out of the bundle.
const catalogs = {
  name: 'pinky-catalogs',
  enforce: 'pre' as const,
  transform(code: string, id: string) {
    if (!/cmd\/pinky\/locales\/[^/]+\.json$/.test(id)) return null;
    const all = JSON.parse(code) as Record<string, unknown>;
    const kept: Record<string, string> = {};
    for (const [k, v] of Object.entries(all)) if (!k.startsWith('@') && typeof v === 'string') kept[k] = v;
    return { code: JSON.stringify(kept), map: null };
  },
};

export default defineConfig({
  plugins: [catalogs, react()],
  resolve: {
    alias: {
      '@': here('./src'),
      '@locales': here('../../cmd/pinky/locales'),
    },
  },
  server: {
    port: 5173,
    fs: { allow: [here('../..')] },
    proxy: {
      '/api': {
        target: api,
        changeOrigin: true,
        headers: { 'X-Pinky-Token': token },
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
  },
});
