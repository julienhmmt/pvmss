import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': { target: 'http://localhost:50000', changeOrigin: true },
			'/login': { target: 'http://localhost:50000', changeOrigin: true },
			'/logout': { target: 'http://localhost:50000', changeOrigin: true },
			'/css': { target: 'http://localhost:50000', changeOrigin: true },
			'/js': { target: 'http://localhost:50000', changeOrigin: true },
			'/vendor': { target: 'http://localhost:50000', changeOrigin: true }
		}
	}
});
