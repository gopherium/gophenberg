// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

test('floats a raised message over the screen rather than in its flow', async ({ page }) => {
	await page.goto('/posts')
	await page.getByRole('button', { name: 'Add New' }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	await page.getByRole('textbox', { name: 'Title' }).fill('A post the toast spec wrote')

	await page.getByRole('button', { name: 'Save draft' }).click()

	const toast = page.locator('#root').getByText('Draft saved.')
	await expect(toast).toBeVisible()
	const region = await toast.evaluate((element) => {
		const held = element.closest('.godmin-toasts') as HTMLElement
		return {
			position: getComputedStyle(held).position,
			withinViewport: held.getBoundingClientRect().bottom <= window.innerHeight,
		}
	})
	expect(region.position).toBe('fixed')
	expect(region.withinViewport).toBe(true)
})
