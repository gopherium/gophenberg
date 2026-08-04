/// <reference types="vitest/config" />
import { getViteConfig } from 'astro/config'

export default getViteConfig({
	test: {
		environment: 'node',
		include: ['test/*.test.ts'],
		coverage: {
			include: ['**/*.ts'],
			exclude: ['**/*.d.ts', '**/test/**', '**/node_modules/**'],
			reporter: ['text'],
			thresholds: {
				statements: 100,
				branches: 100,
				functions: 100,
				lines: 100,
			},
		},
	},
})
