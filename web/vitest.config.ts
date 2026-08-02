import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';

export default defineConfig({
	plugins: [tailwindcss(), ...svelte()],
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'happy-dom',
		globals: true,
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json', 'html'],
			exclude: ['node_modules/', 'src/**/*.test.ts', 'src/**/*.spec.ts']
		}
	},
	resolve: {
		conditions: ['browser', 'svelte'],
		alias: {
			$lib: path.resolve('./src/lib')
		}
	}
});
