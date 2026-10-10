// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)

const created: string[] = []

test.afterEach(async ({ page }) => {
	for (const id of created.splice(0)) {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
})

/**
 * Creates a draft through the API, so the steps that change items act on items of their own.
 * @param page - The page whose session creates the draft.
 * @param title - The title of the draft.
 * @returns The id of the draft.
 */
async function draftTitled(page: Page, title: string): Promise<string> {
	const answer = await page.request.post('/api/content', { data: { type: 'post', title } })
	expect(answer.status()).toBe(201)
	const { id } = (await answer.json()) as { id: string }
	created.push(id)
	return id
}

/**
 * Opens the list narrowed to the title and the row menu of the item carrying it.
 * @param page - The page to drive.
 * @param title - The title of the item.
 */
async function openRowMenu(page: Page, title: string) {
	await page.goto(`/admin/content/post?search=${encodeURIComponent(title)}`)
	const row = page.getByRole('row').filter({ has: page.getByRole('link', { name: title, exact: true }) })
	await row.getByRole('button', { name: 'Actions' }).click()
}

/**
 * Returns the tab of the list's status filter carrying the label, matched whole so no title holding the word answers.
 * @param page - The page holding the list.
 * @param label - The label of the tab.
 * @returns The tab locator.
 */
function statusTab(page: Page, label: string) {
	return page.getByRole('navigation', { name: 'Filter by status' }).getByRole('link', { name: label, exact: true })
}

test('narrows the list by a status tab, back on the first page with the page size kept', async ({ page }) => {
	await page.goto('/admin/content/post?perPage=10&page=2')
	await expect(page.locator('.dataviews-view-table tbody tr').first()).toBeVisible()

	await statusTab(page, 'Draft').click()

	await expect(statusTab(page, 'Draft')).toHaveAttribute('aria-current', 'page')
	await expect(page).toHaveURL(/status=draft/)
	await expect(page).toHaveURL(/perPage=10/)
	await expect(page).not.toHaveURL(/[?&]page=/)
	await expect(page.getByRole('link', { name: 'Draft Ideas for the Newsletter' })).toBeVisible()
	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeHidden()
	await statusTab(page, 'All').click()
	await expect(statusTab(page, 'All')).toHaveAttribute('aria-current', 'page')
	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeVisible()
})

test('draws the pages as a tree, each child a dash deeper, its parent named in the Parent column', async ({ page }) => {
	await page.goto('/admin/content/page')

	await expect(page.getByRole('link', { name: 'About', exact: true })).toBeVisible()
	await expect(page.getByRole('link', { name: /^—\sTeam$/ })).toBeVisible()
	await expect(page.getByRole('link', { name: /^—\s—\sVolunteers$/ })).toBeVisible()
	await page.getByRole('button', { name: 'View options' }).click()
	await page.getByRole('button', { name: 'Parent', exact: true }).click()
	await page.keyboard.press('Escape')
	const team = page.getByRole('row').filter({ has: page.getByRole('link', { name: /Team$/ }) })
	await expect(team.getByRole('cell', { name: 'About', exact: true })).toBeVisible()
})

test('shows the status of each item on All as a badge, and none on a tab of one status', async ({ page }) => {
	await page.goto('/admin/content/post')
	await expect(page.getByRole('columnheader', { name: 'Status' })).toBeVisible()
	const quiet = page.getByRole('row').filter({ has: page.getByRole('link', { name: 'Quiet Hours in the Office' }) })

	await page.getByRole('searchbox', { name: 'Search posts…', exact: true }).fill('Quiet Hours')

	await expect(quiet.getByText('Private', { exact: true })).toBeVisible()
	await statusTab(page, 'Private').click()
	await expect(page.getByRole('link', { name: 'Quiet Hours in the Office' })).toBeVisible()
	await expect(page.getByRole('columnheader', { name: 'Status' })).toBeHidden()
})

/**
 * Returns how many lines the text of an element runs over.
 * @param element - The element holding the text.
 * @returns The count of distinct line tops its text boxes sit on.
 */
async function linesOf(element: Locator): Promise<number> {
	return element.evaluate((held) => {
		const range = document.createRange()
		range.selectNodeContents(held)
		return new Set([...range.getClientRects()].map((box) => Math.round(box.top))).size
	})
}

test('keeps the status badge and the date of a row on one line at 1920 and 1280 wide, as DataViews does', async ({
	page,
}) => {
	const pending = page.getByRole('row').filter({ has: page.getByRole('link', { name: 'A Second Look at Image Sizes' }) })

	for (const width of [1920, 1280]) {
		await page.setViewportSize({ width, height: 900 })
		await page.goto(`/admin/content/post?search=${encodeURIComponent('Image Sizes')}`)
		const cells = [pending.getByText('Pending Review', { exact: true }), pending.getByText(/^Modified: /)]
		for (const cell of cells) {
			await expect(cell).toBeVisible()
			expect(await linesOf(cell)).toBe(1)
		}
	}
})

test('narrows the list to the posts of the author picked under the author chip', async ({ page }) => {
	await page.goto('/admin/content/post')
	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeVisible()

	await page.getByRole('button', { name: 'Add filter' }).click()
	await page.getByRole('menuitem', { name: 'Author', exact: true }).click()
	await page.getByRole('option', { name: 'Author', exact: true }).click()
	await page.keyboard.press('Escape')

	await expect(page).toHaveURL(/filters=/)
	await expect(page.getByRole('link', { name: 'Keyboard Shortcuts Worth Learning' })).toBeVisible()
	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeHidden()
})

test('narrows the list to the posts dated before the moment of the date chip, written as the WordPress chip writes it', async ({
	page,
}) => {
	const before = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000)
	before.setUTCHours(12, 0, 0, 0)
	const filters = JSON.stringify([{ field: 'date', operator: 'before', value: before.toISOString() }])
	await page.goto(`/admin/content/post?filters=${encodeURIComponent(filters)}`)

	await expect(page.locator('.dataviews-view-table tbody tr').first()).toBeVisible()
	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeHidden()
	await page.getByRole('button', { name: 'Filter', exact: true }).click()
	const written = new Intl.DateTimeFormat('en-US', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' })
	await expect(page.getByText(`${written.format(before)} 12:00 pm`)).toBeVisible()
})

test.describe('read on a clock ahead of UTC', () => {
	test.use({ timezoneId: 'Europe/Paris' })

	test('shows and takes the moment of the date chip on the reader clock, in summer and in winter', async ({ page }) => {
		const filters = JSON.stringify([{ field: 'date', operator: 'after', value: '2026-07-20T10:00:00.000Z' }])
		await page.goto(`/admin/content/post?filters=${encodeURIComponent(filters)}`)

		await page.getByRole('button', { name: 'Filter', exact: true }).click()
		await page.getByRole('button', { name: /^Date is after: July 20, 2026 12:00 pm/ }).click()
		const field = page.locator('input[type="datetime-local"]')
		await expect(field).toHaveValue('2026-07-20T12:00')
		await expect(page.getByText(/^Timezone:/)).toBeHidden()
		const asked = page.waitForRequest(
			(request) => new URL(request.url()).searchParams.get('after') === '2026-01-15T08:30:00.000Z',
		)
		await field.fill('2026-01-15T09:30')

		await asked
	})
})

test('renames an item from its row in a dialog that hands the Name field the focus', async ({ page }) => {
	const title = `A post the list spec renames ${RUN}`
	await draftTitled(page, title)
	await openRowMenu(page, title)

	await page.getByRole('menuitem', { name: 'Rename…', exact: true }).click()
	const dialog = page.getByRole('dialog', { name: 'Rename' })
	const field = dialog.getByRole('textbox', { name: 'Name' })
	await expect(field).toBeFocused()
	await field.fill(`${title} again`)
	await dialog.getByRole('button', { name: 'Rename', exact: true }).click()

	await expect(page.locator('#root').getByText('Name updated.')).toBeVisible()
	await expect(page.getByRole('link', { name: `${title} again`, exact: true })).toBeVisible()
})

test('duplicates an item from its row into a new draft under the title typed', async ({ page }) => {
	const title = `A post the list spec copies ${RUN}`
	await draftTitled(page, title)
	await openRowMenu(page, title)

	await page.getByRole('menuitem', { name: 'Duplicate…', exact: true }).click()
	const dialog = page.getByRole('dialog', { name: 'Duplicate' })
	await expect(dialog.getByRole('textbox', { name: 'Title' })).toHaveValue(`${title} (Copy)`)
	await dialog.getByRole('button', { name: 'Duplicate', exact: true }).click()

	await expect(page.locator('#root').getByText(/successfully created\.$/)).toBeVisible()
	await expect(page.getByRole('link', { name: `${title} (Copy)`, exact: true })).toBeVisible()
	const copies = await page.request.get(`/api/content?search=${encodeURIComponent(`${title} (Copy)`)}`)
	created.push(...((await copies.json()) as { items: { id: string }[] }).items.map((item) => item.id))
})

test('opens a published item at its public address in a new tab', async ({ page }) => {
	await openRowMenu(page, 'Welcome to Gophenberg')

	const opened = page.waitForEvent('popup')
	await page.getByRole('menuitem', { name: 'View', exact: true }).click()

	await expect(await opened).toHaveURL(/\/welcome-to-gophenberg$/)
})

test('shows only View on the published row the pointer rests on, as WordPress draws its primary actions', async ({
	page,
}) => {
	await page.goto(`/admin/content/post?search=${encodeURIComponent('Welcome to Gophenberg')}`)
	const row = page.getByRole('row').filter({ has: page.getByRole('link', { name: 'Welcome to Gophenberg', exact: true }) })
	const view = row.getByRole('button', { name: 'View', exact: true })
	await expect(view).toHaveCSS('opacity', '0')

	await row.hover()

	await expect(view).toHaveCSS('opacity', '1')
	await expect(row.getByRole('button', { name: 'Trash…', exact: true })).toHaveCount(0)
})

test('says no items are found when a search matches none', async ({ page }) => {
	await page.goto(`/admin/content/post?search=${encodeURIComponent(`nothing matches ${RUN}`)}`)

	await expect(page.getByText('No items found.')).toBeVisible()
	await expect(page.getByText('Add one with Add New.')).toBeHidden()
})

test('trashes two ticked items at once and counts them in a toast', async ({ page }) => {
	const first = `A post the list spec bins first ${RUN}`
	const second = `A post the list spec bins second ${RUN}`
	await draftTitled(page, first)
	await draftTitled(page, second)
	await page.goto(`/admin/content/post?search=${encodeURIComponent('the list spec bins')}`)

	await page.getByRole('checkbox', { name: first, exact: true }).check()
	await page.getByRole('checkbox', { name: second, exact: true }).check()
	await expect(page.getByText('2 Items selected')).toBeVisible()
	await page.getByRole('button', { name: 'Trash…', exact: true }).click()
	await page.getByRole('dialog').getByRole('button', { name: 'Trash', exact: true }).click()

	await expect(page.locator('#root').getByText('2 items moved to the trash.')).toBeVisible()
	await expect(page.getByRole('link', { name: first, exact: true })).toBeHidden()
})

test('restores one item from the Trash tab and deletes another for good', async ({ page }) => {
	const kept = `A post the list spec keeps ${RUN}`
	const gone = `A post the list spec drops ${RUN}`
	for (const title of [kept, gone]) {
		const id = await draftTitled(page, title)
		expect((await page.request.delete(`/api/content/${id}`)).ok()).toBe(true)
	}
	await page.goto(`/admin/content/post?status=trash&search=${encodeURIComponent('the list spec')}`)
	const row = (title: string) => page.getByRole('row').filter({ has: page.getByRole('link', { name: title, exact: true }) })

	await row(kept).getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Restore', exact: true }).click()
	await expect(page.locator('#root').getByText(`"${kept}" has been restored.`)).toBeVisible()
	await row(gone).getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Permanently delete…', exact: true }).click()
	await page.getByRole('dialog').getByRole('button', { name: 'Permanently delete', exact: true }).click()

	await expect(page.locator('#root').getByText(`"${gone}" permanently deleted.`)).toBeVisible()
	await expect(page.getByRole('link', { name: kept, exact: true })).toBeHidden()
	await expect(page.getByRole('link', { name: gone, exact: true })).toBeHidden()
})

test('opens the trash confirm of a row on Cancel, as WordPress does', async ({ page }) => {
	const title = `A post the list spec asks to trash ${RUN}`
	await draftTitled(page, title)
	await openRowMenu(page, title)

	await page.getByRole('menuitem', { name: 'Trash…', exact: true }).click()

	await expect(page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true })).toBeFocused()
})

test('opens the permanent delete confirm of a row on Cancel, as WordPress does', async ({ page }) => {
	const title = `A post the list spec asks to delete ${RUN}`
	const id = await draftTitled(page, title)
	expect((await page.request.delete(`/api/content/${id}`)).ok()).toBe(true)
	await page.goto(`/admin/content/post?status=trash&search=${encodeURIComponent(title)}`)
	const row = page.getByRole('row').filter({ has: page.getByRole('link', { name: title, exact: true }) })
	await row.getByRole('button', { name: 'Actions' }).click()

	await page.getByRole('menuitem', { name: 'Permanently delete…', exact: true }).click()

	await expect(page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true })).toBeFocused()
})

test('refuses to trash a page that still holds pages, saying why above the list', async ({ page }) => {
	await page.goto('/admin/content/page')
	const team = page.getByRole('row').filter({ has: page.getByRole('link', { name: /Team$/ }) })

	await team.getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Trash…', exact: true }).click()
	await page.getByRole('dialog').getByRole('button', { name: 'Trash', exact: true }).click()

	const list = page.getByRole('region', { name: 'Pages' })
	await expect(list.getByText('This item still holds items nested inside it. Move or delete those first.')).toBeVisible()
	await expect(page.getByRole('link', { name: /Team$/ })).toBeVisible()
})

test('keeps the search a reader typed across a reload', async ({ page }) => {
	await page.goto('/admin/content/post')
	const search = page.getByRole('searchbox', { name: 'Search posts…', exact: true })

	await search.fill('Garden Club')

	await expect(page).toHaveURL(/search=Garden/)
	await expect(page.getByRole('link', { name: 'Garden Club Meeting Recap' })).toBeVisible()
	await page.reload()
	await expect(page.getByRole('searchbox', { name: 'Search posts…', exact: true })).toHaveValue('Garden Club')
	await expect(page.getByRole('link', { name: 'Garden Club Meeting Recap' })).toBeVisible()
	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeHidden()
})

test('pages the list at the size picked under View options and keeps the page across a reload', async ({ page }) => {
	await page.goto('/admin/content/post')
	const rows = page.locator('.dataviews-view-table tbody tr')
	await expect(rows.first()).toBeVisible()

	await page.getByRole('button', { name: 'View options' }).click()
	await page.getByRole('radiogroup', { name: 'Items per page' }).getByRole('radio', { name: '10', exact: true }).click()
	await page.keyboard.press('Escape')

	await expect(page).toHaveURL(/perPage=10/)
	await expect(rows).toHaveCount(10)
	await page.getByRole('button', { name: 'Next page' }).click()
	await expect(page).toHaveURL(/[?&]page=2/)
	await page.reload()
	await expect(page).toHaveURL(/[?&]page=2/)
	await expect(rows).toHaveCount(10)
})
