import { defineConfig } from '@vben/vite-config';

export default defineConfig(async () => {
  return {
    application: {},
    vite: {
      server: {
        proxy: {
          '/api': {
            changeOrigin: true,
            rewrite: (path) => path.replace(/^\/api/, ''),
            // The Grove API listens on :8080 in local development. Keep the
            // /api prefix in the upstream request so /api/v1 and /console/v1
            // remain visible to the backend router.
            target: 'http://127.0.0.1:8080',
            ws: true,
          },
        },
      },
    },
  };
});
