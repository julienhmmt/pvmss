import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { paraglideVitePlugin } from '@inlang/paraglide-js';
import { defineConfig } from 'vite';

const backendUrl = process.env.VITE_BACKEND_URL ?? 'http://localhost:50000';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit(),
		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide',
			emitTsDeclarations: true
		})
	],
	server: {
		host: '0.0.0.0',
		port: 5173,
		strictPort: true,
		// Allow any hostname in development so the server is agnostic of the URL
		// used to reach it (localhost, web-dev.pvmss.orb.local, or any other
		// internal DNS/hosts entry).
		allowedHosts: true,
		// Public origin used for asset URLs and HMR base when behind a proxy.
		// Set ORIGIN=https://web-dev.pvmss.orb.local (no port) for OrbStack TLS.
		origin: process.env.ORIGIN || 'http://localhost:5173',
		watch: {
			// Polling is required inside Docker bind-mount containers where
			// inotify doesn't fire reliably for host-side file changes.
			usePolling: true,
			interval: 1000
		},
		hmr: {
			// Do NOT set hmr.host — it must be a string hostname; passing true
			// stringifies to "true" and the browser tries wss://true/ (DNS
			// failure). Omitting it lets the Vite client infer the host from
			// the page URL, so HMR works for any proxy/OrbStack URL.
			port: 5173,
			// When ORIGIN is set (e.g. https://web-dev.pvmss.orb.local for
			// OrbStack TLS termination on standard port), emit protocol +
			// clientPort derived from it. Vite client URL selection order:
			// clientPort > hmr.port > server.port.
			...(process.env.ORIGIN
				? (() => {
						try {
							const u = new URL(process.env.ORIGIN);
							const isHttps = u.protocol === 'https:';
							const explicitPort = u.port
								? Number(u.port)
								: isHttps
									? 443
									: 80;
							return {
								protocol: isHttps ? ('wss' as const) : ('ws' as const),
								clientPort: explicitPort
							};
						} catch {
							return {};
						}
					})()
				: {})
		},
		proxy: {
			// The web app uses same-origin /api/v1/* fetches. In dev, Vite
			// proxies these to the Go backend. changeOrigin is intentionally
			// false: the server's WebSocket origin check (vm_console.go)
			// compares the Origin header's host to r.Host. Preserving the
			// browser's Host header keeps them in sync; rewriting it would
			// cause a mismatch and reject the console WebSocket.
			'/api': {
				target: backendUrl,
				ws: true
			}
		}
	}
});
