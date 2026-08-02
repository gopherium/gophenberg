// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

const PHONE = { width: 390, height: 844 }

const LONG_TITLE =
	'A post whose title runs on well past the width any phone screen could ever offer a reader'

const UNBREAKABLE = 'W'.repeat(60)

/**
 * Returns how far the document and the canvas overflow the screen.
 * @param page - The page to measure.
 * @returns The overflow of each region in pixels.
 */
async function spills(page: Page) {
	return page.evaluate(() => {
		const canvas = document.querySelector('.godmin-layout__canvas')
		const width = document.documentElement.clientWidth
		return {
			document: document.documentElement.scrollWidth - width,
			canvas: canvas === null ? 0 : Math.round(canvas.scrollWidth - canvas.clientWidth),
		}
	})
}

test.beforeEach(async ({ page }) => {
	await page.setViewportSize(PHONE)
})

test('seeds a hostile post and fits every screen to a phone', async ({ page }) => {
	const created = await page.request.post('/api/posts', {
		data: { type: 'post', title: `${LONG_TITLE} ${UNBREAKABLE}` },
	})
	expect(created.status()).toBe(201)

	for (const path of ['/', '/posts', '/users', '/users/new']) {
		await page.goto(path)
		await expect(page.getByRole('main')).toBeVisible()

		const overflow = await spills(page)
		expect(overflow.document, `${path} spills the document`).toBeLessThanOrEqual(0)
		expect(overflow.canvas, `${path} spills the canvas`).toBeLessThanOrEqual(0)
	}
})
