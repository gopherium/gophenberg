// SPDX-License-Identifier: Apache-2.0

/// <reference types="vitest/config" />
import { godminDedupe, godminSingleCopy } from '@gopherium/godmin/vite'
import react from '@vitejs/plugin-react'
import dsTokenFallbacks from '@wordpress/theme/vite-plugins/vite-ds-token-fallbacks'
import { defineConfig } from 'vite'

export default defineConfig({
	plugins: [react(), dsTokenFallbacks(), godminSingleCopy()],
	resolve: {
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
		css: { include: [/index\.css$/] },
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
			exclude: ['src/main.tsx', '**/test/**', '**/node_modules/**'],
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
