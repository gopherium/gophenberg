// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

const PHONE = { width: 390, height: 844 }

test('pads the canvas and keeps the rail beside it on a desktop', async ({ page }) => {
	await page.goto('/admin/')

	await expect(page.getByRole('navigation')).toBeVisible()
	await expect(page.getByRole('button', { name: 'Open navigation' })).toBeHidden()
	const padding = await page
		.locator('.godmin-layout__canvas')
		.evaluate((canvas) => getComputedStyle(canvas).padding)
	expect(padding).toBe('16px 24px')
})

test('reads every rail row clearly against the chrome', async ({ page }) => {
	await page.goto('/admin/content/post')
	await expect(page.getByRole('link', { name: 'All Posts' })).toBeVisible()

	const rows = await page.locator('.gophenberg-menu__item').evaluateAll((found) =>
		found.map((row) => {
			const rgb = getComputedStyle(row).color.match(/\d+/g) ?? []
			const channel = (raw: string) => {
				const value = Number(raw) / 255
				return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
			}
			const [r = 0, g = 0, b = 0] = rgb.map(channel)
			return { text: row.textContent, luminance: 0.2126 * r + 0.7152 * g + 0.0722 * b }
		}),
	)

	expect(rows.length).toBeGreaterThan(1)
	for (const row of rows) {
		expect(row.luminance, `${row.text} is too dark for the chrome`).toBeGreaterThan(0.5)
	}
})

test('spans the posts listing across the canvas', async ({ page }) => {
	await page.goto('/admin/content/post')
	await expect(page.getByRole('heading', { level: 1, name: 'Posts' })).toBeVisible()
	await expect(page.locator('.dataviews-view-table')).toBeVisible()

	const fit = await page.evaluate(() => {
		const canvas = document.querySelector('.godmin-layout__canvas') as HTMLElement
		const list = document.querySelector('.godmin-page__list') as HTMLElement
		const table = document.querySelector('.dataviews-view-table') as HTMLElement
		const inside = canvas.getBoundingClientRect().left + canvas.clientLeft
		const edges = list.getBoundingClientRect()
		return {
			styled: getComputedStyle(table).borderCollapse,
			start: edges.left - inside,
			end: inside + canvas.clientWidth - edges.right,
			filled: table.getBoundingClientRect().width / edges.width,
		}
	})

	expect(fit.styled).toBe('collapse')
	expect(fit.start).toBeCloseTo(0, 0)
	expect(fit.end).toBeCloseTo(0, 0)
	expect(fit.filled).toBeGreaterThan(0.99)
})

test('folds the rail into a drawer on a phone', async ({ page }) => {
	await page.setViewportSize(PHONE)

	await page.goto('/admin/')

	await expect(page.locator('.godmin-layout__rail')).toHaveCount(0)
	await page.getByRole('button', { name: 'Open navigation' }).click()
	const drawer = page.getByRole('dialog')
	await expect(drawer.getByRole('link', { name: 'Posts' })).toBeVisible()
})

test('meets the screen edges on a phone without spilling it', async ({ page }) => {
	await page.setViewportSize(PHONE)

	await page.goto('/admin/')

	const canvas = page.locator('.godmin-layout__canvas')
	await expect(canvas).toBeVisible()
	const fit = await canvas.evaluate((element) => ({
		margin: getComputedStyle(element).margin,
		spills: element.getBoundingClientRect().right > document.documentElement.clientWidth,
	}))
	expect(fit.margin).toBe('0px')
	expect(fit.spills).toBe(false)
})
