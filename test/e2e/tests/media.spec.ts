// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)
const PICTURE = `harbor-e2e-${RUN}`
const POST_TITLE = `A post carrying a picture ${RUN}`

// PHOTO_BASE64 is an eight by eight JPEG painted for this suite.
const PHOTO_BASE64 =
	'/9j/2wCEAAYEBQYFBAYGBQYHBwYIChAKCgkJChQODwwQFxQYGBcUFhYaHSUfGhsjHBYWICwgIyYnKSopGR8tMC0oMCUo' +
	'KSgBBwcHCggKEwoKEygaFhooKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKP/A' +
	'ABEIAAgACAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQEC' +
	'AwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldY' +
	'WVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY' +
	'2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncA' +
	'AQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVW' +
	'V1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV' +
	'1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/AMTwt8P/ALn7n9K6r/hX/wD0x/Suo8LfwV1VedjM3xPt' +
	'HqXw5nuL+pR94//Z'

/**
 * Returns the painted photo as an upload payload.
 * @param name - The file name the upload carries.
 * @returns The file Playwright hands a chooser.
 */
function photo(name: string) {
	return { name, mimeType: 'image/jpeg', buffer: Buffer.from(PHOTO_BASE64, 'base64') }
}

/**
 * Returns the canvas the block editor writes into.
 * @param page - The page holding the editor.
 * @returns The canvas frame.
 */
function canvas(page: Page) {
	return page.frameLocator('iframe[name="editor-canvas"]')
}

const createdPosts: string[] = []

test.afterEach(async ({ page }) => {
	for (const id of createdPosts.splice(0)) {
		await page.request.delete(`/api/posts/${id}?force=true`)
	}
	const listed = await page.request.get(`/api/media?search=e2e-${RUN}`)
	const body = (await listed.json()) as { items: { id: number }[] }
	for (const item of body.items) {
		await page.request.delete(`/api/media/${item.id}`)
	}
})

test('uploads a picture to the library and serves it publicly', async ({ page }) => {
	await page.goto('/admin/media')
	await expect(page.getByRole('heading', { name: 'Media' })).toBeVisible()

	await page.getByLabel('Add media').setInputFiles(photo(`${PICTURE}.jpg`))

	await expect(page.getByText(PICTURE)).toBeVisible()
	const listed = await page.request.get(`/api/media?search=${PICTURE}`)
	const body = (await listed.json()) as { items: { file: string }[] }
	expect(body.items).toHaveLength(1)

	const served = await page.request.get(`/media/${body.items[0].file}`)
	expect(served.status()).toBe(200)
	expect(served.headers()['content-type']).toContain('image/jpeg')
})

test('places an uploaded picture in a post and publishes it', async ({ page }) => {
	await page.goto('/admin/posts')
	await page.getByRole('button', { name: 'Add New' }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	const postId = page.url().match(/posts\/([0-9a-f-]+)\/edit/)?.[1]
	if (postId !== undefined) {
		createdPosts.push(postId)
	}
	await page.getByRole('textbox', { name: 'Title' }).fill(POST_TITLE)

	await canvas(page).getByRole('button', { name: 'Add default block' }).click()
	await page.keyboard.type('/image')
	await page.keyboard.press('Enter')
	const chooserOpens = page.waitForEvent('filechooser')
	await canvas(page).getByRole('button', { name: 'Upload' }).click()
	const chooser = await chooserOpens
	await chooser.setFiles(photo(`${PICTURE}-placed.jpg`))
	await expect(canvas(page).locator('img[src^="/media/"]')).toBeVisible()

	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(page.locator('#root').getByText('Draft saved.')).toBeVisible()
	await page.reload()
	const placed = canvas(page).locator('img[src^="/media/"]')
	await expect(placed).toBeVisible()

	await placed.click()
	await expect(placed).toBeVisible()

	await page.getByRole('button', { name: 'Publish' }).click()
	await expect(page.locator('#root').getByText('Post published.')).toBeVisible()

	const slugged = await page.request.get(`/api/posts/${postId}`)
	const post = (await slugged.json()) as { slug: string }
	await page.goto(`/post/${post.slug}`)
	const published = page.locator(`img[src^="/media/"]`)
	await expect(published).toBeVisible()
	const src = await published.getAttribute('src')
	const served = await page.request.get(src as string)
	expect(served.status()).toBe(200)
})
