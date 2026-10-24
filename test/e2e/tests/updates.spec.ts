// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

test('an administrator reaches the Updates screen from the sidebar and sees the version that runs', async ({ page }) => {
	const answered = await page.request.get('/api/version')
	expect(answered.ok()).toBe(true)
	const { version } = (await answered.json()) as { version: string }

	await page.goto('/admin/')
	await page.getByRole('region', { name: 'Navigation' }).getByRole('link', { name: /^Updates/ }).click()

	await expect(page).toHaveURL(/\/admin\/update-core$/)
	await expect(page.getByRole('heading', { level: 2, name: 'Gophenberg Updates' })).toBeVisible()
	await expect(page.getByText(`Current version: ${version}`)).toBeVisible()
	await expect(page.getByText('Gophenberg has not checked for updates yet.')).toBeVisible()
	await expect(page.getByText('This site does not check for updates on its own.')).toBeVisible()
})
