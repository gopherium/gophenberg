// SPDX-License-Identifier: Apache-2.0

/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import dsTokenFallbacks from '@wordpress/theme/vite-plugins/vite-ds-token-fallbacks'
import { defineConfig } from 'vite'

export default defineConfig({
	plugins: [react(), dsTokenFallbacks()],
	resolve: {
		dedupe: [
			'react',
			'react-dom',
			'@tanstack/react-query',
			'@tanstack/react-router',
			'@wordpress/theme',
			'@wordpress/ui',
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
