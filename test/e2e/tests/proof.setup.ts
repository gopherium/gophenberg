// SPDX-License-Identifier: Apache-2.0

import { expect, test as setup } from '@playwright/test'

import { proofAuthFile, proofCredentials, proofLocale } from '../env'

setup('signs the proof reader in and stores their language', async ({ page }) => {
	await page.goto('/admin/')

	await page.getByLabel('Email').fill(proofCredentials.email)
	await page.getByLabel('Password').fill(proofCredentials.password)
	await page.getByRole('button', { name: 'Log in' }).click()

	await expect(page.getByText(proofCredentials.name)).toBeVisible()

	const stored = await page.request.patch('/api/locale', {
		data: { locale: proofLocale.locale },
	})
	expect(stored.ok()).toBe(true)

	await page.context().storageState({ path: proofAuthFile })
})
