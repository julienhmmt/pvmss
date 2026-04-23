import { sveltekit } from "@sveltejs/kit/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import path from "path";

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  resolve: {
    alias: {
      $lib: path.resolve("./src/lib"),
    },
  },
  server: {
    host: "0.0.0.0",
    port: 5173,
    strictPort: true,
    allowedHosts: [
      "localhost",
      ".orb.local",
    ],
    watch: {
      usePolling: true,
      interval: 1000,
    },
    hmr: {
      host: "localhost",
      port: 5173,
      clientPort: 5173,
    },
    proxy: {
      "/api": {
        target: process.env.VITE_BACKEND_URL ?? "http://localhost:50000",
        changeOrigin: true,
        ws: true,
        configure: (proxy) => {
          // Forward the original Host as X-Forwarded-Host so the backend
          // WebSocket origin check can validate the browser's Origin header
          // against the frontend dev server's hostname rather than the
          // proxied backend hostname.
          proxy.on("proxyReqWs", (proxyReq, req) => {
            const host = req.headers.host;
            if (host) {
              proxyReq.setHeader("X-Forwarded-Host", host);
            }
          });
        },
      },
    },
  },
});
