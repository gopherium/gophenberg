// SPDX-License-Identifier: Apache-2.0

import { fileURLToPath } from 'node:url'

export const repoRoot = fileURLToPath(new URL('../..', import.meta.url))

export const authFile = fileURLToPath(new URL('.auth/user.json', import.meta.url))

export const baseURL = process.env.GOPHENBERG_E2E_URL ?? 'http://localhost:8081'

export const credentials = {
	email: process.env.GOPHENBERG_E2E_EMAIL ?? 'e2e@example.com',
	password: process.env.GOPHENBERG_E2E_PASSWORD ?? 'correct horse battery',
	name: process.env.GOPHENBERG_E2E_NAME ?? 'Grace Hopper',
}
