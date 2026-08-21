// SPDX-License-Identifier: Apache-2.0

import { defineConfig, devices } from '@playwright/test'

import { authFile, baseURL, proofAuthFile, repoRoot } from './env'

export default defineConfig({
	testDir: './tests',
	fullyParallel: false,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: 1,
	reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
	use: {
		baseURL,
		timezoneId: 'UTC',
		trace: 'on-first-retry',
		screenshot: 'only-on-failure',
	},
	projects: [
		{ name: 'setup', testMatch: '**/auth.setup.ts' },
		{ name: 'proof-setup', testMatch: '**/proof.setup.ts' },
		{ name: 'rank-setup', testMatch: '**/ranks.setup.ts', dependencies: ['setup'] },
		{
			name: 'chromium',
			testIgnore: '**/proof-locale.spec.ts',
			use: { ...devices['Desktop Chrome'], storageState: authFile },
			dependencies: ['setup', 'rank-setup'],
		},
		{
			name: 'proof',
			testMatch: '**/proof-locale.spec.ts',
			use: { ...devices['Desktop Chrome'], storageState: proofAuthFile },
			dependencies: ['proof-setup'],
		},
	],
	webServer: {
		command: 'make e2e-serve',
		cwd: repoRoot,
		url: baseURL,
		reuseExistingServer: !process.env.CI,
		timeout: 180_000,
		gracefulShutdown: { signal: 'SIGTERM', timeout: 10_000 },
	},
})
