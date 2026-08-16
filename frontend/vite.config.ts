// SPDX-License-Identifier: Apache-2.0

/// <reference types="vitest/config" />
import { godminDedupe, godminSingleCopy, godminStylesheetFirst } from '@gopherium/godmin/vite'
import react from '@vitejs/plugin-react'
import dsTokenFallbacks from '@wordpress/theme/vite-plugins/vite-ds-token-fallbacks'
import { defineConfig } from 'vite'
import { fileURLToPath } from 'node:url'

import { adminBasepath } from './src/basepath.js'

const backend = process.env.GOPHENBERG_API || 'http://localhost:8081'

// The workers @wordpress/upload-media reaches for, stubbed out until the media cycle ships.
const mediaWorkerStubs = {
	'@wordpress/vips/worker': fileURLToPath(new URL('./stubs/vips-worker.ts', import.meta.url)),
	'@wordpress/video-conversion/worker': fileURLToPath(
		new URL('./stubs/video-conversion-worker.ts', import.meta.url),
	),
}

export default defineConfig({
	base: adminBasepath + '/',
	plugins: [react(), dsTokenFallbacks(), godminSingleCopy(), godminStylesheetFirst()],
	resolve: {
		alias: mediaWorkerStubs,
		dedupe: [
			...godminDedupe,
			'@tanstack/react-query',
			'@tanstack/react-router',
		],
	},
	server: {
		port: 5174,
		proxy: {
			'/api': backend,
			'/media': backend,
		},
	},
	test: {
		environment: 'jsdom',
		env: { TZ: 'UTC' },
		css: { include: [/index\.css$/, /src\/content\/editor\.css$/, /src\/media\/media\.css$/] },
		hookTimeout: 120000,
		server: { deps: { inline: [/@wordpress\//] } },
		setupFiles: ['./src/test/setup.ts'],
		include: [
			'src/**/*.test.{ts,tsx}',
			'../sdk/frontend/test/*.test.{ts,tsx}',
			'../plugins/*/frontend/test/*.test.{ts,tsx}',
		],
		coverage: {
			include: [
				'src/**',
				'scripts/**/*.ts',
				'../sdk/frontend/**/*.{ts,tsx}',
				'../plugins/*/frontend/**/*.{ts,tsx}',
			],
			exclude: [
				'src/main.tsx',
				'scripts/write-pot.ts',
				'scripts/write-catalogs.ts',
				'../sdk/frontend/scripts/build-site-assets.ts',
				'**/*.d.ts',
				'**/test/**',
				'**/node_modules/**',
			],
			allowExternal: true,
			reporter: ['text', 'lcov'],
			thresholds: {
				statements: 100,
				branches: 100,
				functions: 100,
				lines: 100,
			},
		},
	},
})
