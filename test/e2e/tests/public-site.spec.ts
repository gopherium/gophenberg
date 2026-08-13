// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

test('lists the demo posts on the public index', async ({ page }) => {
	await page.goto('/')

	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeVisible()
	await expect(page.getByRole('link', { name: 'Writing with Blocks' })).toBeVisible()
	await expect(page.getByRole('link', { name: 'Pictures from Elsewhere' })).toBeVisible()
})

test('serves the public index from the theme rather than the built-in renderer', async ({ page }) => {
	await page.goto('/')

	await expect(page.getByRole('banner').getByRole('link', { name: 'Gophenberg Starter' })).toBeVisible()
})

test('renders a post through the theme quote override', async ({ page }) => {
	await page.goto('/welcome-to-gophenberg')

	await expect(page.getByRole('heading', { name: 'Welcome to Gophenberg', level: 1 })).toBeVisible()
	const quote = page.locator('figure.starter-quote')
	await expect(quote).toBeVisible()
	await expect(quote.locator('figcaption')).toHaveText('Maria Perez')
})

test('names what served a themed page', async ({ page }) => {
	await page.goto('/welcome-to-gophenberg')

	await expect(page.locator('meta[name="generator"]')).toHaveAttribute('content', /^Gophenberg \d+\.\d+$/)
})

test('carries the identification headers on a themed response', async ({ page }) => {
	const response = await page.request.get('/welcome-to-gophenberg')

	expect(response.status()).toBe(200)
	expect(response.headers()['x-generator']).toMatch(/^Gophenberg \d+\.\d+$/)
	expect(response.headers()['link']).toContain('gophenberg.org/api')
})

test('links the block stylesheet the stored markup was written against', async ({ page }) => {
	await page.goto('/welcome-to-gophenberg')

	const href = await page.locator('link[rel="stylesheet"][href*="blocks.css"]').getAttribute('href')
	expect(href).toBe('/gophenberg/blocks.css')

	const stylesheet = await page.request.get(href as string)
	expect(stylesheet.status()).toBe(200)
})

test('serves a page the theme owns rather than one the kit injected', async ({ page }) => {
	await page.goto('/about')

	await expect(page.getByRole('heading', { name: 'About this theme' })).toBeVisible()
})

test('keeps the readiness probe off the public site', async ({ page }) => {
	const response = await page.request.get('/_gophenberg/health')

	expect(response.status()).toBe(404)
})

test('shows a post published in the admin without rebuilding the theme', async ({ page }) => {
	const title = 'Published While the Theme Was Running'

	const created = await page.request.post('/api/content', { data: { title } })
	expect(created.status()).toBe(201)
	const { id, updated_at: updatedAt } = await created.json()

	const published = await page.request.patch(`/api/content/${id}`, {
		data: { status: 'published', updated_at: updatedAt },
	})
	expect(published.status()).toBe(200)

	try {
		await page.goto('/')
		await expect(page.getByRole('link', { name: title })).toBeVisible()
	} finally {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
})
