// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

test('floats a message raised in the editor clear of its footer, over the writing area', async ({ page }) => {
	await page.goto('/admin/content/post')
	await page.getByRole('button', { name: 'Add New' }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	await page.getByRole('textbox', { name: 'Title' }).fill('A post the toast placement spec wrote')

	await page.getByRole('button', { name: 'Save draft' }).click()

	const toast = page.locator('#root').getByText('Draft saved.')
	await expect(toast).toBeVisible()
	const placement = () =>
		toast.evaluate((element) => {
			const box = (element.closest('.godmin-toast') as HTMLElement).getBoundingClientRect()
			const rect = (selector: string) => (document.querySelector(selector) as HTMLElement).getBoundingClientRect()
			return {
				clearOfFoot: rect('.gophenberg-editor__foot').top - box.bottom >= 16,
				offCentre: Math.round(
					Math.abs((box.left + box.right) / 2 - (rect('.gophenberg-editor__main').left + rect('.gophenberg-editor__sidebar').left) / 2),
				),
			}
		})
	await expect.poll(placement).toEqual({ clearOfFoot: true, offCentre: 0 })
})
