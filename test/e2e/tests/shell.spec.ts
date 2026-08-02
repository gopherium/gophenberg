// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

const PHONE = { width: 390, height: 844 }

test('pads the canvas and keeps the rail beside it on a desktop', async ({ page }) => {
	await page.goto('/')

	await expect(page.getByRole('navigation')).toBeVisible()
	await expect(page.getByRole('button', { name: 'Open navigation' })).toBeHidden()
	const padding = await page
		.locator('.godmin-layout__canvas')
		.evaluate((canvas) => getComputedStyle(canvas).padding)
	expect(padding).toBe('24px')
})

test('folds the rail into a drawer on a phone', async ({ page }) => {
	await page.setViewportSize(PHONE)

	await page.goto('/')

	await expect(page.locator('.godmin-layout__rail')).toHaveCount(0)
	await page.getByRole('button', { name: 'Open navigation' }).click()
	const drawer = page.getByRole('dialog')
	await expect(drawer.getByRole('link', { name: 'Posts' })).toBeVisible()
})

test('meets the screen edges on a phone without spilling it', async ({ page }) => {
	await page.setViewportSize(PHONE)

	await page.goto('/')

	const canvas = page.locator('.godmin-layout__canvas')
	await expect(canvas).toBeVisible()
	const fit = await canvas.evaluate((element) => ({
		margin: getComputedStyle(element).margin,
		spills: element.getBoundingClientRect().right > document.documentElement.clientWidth,
	}))
	expect(fit.margin).toBe('0px')
	expect(fit.spills).toBe(false)
})
