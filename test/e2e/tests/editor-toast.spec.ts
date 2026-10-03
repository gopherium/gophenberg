// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

/**
 * Opens a fresh draft, gives it a title and saves it, raising the saved message.
 * @param page - The page to drive.
 * @returns The raised message.
 */
async function saveDraft(page: Page) {
	await page.getByRole('textbox', { name: 'Title' }).fill('A post the toast placement spec wrote')
	await page.getByRole('button', { name: 'Save draft' }).click()
	const toast = page.locator('#root').getByText('Draft saved.')
	await expect(toast).toBeVisible()
	return toast
}

/**
 * Reads how far the message floats above the editor footer and from the centre of the writing area.
 * @param toast - The raised message.
 * @returns Both distances in whole pixels.
 */
function placement(toast: Locator) {
	return toast.evaluate((element) => {
		const box = (element.closest('.godmin-toast') as HTMLElement).getBoundingClientRect()
		const rect = (selector: string) => (document.querySelector(selector) as HTMLElement).getBoundingClientRect()
		const outline = document.querySelector('.gophenberg-editor__outline')
		const start = outline === null ? rect('.gophenberg-editor__main').left : outline.getBoundingClientRect().right
		return {
			aboveFoot: Math.round(rect('.gophenberg-editor__foot').top - box.bottom),
			offCentre: Math.round(Math.abs((box.left + box.right) / 2 - (start + rect('.gophenberg-editor__sidebar').left) / 2)),
		}
	})
}

const created: string[] = []

test.beforeEach(async ({ page }) => {
	await page.goto('/admin/content/post')
	await page.getByRole('button', { name: 'Add New' }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	const id = page.url().match(/content\/[a-z-]+\/([0-9a-f-]+)\/edit/)?.[1]
	if (id !== undefined) {
		created.push(id)
	}
})

test.afterEach(async ({ page }) => {
	for (const id of created.splice(0)) {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
})

test('floats a message raised in the editor clear of its footer, over the writing area', async ({ page }) => {
	const toast = await saveDraft(page)

	await expect.poll(() => placement(toast)).toEqual({ aboveFoot: 16, offCentre: 0 })
})

test('keeps a message over the writing area while the list view is open', async ({ page }) => {
	await page.getByRole('button', { name: 'List view' }).click()
	await expect(page.locator('.gophenberg-editor__outline')).toBeVisible()

	const toast = await saveDraft(page)

	await expect.poll(() => placement(toast)).toEqual({ aboveFoot: 16, offCentre: 0 })
})
