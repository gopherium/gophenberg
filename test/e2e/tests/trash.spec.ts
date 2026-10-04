// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'

const created: string[] = []

/**
 * Creates an item of a type and moves it to the trash.
 * @param request - The signed in API client.
 * @param type - The content type of the item.
 * @param title - The title of the item.
 * @returns The id of the item.
 */
async function trashed(request: APIRequestContext, type: string, title: string): Promise<string> {
	const answer = await request.post('/api/content', { data: { type, title } })
	expect(answer.status()).toBe(201)
	const { id } = (await answer.json()) as { id: string }
	created.push(id)
	expect((await request.delete(`/api/content/${id}`)).ok()).toBe(true)
	return id
}

test.afterEach(async ({ page }) => {
	for (const id of created.splice(0)) {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
})

test('empties only the trash of the type on screen', async ({ page }) => {
	const post = await trashed(page.request, 'post', 'A post the empty trash spec trashed')
	const item = await trashed(page.request, 'page', 'A page the empty trash spec trashed')
	await page.goto('/admin/content/page')
	await page.getByRole('button', { name: /^Trash \(\d+\)$/ }).click()
	await page.getByRole('button', { name: 'Empty Trash' }).click()

	await page.getByRole('button', { name: 'Delete All' }).click()

	await expect(page.getByRole('alertdialog')).toBeHidden()
	expect((await page.request.get(`/api/content/${item}`)).status()).toBe(404)
	const left = await page.request.get(`/api/content/${post}`)
	expect(left.status()).toBe(200)
	expect(((await left.json()) as { status: string }).status).toBe('trash')
})
