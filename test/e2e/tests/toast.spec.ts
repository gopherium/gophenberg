// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

test('floats a raised message over the screen rather than in its flow', async ({ page }) => {
	await page.goto('/admin/content/post')
	await page.getByRole('main').getByRole('button', { name: 'Add New', exact: true }).click()
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

test('holds a raised message wide enough to read it', async ({ page }) => {
	await page.goto('/admin/content/post')
	await page.getByRole('main').getByRole('button', { name: 'Add New', exact: true }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	await page.getByRole('textbox', { name: 'Title' }).fill('A post the toast width spec wrote')

	await page.getByRole('button', { name: 'Save draft' }).click()

	const toast = page.locator('#root').getByText('Draft saved.')
	await expect(toast).toBeVisible()
	const measured = await toast.evaluate((element) => {
		const inset = (of: HTMLElement): number => {
			const style = getComputedStyle(of)
			return ['padding-top', 'padding-bottom', 'border-top-width', 'border-bottom-width'].reduce(
				(total, side) => total + Number.parseFloat(style.getPropertyValue(side)),
				0,
			)
		}
		const held = element.closest('.godmin-toasts') as HTMLElement
		const box = element.closest('.godmin-toast') as HTMLElement
		const widest = [...held.querySelectorAll('*')].reduce(
			(most, child) => Math.max(most, child.getBoundingClientRect().width),
			0,
		)
		return {
			region: held.getBoundingClientRect().width,
			widest,
			textHeight: box.getBoundingClientRect().height - inset(box),
			lineHeight: Number.parseFloat(getComputedStyle(box).lineHeight),
		}
	})

	expect(measured.region).toBeGreaterThanOrEqual(measured.widest)
	expect(measured.lineHeight).toBeGreaterThan(0)
	expect(measured.textHeight).toBeLessThan(measured.lineHeight * 2)
})
