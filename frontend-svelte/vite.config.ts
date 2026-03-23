import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		host: true,
		proxy: {
			'/api': { target: process.env.VITE_BACKEND_URL ?? 'http://localhost:50000', changeOrigin: true },
			'/login': { target: process.env.VITE_BACKEND_URL ?? 'http://localhost:50000', changeOrigin: true },
			'/logout': { target: process.env.VITE_BACKEND_URL ?? 'http://localhost:50000', changeOrigin: true },
			'/css': { target: process.env.VITE_BACKEND_URL ?? 'http://localhost:50000', changeOrigin: true },
			'/js': { target: process.env.VITE_BACKEND_URL ?? 'http://localhost:50000', changeOrigin: true },
			'/vendor': { target: process.env.VITE_BACKEND_URL ?? 'http://localhost:50000', changeOrigin: true }
		}
	}
});
