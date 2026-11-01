// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { APIRequest, Locator, Page } from '@playwright/test'

import { baseURL, credentials } from '../env'

const PHONE = { width: 390, height: 844 }

const LAPTOP = { width: 1280, height: 800 }

const LONG_TITLE =
	'A post whose title runs on well past the width any phone screen could ever offer a reader'

const UNBREAKABLE = 'W'.repeat(60)

const RUN = Math.random().toString(36).slice(2, 8)

const LONG_NAME = Array.from({ length: 11 }, () => 'Maria Perez').join(' ')

const LONG_NAMES = [
	{ reads: 'of many words', condition: 'spaced', name: LONG_NAME },
	{ reads: 'with no spaces', condition: 'unspaced', name: LONG_NAME.replaceAll(' ', '') },
]

const POST_TABS = [
	{ tab: 'All', narrowing: '' },
	{ tab: 'Draft', narrowing: 'status=draft&' },
]

/**
 * Creates an author under the name and a post that author writes, through the API the admin uses.
 * @param page - The page of the signed in administrator, which creates the account.
 * @param requests - The factory of request contexts, which opens the author's own session.
 * @param condition - The condition the name stands for, naming the account and the post.
 * @param name - The display name of the author.
 * @returns The id and the title of the post.
 */
async function postWrittenBy(page: Page, requests: APIRequest, condition: string, name: string) {
	const email = `${condition}-${RUN}@example.com`
	const made = await page.request.post('/api/users', {
		data: { email, name, password: credentials.password, role: 'author' },
	})
	expect(made.status()).toBe(201)
	const author = await requests.newContext({ baseURL })
	const login = await author.post('/api/auth/login', { data: { email, password: credentials.password } })
	expect(login.ok()).toBe(true)
	const title = `A post signed with the ${condition} long name ${RUN}`
	const created = await author.post('/api/content', { data: { type: 'post', title } })
	expect(created.status()).toBe(201)
	const { id } = (await created.json()) as { id: string }
	await author.dispose()
	return { id, title }
}

/**
 * Returns how the author name is drawn inside its table cell.
 * @param name - The author name in its cell.
 * @returns The text overflow of the name, whether its text runs past its box, and whether its box ends inside the cell.
 */
async function cutOf(name: Locator) {
	return name.evaluate((element) => {
		const cell = element.closest('td') as HTMLElement
		return {
			textOverflow: getComputedStyle(element).textOverflow,
			clipped: element.scrollWidth > element.clientWidth,
			insideCell: element.getBoundingClientRect().right <= cell.getBoundingClientRect().right,
		}
	})
}

/**
 * Opens the posts tab searched for one title and returns how its table sits in its card.
 * @param page - The page of the signed in administrator.
 * @param narrowing - The status the tab narrows the list to, as the address writes it before the search.
 * @param title - The title of the post the list is searched for.
 * @returns How far the table runs past its card, and the width of the Title column.
 */
async function postsTableFor(page: Page, narrowing: string, title: string) {
	await page.goto(`/admin/content/post?${narrowing}search=${encodeURIComponent(title)}`)
	const table = page.locator('.dataviews-view-table')
	await expect(table.getByRole('link', { name: title, exact: true })).toBeVisible()
	const heading = await table.getByRole('columnheader', { name: 'Title', exact: true }).boundingBox()
	const spill = await table.evaluate((element) => {
		const scroller = element.closest('.dataviews-layout__container') as HTMLElement
		return scroller.scrollWidth - scroller.clientWidth
	})
	return { table, spill, title: Math.round(heading?.width ?? 0) }
}

/**
 * Returns the left and right edges of an element, rounded to the pixel.
 * @param element - The element to measure.
 * @returns The edges in pixels from the left of the viewport.
 */
async function edgesOf(element: Locator) {
	const box = await element.boundingBox()
	const left = Math.round(box?.x ?? 0)
	return { left, right: left + Math.round(box?.width ?? 0) }
}

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
	const created = await page.request.post('/api/content', {
		data: { type: 'post', title: `${LONG_TITLE} ${UNBREAKABLE}` },
	})
	expect(created.status()).toBe(201)

	const screens = [
		'/admin/',
		'/admin/content/post',
		'/admin/content/post?status=trash',
		'/admin/content/page',
		'/admin/content/category',
		'/admin/content-types',
		'/admin/media',
		'/admin/users',
		'/admin/users/new',
	]
	for (const path of screens) {
		await page.goto(path)
		await expect(page.getByRole('main')).toBeVisible()

		const overflow = await spills(page)
		expect(overflow.document, `${path} spills the document`).toBeLessThanOrEqual(0)
		expect(overflow.canvas, `${path} spills the canvas`).toBeLessThanOrEqual(0)
	}
})

test('fits the posts table to its card at 1280 beside a long title, its listed fields open', async ({ page }) => {
	const title = `${LONG_TITLE} ${RUN}`
	const created = await page.request.post('/api/content', { data: { type: 'post', title } })
	expect(created.status()).toBe(201)
	const { id } = (await created.json()) as { id: string }
	await page.setViewportSize(LAPTOP)

	await page.goto(`/admin/content/post?search=${encodeURIComponent(title)}`)
	const table = page.locator('.dataviews-view-table')
	await expect(table.getByRole('link', { name: title, exact: true })).toBeVisible()

	const scroll = await table.evaluate((element) => {
		const scroller = element.closest('.dataviews-layout__container') as HTMLElement
		return scroller.scrollWidth - scroller.clientWidth
	})
	expect(scroll).toBeLessThanOrEqual(0)
	await expect(table.locator('.dataviews-view-table__actions-column--stuck')).toHaveCount(0)
	await expect(page.getByRole('columnheader', { name: 'Featured' })).toBeVisible()
	await page.request.delete(`/api/content/${id}?force=true`)
})

test('fits the content types table to its card at 1280, every row action in one menu', async ({ page }) => {
	await page.setViewportSize(LAPTOP)

	await page.goto('/admin/content-types')
	const table = page.locator('.dataviews-view-table')
	await expect(table.locator('.dataviews-title-field', { hasText: 'Pages' })).toBeVisible()

	const scroll = await table.evaluate((element) => {
		const scroller = element.closest('.dataviews-layout__container') as HTMLElement
		return scroller.scrollWidth - scroller.clientWidth
	})
	expect(scroll).toBeLessThanOrEqual(0)
	await expect(table.locator('.dataviews-view-table__actions-column--stuck')).toHaveCount(0)
	const badges = table.locator('.gophenberg-status')
	for (const badge of await badges.all()) {
		const kept = await badge.evaluate((element) => {
			const cell = element.closest('td') as HTMLElement
			return {
				oneLine: getComputedStyle(element).whiteSpace === 'nowrap',
				insideCell: element.getBoundingClientRect().right <= cell.getBoundingClientRect().right,
			}
		})
		expect(kept).toEqual({ oneLine: true, insideCell: true })
	}
	expect(await badges.count()).toBeGreaterThan(0)
})

test('fits the users table to its card at 1280 beside a long name, every default column shown', async ({ page }) => {
	const email = `listed-${RUN}@example.com`
	const made = await page.request.post('/api/users', {
		data: { email, name: LONG_NAME, password: credentials.password, role: 'author' },
	})
	expect(made.status()).toBe(201)
	await page.setViewportSize(LAPTOP)

	await page.goto(`/admin/users?search=${encodeURIComponent(email)}`)
	const table = page.locator('.dataviews-view-table')
	await expect(table.getByText(email)).toBeVisible()

	const scroll = await table.evaluate((element) => {
		const scroller = element.closest('.dataviews-layout__container') as HTMLElement
		return scroller.scrollWidth - scroller.clientWidth
	})
	expect(scroll).toBeLessThanOrEqual(0)
	await expect(table.locator('.dataviews-view-table__actions-column--stuck')).toHaveCount(0)
	for (const column of ['Name', 'Status', 'Role', 'Posts']) {
		await expect(table.getByRole('columnheader', { name: column, exact: true })).toBeVisible()
	}
})

test('lays Add User out in one column on a phone, each field as wide as the form and Add User under them', async ({
	page,
}) => {
	await page.goto('/admin/users/new')
	const form = page.locator('form.godmin-form')
	const role = page.getByRole('combobox', { name: 'Role' })
	await expect(role).toBeVisible()

	const edges = await edgesOf(form)
	const email = page.getByRole('textbox', { name: /^Email/ })
	const name = page.getByRole('textbox', { name: /^Name/ })
	const rows = [
		{ label: 'Email', starts: email, ends: email },
		{ label: 'Name', starts: name, ends: name },
		{ label: 'Password', starts: page.getByLabel('Password', { exact: true }), ends: page.getByLabel('Hide password') },
		{ label: 'Role', starts: role, ends: role },
	]
	for (const { label, starts, ends } of rows) {
		const left = (await edgesOf(starts)).left - edges.left
		const right = edges.right - (await edgesOf(ends)).right
		expect(Math.abs(left), `${label} starts at the left of the form`).toBeLessThanOrEqual(1)
		expect(Math.abs(right), `${label} ends at the right of the form`).toBeLessThanOrEqual(1)
	}
	const roleBox = await role.boundingBox()
	const submit = await page.getByRole('button', { name: 'Add User', exact: true }).boundingBox()
	expect(submit?.y ?? 0).toBeGreaterThanOrEqual((roleBox?.y ?? 0) + (roleBox?.height ?? 0))
})

for (const { reads, condition, name } of LONG_NAMES) {
	test(`cuts an author name ${reads} with an ellipsis and keeps the posts table in its card at 1280, as WordPress does`, async ({
		page,
		playwright,
	}) => {
		const besideTitle = `A post signed with a short name beside the ${condition} one ${RUN}`
		const created = await page.request.post('/api/content', { data: { type: 'post', title: besideTitle } })
		expect(created.status()).toBe(201)
		const beside = (await created.json()) as { id: string }
		const post = await postWrittenBy(page, playwright.request, condition, name)
		await page.setViewportSize(LAPTOP)

		for (const { tab, narrowing } of POST_TABS) {
			const short = await postsTableFor(page, narrowing, besideTitle)
			const long = await postsTableFor(page, narrowing, post.title)
			const row = page.getByRole('row').filter({ has: page.getByRole('link', { name: post.title, exact: true }) })
			const author = row.getByText(name, { exact: true })
			await expect(author).toBeVisible()

			expect(await cutOf(author), `${tab} cuts the name`).toEqual({
				textOverflow: 'ellipsis',
				clipped: true,
				insideCell: true,
			})
			expect.soft(long.spill, `${tab} runs the table past its card`).toBeLessThanOrEqual(0)
			const stuck = long.table.locator('.dataviews-view-table__actions-column--stuck')
			await expect.soft(stuck, `${tab} sticks the actions over the columns`).toHaveCount(0)
			expect.soft(long.title, `${tab} narrows the title`).toBeGreaterThanOrEqual(short.title)
		}
		await page.request.delete(`/api/content/${post.id}?force=true`)
		await page.request.delete(`/api/content/${beside.id}?force=true`)
	})
}
