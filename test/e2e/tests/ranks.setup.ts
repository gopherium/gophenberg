// SPDX-License-Identifier: Apache-2.0

import { expect, test as setup } from '@playwright/test'

import { authorAuthFile, editorAuthFile, seededRanks } from '../env'

setup('logs in as the seeded editor and stores the session', async ({ page }) => {
	await page.goto('/admin/')

	await page.getByLabel('Email').fill(seededRanks.editor.email)
	await page.getByLabel('Password').fill(seededRanks.editor.password)
	await page.getByRole('button', { name: 'Log in' }).click()

	await expect(page.getByText('Welcome to Gophenberg.')).toBeVisible()
	await page.context().storageState({ path: editorAuthFile })
})

setup('logs in as the seeded author and stores the session', async ({ page }) => {
	await page.goto('/admin/')

	await page.getByLabel('Email').fill(seededRanks.author.email)
	await page.getByLabel('Password').fill(seededRanks.author.password)
	await page.getByRole('button', { name: 'Log in' }).click()

	await expect(page.getByText('Welcome to Gophenberg.')).toBeVisible()
	await page.context().storageState({ path: authorAuthFile })
})
