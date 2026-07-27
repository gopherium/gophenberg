// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

import { credentials } from '../env'

test('serves the admin shell to a stored session', async ({ page }) => {
	await page.goto('/')

	await expect(page.getByRole('heading', { name: 'Gophenberg' })).toBeVisible()
	await expect(page.getByText('Welcome to Gophenberg.')).toBeVisible()
	await expect(page.getByText(credentials.name)).toBeVisible()
	await expect(page.getByRole('link', { name: 'Users' })).toBeVisible()
})

test('reports the application version to a stored session', async ({ page }) => {
	const response = await page.request.get('/api/version')

	expect(response.status()).toBe(200)
	expect((await response.json()).version).toMatch(/^\d+\.\d+\.\d+$/)
})
