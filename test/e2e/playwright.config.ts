// SPDX-License-Identifier: Apache-2.0

import { defineConfig, devices } from '@playwright/test'

import { authFile, baseURL, mailURL, proofAuthFile, repoRoot, smtpPort } from './env'

/** How each server under test starts, waits for its first answer and stops. */
const served = {
	cwd: repoRoot,
	reuseExistingServer: !process.env.CI,
	timeout: 180_000,
	gracefulShutdown: { signal: 'SIGTERM', timeout: 10_000 },
} as const

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
		{ name: 'role-setup', testMatch: '**/roles.setup.ts', dependencies: ['setup'] },
		{
			name: 'chromium',
			testIgnore: '**/proof-locale.spec.ts',
			use: { ...devices['Desktop Chrome'], storageState: authFile },
			dependencies: ['setup', 'role-setup'],
		},
		{
			name: 'proof',
			testMatch: '**/proof-locale.spec.ts',
			use: { ...devices['Desktop Chrome'], storageState: proofAuthFile },
			dependencies: ['proof-setup'],
		},
	],
	webServer: [
		{ ...served, command: 'make e2e-serve', url: baseURL },
		{
			...served,
			command: 'make e2e-serve-mail',
			url: mailURL,
			env: { E2E_MAIL_ADDR: new URL(mailURL).host, E2E_MAIL_URL: mailURL, E2E_SMTP_PORT: String(smtpPort) },
		},
	],
})
