// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

export const repoRoot = fileURLToPath(new URL('../..', import.meta.url))

export const authFile = fileURLToPath(new URL('.auth/user.json', import.meta.url))

const starterManifest = JSON.parse(
	readFileSync(fileURLToPath(new URL('../theme/theme.json', import.meta.url)), 'utf8'),
) as { name: string; version: string }

export const starterTheme = { name: starterManifest.name, version: starterManifest.version }

export const uploadedTheme = { name: 'driftwood', version: '9.9.9' }

export const uploadArchive = fileURLToPath(
	new URL(`../../.e2e-archive/${uploadedTheme.name}.zip`, import.meta.url),
)

export const baseURL = process.env.GOPHENBERG_E2E_URL ?? 'http://localhost:8081'

export const credentials = {
	email: process.env.GOPHENBERG_E2E_EMAIL ?? 'e2e@example.com',
	password: process.env.GOPHENBERG_E2E_PASSWORD ?? 'correct horse battery',
	name: process.env.GOPHENBERG_E2E_NAME ?? 'Grace Hopper',
}
