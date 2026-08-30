import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';

export default defineConfig({
	plugins: [tailwindcss(), ...svelte()],
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'happy-dom',
		setupFiles: ['./src/test/setup.ts'],
		globals: true,
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json', 'lcov', 'html'],
			exclude: ['node_modules/', 'src/**/*.test.ts', 'src/**/*.spec.ts']
		}
	},
	resolve: {
		conditions: ['browser', 'svelte'],
		alias: {
			$lib: path.resolve('./src/lib'),
			'$app/paths': path.resolve('./src/test/app-paths.ts'),
			'$app/navigation': path.resolve('./src/test/app-navigation.ts'),
			'$app/state': path.resolve('./src/test/app-state.ts')
		}
	}
});
