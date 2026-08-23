// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'

import { expect, test } from '@playwright/test'

import { staleArchive, staleManifest, staleTheme, starterTheme, uploadArchive, uploadedTheme } from '../env'

const READY = { timeout: 60_000 }

test.beforeEach(async ({ request }) => {
	const restored = await request.post('/api/themes/active', {
		data: { name: starterTheme.name },
		timeout: 60_000,
	})
	expect(restored.ok()).toBe(true)
})

test('installs a theme, serves the site through it, and rolls back', async ({ page }) => {
	await page.goto('/admin/themes')
	await expect(page.getByRole('heading', { level: 1, name: 'Themes' })).toBeVisible()
	await expect(
		page.getByText(`${starterTheme.name} ${starterTheme.version} is serving the public site.`),
	).toBeVisible()

	await page.getByLabel('Theme archive').setInputFiles(uploadArchive)
	await page.getByRole('button', { name: 'Install theme' }).click()

	const installed = page.getByRole('row', { name: new RegExp(uploadedTheme.name) })
	await expect(installed).toBeVisible(READY)
	await expect(installed.getByText(uploadedTheme.version)).toBeVisible()
	await expect(installed.getByText('Installed')).toBeVisible()

	await page.getByRole('button', { name: `Activate ${uploadedTheme.name}` }).click()

	await expect(
		page.getByText(`${uploadedTheme.name} ${uploadedTheme.version} is serving the public site.`),
	).toBeVisible(READY)
	await expect(installed.getByText('Serving')).toBeVisible()

	const themed = await page.request.get('/')
	expect(themed.status()).toBe(200)
	expect(await themed.text()).toContain('Gophenberg Starter')

	await page.getByRole('button', { name: `Roll back to ${starterTheme.name}` }).click()

	await expect(
		page.getByText(`${starterTheme.name} ${starterTheme.version} is serving the public site.`),
	).toBeVisible(READY)
	await expect(
		page.getByRole('row', { name: new RegExp(starterTheme.name) }).getByText('Serving'),
	).toBeVisible()

	const restored = await page.request.get('/')
	expect(restored.status()).toBe(200)
	expect(await restored.text()).toContain('Gophenberg Starter')
})

test('refuses a theme built on a kit this site does not serve', async ({ page }) => {
	const builtOn = (JSON.parse(readFileSync(staleManifest, 'utf8')) as { kit: string }).kit
	const handshake = await page.request.get('/api/content/v1')
	const served = ((await handshake.json()) as { kit: string[] }).kit

	await page.goto('/admin/themes')

	await page.getByLabel('Theme archive').setInputFiles(staleArchive)
	await page.getByRole('button', { name: 'Install theme' }).click()

	const refusal = page.getByRole('alert')
	await expect(refusal).toContainText(builtOn)
	for (const kit of served) {
		await expect(refusal).toContainText(kit)
	}
	await expect(page.getByRole('row', { name: new RegExp(staleTheme.name) })).toHaveCount(0)
	await expect(
		page.getByText(`${starterTheme.name} ${starterTheme.version} is serving the public site.`),
	).toBeVisible()

	const untouched = await page.request.get('/')
	expect(untouched.status()).toBe(200)
	expect(await untouched.text()).toContain('Gophenberg Starter')
})

test('refuses an archive that is not a theme and leaves the site alone', async ({ page }) => {
	await page.goto('/admin/themes')

	await page.getByLabel('Theme archive').setInputFiles({
		name: 'riverbed.zip',
		mimeType: 'application/zip',
		buffer: Buffer.from('this is not an archive'),
	})
	await page.getByRole('button', { name: 'Install theme' }).click()

	await expect(page.getByRole('alert')).toHaveText(
		'This file could not be opened as a zip. Package the theme again and upload the new file.',
	)
	await expect(page.getByRole('row', { name: /riverbed/ })).toHaveCount(0)
	await expect(
		page.getByText(`${starterTheme.name} ${starterTheme.version} is serving the public site.`),
	).toBeVisible()
})
