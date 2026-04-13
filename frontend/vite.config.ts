import { sveltekit } from '@sveltejs/kit/vite';
import { svelteTesting } from '@testing-library/svelte/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit(), svelteTesting()],
	server: {
		host: '0.0.0.0',
		port: 5173,
		strictPort: false,
		watch: {
			ignored: ['**/node_modules/**']
		},
		hmr: {
			host: '0.0.0.0'
		},
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				changeOrigin: true
			},
			'/health': {
				target: 'http://localhost:8080',
				changeOrigin: true
			}
		}
	}
});
