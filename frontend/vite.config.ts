import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { fileURLToPath } from 'url';
import path from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 3000
	},
	resolve: {
		alias: {
			$lib: path.resolve(__dirname, './src/lib'),
			$components: path.resolve(__dirname, './src/lib/components')
		}
	}
});
