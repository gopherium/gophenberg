// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

test('spreads the block inspector across the sidebar without sideways overflow', async ({
	page,
}) => {
	await page.goto('/admin/posts')
	await page.getByRole('button', { name: /^Published/ }).click()
	await page.getByRole('link', { name: 'Pictures from Elsewhere' }).click()
	await page.frameLocator('iframe[name="editor-canvas"]').getByRole('img').click()
	await page.getByRole('tab', { name: 'Block' }).click()
	await expect(page.getByRole('tab', { name: 'Settings' })).toBeVisible()

	const sidebar = page.locator('.gophenberg-editor__sidebar')
	const overflow = await sidebar.evaluate((el) => el.scrollWidth - el.clientWidth)
	expect(overflow).toBeLessThanOrEqual(1)

	const sidebarWidth = await sidebar.evaluate((el) => el.clientWidth)
	const inspector = await page.locator('.block-editor-block-inspector').boundingBox()
	expect(inspector?.width).toBeGreaterThanOrEqual(sidebarWidth - 1)

	const settings = page.getByRole('tab', { name: 'Settings' })
	const tab = await settings.boundingBox()
	const icon = await settings.locator('svg').first().boundingBox()
	if (tab === null || icon === null) {
		throw new Error('the settings tab did not render')
	}
	const tabCenter = tab.x + tab.width / 2
	const iconCenter = icon.x + icon.width / 2
	expect(Math.abs(tabCenter - iconCenter)).toBeLessThanOrEqual(1)
})
