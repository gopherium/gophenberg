import { defineConfig } from 'vitest/config'

export default defineConfig({
	test: {
		environment: 'node',
		include: ['test/*.test.ts'],
		coverage: {
			include: ['*.ts', 'components/**/*.ts'],
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
