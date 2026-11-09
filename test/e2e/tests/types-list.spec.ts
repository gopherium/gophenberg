// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)

const PHONE = { width: 390, height: 844 }

const made: string[] = []

test.afterEach(async ({ page }) => {
	for (const key of made.splice(0)) {
		await page.request.patch(`/api/types/${key}`, { data: { active: true } })
		await page.request.delete(`/api/types/${key}`)
	}
})

/**
 * Registers a type through the API, so the steps that change types act on types of their own.
 * @param page - The page whose session registers the type.
 * @param word - The word the key, the names and the route word are made from.
 * @returns The plural name the list shows.
 */
async function typeNamed(page: Page, word: string): Promise<string> {
	const key = `${word}-${RUN}`
	const answer = await page.request.post('/api/types', {
		data: {
			key,
			singular_label: `${word} ${RUN}`,
			plural_label: `${word}s ${RUN}`,
			description: '',
			route_word: `${word}s-${RUN}`,
		},
	})
	expect(answer.status()).toBe(201)
	made.push(key)
	return `${word}s ${RUN}`
}

/**
 * Returns the row of the types table showing a name.
 * @param page - The page holding the list.
 * @param name - The name the row shows.
 * @returns The row locator.
 */
function rowOf(page: Page, name: string): Locator {
	return page.getByRole('row').filter({ has: page.locator('.dataviews-title-field', { hasText: name }) })
}

/**
 * Opens the list narrowed to a name and the row menu of the type showing it.
 * @param page - The page to drive.
 * @param name - The name the row shows.
 */
async function openRowMenu(page: Page, name: string) {
	await page.goto(`/admin/content-types?search=${encodeURIComponent(name)}`)
	await rowOf(page, name).getByRole('button', { name: 'Actions' }).click()
}

/**
 * Returns the box of the open dialog or confirm once its opening animation ends, rounded to the pixel.
 * @param page - The page holding the dialog.
 * @param side - The edge or size of the box to read.
 * @returns The poll that reads it until it holds still.
 */
function frameBox(page: Page, side: 'width' | 'left') {
	return expect.poll(async () => {
		const box = await page.getByRole('dialog').or(page.getByRole('alertdialog')).boundingBox()
		return Math.round(side === 'width' ? (box?.width ?? 0) : (box?.x ?? 0))
	})
}

test('lists the content types in a WordPress table, every row action in its menu', async ({ page }) => {
	await page.goto('/admin/content-types')

	const posts = rowOf(page, 'Posts')
	await expect(posts.getByRole('cell', { name: '/', exact: true })).toBeVisible()
	await expect(posts.getByText('Default', { exact: true })).toBeVisible()
	await expect(posts.getByRole('button')).toHaveCount(1)
	await rowOf(page, 'Pages').getByRole('button', { name: 'Actions' }).click()
	await expect(page.getByRole('menuitem')).toHaveText([
		'Describe…',
		'Change address…',
		'Make default…',
		'Stop nesting',
		'Deactivate',
		'Delete…',
	])
})

test('narrows the content types to a search', async ({ page }) => {
	await page.goto('/admin/content-types')
	await expect(rowOf(page, 'Pages')).toBeVisible()

	await page.getByRole('searchbox', { name: 'Search content types…' }).fill('categ')

	await expect(rowOf(page, 'Categories')).toBeVisible()
	await expect(rowOf(page, 'Pages')).toHaveCount(0)
	await expect(page).toHaveURL(/search=categ/)
})

test('adds a type from a 560 pixel Add Content Type whose Add waits for both names', async ({ page }) => {
	await page.goto('/admin/content-types')

	await page.getByRole('button', { name: 'Add Content Type' }).click()
	const dialog = page.getByRole('dialog', { name: 'Add Content Type' })
	await expect(dialog.getByLabel('Singular name')).toBeFocused()
	await frameBox(page, 'width').toBe(560)
	await expect(dialog.getByRole('button', { name: 'Add', exact: true })).toHaveAttribute('aria-disabled', 'true')
	await dialog.getByLabel('Singular name').fill(`Lesson ${RUN}`)
	await dialog.getByLabel('Plural name').fill(`Lessons ${RUN}`)
	made.push(`lesson-${RUN}`)
	await dialog.getByRole('button', { name: 'Add', exact: true }).click()

	await expect(page.locator('.godmin-toast', { hasText: `"Lessons ${RUN}" successfully created.` })).toBeVisible()
	await expect(dialog).toHaveCount(0)
	await expect(rowOf(page, `Lessons ${RUN}`)).toBeVisible()
})

test('keeps Add Content Type open on the reason a taken key gave, said inside it', async ({ page }) => {
	await page.goto('/admin/content-types')

	await page.getByRole('button', { name: 'Add Content Type' }).click()
	const dialog = page.getByRole('dialog', { name: 'Add Content Type' })
	await dialog.getByLabel('Singular name').fill('Post')
	await dialog.getByLabel('Plural name').fill(`Posts ${RUN}`)
	await dialog.getByRole('button', { name: 'Add', exact: true }).click()

	await expect(dialog.getByText('Another content type already uses this key.')).toBeVisible()
	await expect(page.locator('.godmin-toast')).toHaveCount(0)
	await expect(dialog.getByLabel('Plural name')).toHaveValue(`Posts ${RUN}`)
})

test('describes a type in a 560 pixel row dialog that closes on Save', async ({ page }) => {
	const name = await typeNamed(page, 'guide')
	await openRowMenu(page, name)

	await page.getByRole('menuitem', { name: 'Describe…' }).click()
	const dialog = page.getByRole('dialog', { name: 'Describe' })
	await expect(dialog.getByLabel('Description')).toBeFocused()
	await frameBox(page, 'width').toBe(560)
	await dialog.getByLabel('Description').fill('Step by step guides for new readers.')
	await dialog.getByRole('button', { name: 'Save' }).click()

	await expect(dialog).toHaveCount(0)
	await expect(page.locator('.godmin-toast', { hasText: 'Description updated.' })).toBeVisible()
	await expect(rowOf(page, name).getByText('Step by step guides for new readers.')).toBeVisible()
})

test('moves every address of a type from Change address', async ({ page }) => {
	const name = await typeNamed(page, 'recipe')
	await openRowMenu(page, name)

	await page.getByRole('menuitem', { name: 'Change address…' }).click()
	const dialog = page.getByRole('dialog', { name: 'Change address' })
	await dialog.getByLabel('Route word').fill(`dishes-${RUN}`)
	await dialog.getByRole('button', { name: 'Move every address' }).click()

	await expect(page.locator('.godmin-toast', { hasText: 'Address updated.' })).toBeVisible()
	await expect(rowOf(page, name).getByText(`/dishes-${RUN}`)).toBeVisible()
})

test('lets a type nest and switches it off and on from its menu', async ({ page }) => {
	const name = await typeNamed(page, 'notice')
	await openRowMenu(page, name)

	await page.getByRole('menuitem', { name: 'Let items nest' }).click()
	await expect(page.locator('.godmin-toast', { hasText: 'Content type updated.' })).toBeVisible()
	await expect(rowOf(page, name).getByText('Nests', { exact: true })).toBeVisible()
	await rowOf(page, name).getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Deactivate' }).click()
	await expect(page.locator('.godmin-toast', { hasText: `"${name}" deactivated.` })).toBeVisible()
	await expect(rowOf(page, name).getByText('Closed', { exact: true })).toBeVisible()
	await rowOf(page, name).getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Activate' }).click()

	await expect(page.locator('.godmin-toast', { hasText: `"${name}" activated.` })).toBeVisible()
	await expect(rowOf(page, name).getByText('Closed', { exact: true })).toHaveCount(0)
})

test('deletes a type behind a 560 pixel confirm titled by the question, Cancel focused', async ({ page }) => {
	const name = await typeNamed(page, 'memo')
	await openRowMenu(page, name)

	await page.getByRole('menuitem', { name: 'Delete…' }).click()
	const dialog = page.getByRole('alertdialog')
	await expect(dialog.getByRole('heading', { name: `Delete "${name}"?` })).toBeVisible()
	await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
	await expect(dialog.getByRole('heading')).toHaveCount(1)
	await frameBox(page, 'width').toBe(560)
	await dialog.getByRole('button', { name: 'Delete', exact: true }).click()

	await expect(page.locator('.godmin-toast', { hasText: `"${name}" permanently deleted.` })).toBeVisible()
	await expect(rowOf(page, name)).toHaveCount(0)
})

test('deletes two picked types behind one confirm', async ({ page }) => {
	const first = await typeNamed(page, 'brief')
	const second = await typeNamed(page, 'digest')
	await page.goto(`/admin/content-types?search=${RUN}`)

	await rowOf(page, first).getByRole('checkbox').check()
	await rowOf(page, second).getByRole('checkbox').check()
	await page.locator('.dataviews-bulk-actions-footer__action-buttons').getByRole('button', { name: 'Delete…' }).click()
	const dialog = page.getByRole('alertdialog')
	await expect(dialog.getByRole('heading', { name: 'Delete 2 content types?' })).toBeVisible()
	await dialog.getByRole('button', { name: 'Delete', exact: true }).click()

	await expect(page.locator('.godmin-toast', { hasText: '2 content types permanently deleted.' })).toBeVisible()
	await expect(rowOf(page, first)).toHaveCount(0)
	await expect(rowOf(page, second)).toHaveCount(0)
})

test('opens a row dialog as a card in the middle of a phone, 24 pixels from each edge', async ({ page }) => {
	const name = await typeNamed(page, 'tip')
	await page.setViewportSize(PHONE)
	await page.goto(`/admin/content-types?search=${encodeURIComponent(name)}`)

	await page.getByRole('grid').getByRole('button', { name: 'Actions' }).first().click()
	await page.getByRole('menuitem', { name: 'Describe…' }).click()

	await frameBox(page, 'width').toBe(PHONE.width - 48)
	await frameBox(page, 'left').toBe(24)
})
