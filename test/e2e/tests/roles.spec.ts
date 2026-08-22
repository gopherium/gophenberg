// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'

import { authorAuthFile, editorAuthFile } from '../env'

/** A content row as the admin API lists it. */
interface ListedPost {
	id: string
	title: string
	author_id: string
	updated_at: string
}

/**
 * Returns a seeded post another account wrote, as seen by the signed in request context.
 * @param request - The request context of the signed in account.
 * @returns The foreign post.
 */
async function foreignPost(request: APIRequestContext): Promise<ListedPost> {
	const session = await request.get('/api/auth/session')
	expect(session.ok()).toBe(true)
	const { id: mine } = (await session.json()) as { id: string }
	const listed = await request.get('/api/content?type=post')
	expect(listed.ok()).toBe(true)
	const { items } = (await listed.json()) as { items: ListedPost[] }
	const foreign = items.find((item) => item.author_id !== mine)
	expect(foreign, 'the seed holds no post written by another account').toBeDefined()
	return foreign as ListedPost
}

test.describe('an author', () => {
	test.use({ storageState: authorAuthFile })

	test('sees only the screens its role reaches', async ({ page }) => {
		await page.goto('/admin/')
		const menu = page.getByRole('navigation')

		await expect(menu.getByRole('link', { name: 'Media' })).toBeVisible()
		await expect(menu.getByRole('link', { name: 'Language' })).toBeVisible()
		await expect(menu.getByRole('link', { name: 'Users' })).toHaveCount(0)
		await expect(menu.getByRole('link', { name: 'Themes' })).toHaveCount(0)
		await expect(menu.getByRole('link', { name: 'Content Types' })).toHaveCount(0)
	})

	test('is returned to the dashboard from a pasted admin link', async ({ page }) => {
		await page.goto('/admin/users')

		await expect(page.getByText('Welcome to Gophenberg.')).toBeVisible()
		await expect(page.getByRole('heading', { name: 'Users' })).toHaveCount(0)
	})

	test('is refused the account list by the API', async ({ request }) => {
		const answer = await request.get('/api/users')

		expect(answer.status()).toBe(403)
		expect(((await answer.json()) as { code: string }).code).toBe('role_insufficient')
	})

	test('is refused a change to work another account wrote', async ({ request }) => {
		const foreign = await foreignPost(request)

		const answer = await request.patch(`/api/content/${foreign.id}`, {
			data: { updated_at: foreign.updated_at, title: `${foreign.title} edited` },
		})

		expect(answer.status()).toBe(403)
		expect(((await answer.json()) as { code: string }).code).toBe('not_the_author')
	})

	test('opens a post another account wrote as a reading view', async ({ page, request }) => {
		const foreign = await foreignPost(request)

		await page.goto(`/admin/content/post/${foreign.id}/edit`)

		await expect(
			page
				.locator('.godmin-page')
				.getByText('Another account wrote this, so you are reading it rather than editing it.'),
		).toBeVisible()
		await expect(page.getByRole('textbox', { name: 'Title' })).toHaveCount(0)
	})
})

test.describe('an editor', () => {
	test.use({ storageState: editorAuthFile })

	test('sees the content screens and not the keys to the site', async ({ page }) => {
		await page.goto('/admin/')
		const menu = page.getByRole('navigation')

		await expect(menu.getByRole('link', { name: 'Media' })).toBeVisible()
		await expect(menu.getByRole('link', { name: 'Users' })).toHaveCount(0)
		await expect(menu.getByRole('link', { name: 'Themes' })).toHaveCount(0)
	})

	test('is refused the theme list by the API', async ({ request }) => {
		const answer = await request.get('/api/themes')

		expect(answer.status()).toBe(403)
		expect(((await answer.json()) as { code: string }).code).toBe('role_insufficient')
	})

	test('opens a post another account wrote as the editor', async ({ page, request }) => {
		const foreign = await foreignPost(request)

		await page.goto(`/admin/content/post/${foreign.id}/edit`)

		await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	})
})
