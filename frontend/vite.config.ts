// SPDX-License-Identifier: Apache-2.0

/// <reference types="vitest/config" />
import { godminDedupe, godminSingleCopy } from '@gopherium/godmin/vite'
import react from '@vitejs/plugin-react'
import dsTokenFallbacks from '@wordpress/theme/vite-plugins/vite-ds-token-fallbacks'
import { defineConfig } from 'vite'
import { fileURLToPath } from 'node:url'

import { adminBasepath } from './src/basepath.js'

// The workers @wordpress/upload-media reaches for, stubbed out until the media cycle ships.
const mediaWorkerStubs = {
	'@wordpress/vips/worker': fileURLToPath(new URL('./stubs/vips-worker.ts', import.meta.url)),
	'@wordpress/video-conversion/worker': fileURLToPath(
		new URL('./stubs/video-conversion-worker.ts', import.meta.url),
	),
}

export default defineConfig({
	base: adminBasepath + '/',
	plugins: [react(), dsTokenFallbacks(), godminSingleCopy()],
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
			'/api': process.env.GOPHENBERG_API || 'http://localhost:8081',
		},
	},
	test: {
		environment: 'jsdom',
		env: { TZ: 'UTC' },
		css: { include: [/index\.css$/, /src\/posts\/editor\.css$/] },
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
				'../sdk/frontend/**/*.{ts,tsx}',
				'../plugins/*/frontend/**/*.{ts,tsx}',
			],
			exclude: ['src/main.tsx', '**/*.d.ts', '**/test/**', '**/node_modules/**'],
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
