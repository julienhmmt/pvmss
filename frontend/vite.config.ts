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
    // Allow any hostname in development so the server is agnostic of the URL used to reach it
    // (e.g. localhost, frontend-dev.pvmss.orb.local, or any other internal DNS/hosts).
    allowedHosts: true,
    // Public origin used for asset URLs and HMR base when behind a proxy.
    // Set ORIGIN=https://frontend-dev.pvmss.orb.local (no port) for OrbStack TLS access.
    origin: process.env.ORIGIN || "http://localhost:5173",
    watch: {
      usePolling: true,
      interval: 1000,
    },
    hmr: {
      // Do NOT set `hmr.host`. It must be a string hostname; passing `true`
      // stringifies to "true" and the browser tries `wss://true/` (DNS failure).
      // Omitting it makes the Vite client infer the host from the page URL, so
      // HMR works for any proxy/OrbStack URL (localhost, *.orb.local, etc.).
      // Internal HMR WebSocket listener port inside the container (always 5173).
      // The client-visible port is overridden below via clientPort when ORIGIN is set.
      port: 5173,
      // When ORIGIN is set (e.g. https://frontend-dev.pvmss.orb.local for OrbStack TLS
      // termination on standard port), also emit protocol + clientPort derived from it.
      // Vite client URL selection order: clientPort > hmr.port > server.port.
      // clientPort forces the browser to connect to the external port (443/80 or explicit)
      // instead of falling back to the internal 5173, which has no listener on the public hostname.
      ...(process.env.ORIGIN
        ? (() => {
            try {
              const u = new URL(process.env.ORIGIN);
              const isHttps = u.protocol === "https:";
              const explicitPort = u.port
                ? Number(u.port)
                : isHttps
                  ? 443
                  : 80;
              return {
                protocol: isHttps ? "wss" : "ws",
                clientPort: explicitPort,
              };
            } catch {
              return {};
            }
          })()
        : {}),
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
