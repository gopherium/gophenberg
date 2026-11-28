// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { APIRequest, Locator, Page } from '@playwright/test'

import { baseURL, credentials } from '../env'

const PHONE = { width: 390, height: 844 }

const LAPTOP = { width: 1280, height: 800 }

const SMALL_LAPTOP = { width: 1024, height: 768 }

const LONG_TITLE =
	'A post whose title runs on well past the width any phone screen could ever offer a reader'

const UNBREAKABLE = 'W'.repeat(60)

const RUN = Math.random().toString(36).slice(2, 8)

/** How far apart, in pixels, the middle of a line of words and the middle of a button beside it may sit. */
const LEVEL_SLACK = 1

const LONG_NAME = Array.from({ length: 11 }, () => 'Maria Perez').join(' ')

const madeGroups: number[] = []

const madeTypes: string[] = []

const LONG_NAMES = [
	{ reads: 'of many words', condition: 'spaced', name: LONG_NAME },
	{ reads: 'with no spaces', condition: 'unspaced', name: LONG_NAME.replaceAll(' ', '') },
]

const POST_TABS = [
	{ tab: 'All', narrowing: '' },
	{ tab: 'Draft', narrowing: 'status=draft&' },
]

/** The moment the posts list writes at its widest in UTC, May with a two-digit day and a two-digit evening hour. */
const WIDEST_MOMENT = { at: '2026-05-28T23:11:00Z', reads: 'Modified: May 28, 2026 11:11 pm' }

/**
 * Dates every post the page lists at the widest moment.
 * @param page - The page whose post listings are dated.
 */
async function listedAtWidestMoment(page: Page) {
	await page.route('**/api/content?*', async (route) => {
		const answer = await route.fetch()
		const body = (await answer.json()) as { items: Record<string, unknown>[] }
		const items = body.items.map((item) => ({ ...item, updated_at: WIDEST_MOMENT.at }))
		await route.fulfill({ response: answer, json: { ...body, items } })
	})
}

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
 * Reports whether an element ends inside the content box of its table cell, short of the next column.
 * @param element - The element in its cell.
 * @returns Whether its right edge stays inside the content box of its cell.
 */
async function endsInsideCell(element: Locator) {
	return element.evaluate((node) => {
		const cell = node.closest('td') as HTMLElement
		const edge = cell.getBoundingClientRect().right - Number.parseFloat(getComputedStyle(cell).paddingRight)
		return Math.round(node.getBoundingClientRect().right) <= Math.round(edge)
	})
}

/**
 * Reports whether an element sits inside the visible box of the table scroller that holds it.
 * @param element - The element inside a DataViews table.
 * @returns Whether its left and right edges fall within the scroller.
 */
async function insideScroller(element: Locator) {
	return element.evaluate((node) => {
		const scroller = (node.closest('.dataviews-layout__container') as HTMLElement).getBoundingClientRect()
		const box = node.getBoundingClientRect()
		return Math.round(box.left) >= Math.round(scroller.left) && Math.round(box.right) <= Math.round(scroller.right)
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
 * Returns how far down the viewport the middle of the first line of words inside an element sits.
 * @param element - The element holding the words.
 * @returns The middle of the first line in pixels from the top of the viewport.
 */
async function lineMiddleOf(element: Locator) {
	return element.evaluate((node) => {
		const words = document.createRange()
		words.selectNodeContents(node)
		const [line] = [...words.getClientRects()]
		return line.top + line.height / 2
	})
}

/**
 * Returns how many lines the words inside an element take.
 * @param element - The element holding the words.
 * @returns The number of lines.
 */
async function linesOf(element: Locator) {
	return element.evaluate((node) => {
		const words = document.createRange()
		words.selectNodeContents(node)
		return new Set([...words.getClientRects()].map((line) => Math.round(line.top))).size
	})
}

/**
 * Returns how far down the viewport the middle of an element sits.
 * @param element - The element to measure.
 * @returns The middle in pixels from the top of the viewport.
 */
async function middleOf(element: Locator) {
	const box = await element.boundingBox()
	return (box?.y ?? 0) + (box?.height ?? 0) / 2
}

/**
 * Returns where a line after the box of a table cell sits against the first line in that box and the cell's edge.
 * @param element - The line after the cell box.
 * @returns Whether it starts under the first line and level with its start, whether it ends inside the cell, and
 * its font size.
 */
async function underCellBox(element: Locator) {
	return element.evaluate((node) => {
		const cell = node.closest('td') as HTMLElement
		const first = (cell.querySelector('.godmin-table__cell > *') as HTMLElement).getBoundingClientRect()
		const edge = cell.getBoundingClientRect().right - Number.parseFloat(getComputedStyle(cell).paddingRight)
		const box = node.getBoundingClientRect()
		return {
			under: Math.round(box.top) >= Math.round(first.bottom),
			level: Math.round(box.left) === Math.round(first.left),
			inside: Math.round(box.right) <= Math.round(edge),
			size: getComputedStyle(node).fontSize,
		}
	})
}

/**
 * Returns how far the table region holding an element scrolls sideways.
 * @param element - The element inside a godmin table region.
 * @returns The overflow of the region in pixels.
 */
async function sidewaysOf(element: Locator) {
	return element.evaluate((node) => {
		const region = node.closest('.godmin-table-scroll') as HTMLElement
		return region.scrollWidth - region.clientWidth
	})
}

/**
 * Returns how the band holding an open field's Close field sits in the open part.
 * @param close - The Close field button.
 * @returns The top line of the band, its height, whether the button sits in its middle, whether it spans the open
 * part, and whether it comes last there.
 */
async function footerOf(close: Locator) {
	return close.evaluate((node) => {
		const band = node.parentElement as HTMLElement
		const part = band.parentElement as HTMLElement
		const style = getComputedStyle(band)
		const box = band.getBoundingClientRect()
		const button = node.getBoundingClientRect()
		const above = button.top - box.top - Number.parseFloat(style.borderTopWidth)
		return {
			line: `${style.borderTopWidth} ${style.borderTopStyle}`,
			height: Math.round(box.height),
			centred: Math.abs(above - (box.bottom - button.bottom)) <= 1,
			spans: Math.round(box.width) === Math.round(part.getBoundingClientRect().width),
			last: part.lastElementChild === band,
		}
	})
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

/**
 * Registers a type of its own and a group placed on that type alone, holding one text field, through the API.
 * @param page - The page of the signed in administrator.
 * @returns The id of the group.
 */
async function groupOfItsOwn(page: Page): Promise<number> {
	const key = `fitting-${RUN}`
	const typed = await page.request.post('/api/types', {
		data: {
			key,
			singular_label: `Fitting ${RUN}`,
			plural_label: `Fittings ${RUN}`,
			description: '',
			route_word: `fittings-${RUN}`,
		},
	})
	expect(typed.status()).toBe(201)
	madeTypes.push(key)
	const grouped = await page.request.post('/api/groups', {
		data: { title: `${LONG_TITLE} ${RUN}`, location: [[{ source: 'content_type', operator: '==', value: key }]] },
	})
	expect(grouped.status()).toBe(201)
	const { id } = (await grouped.json()) as { id: number }
	madeGroups.push(id)
	const field = await page.request.post(`/api/groups/${id}/fields`, {
		data: { key: 'measure', label: 'Measure', kind: 'text', many: false, required: false },
	})
	expect(field.status()).toBe(201)
	return id
}

test.beforeEach(async ({ page }) => {
	await page.setViewportSize(PHONE)
})

test.afterEach(async ({ page }) => {
	for (const id of madeGroups.splice(0)) {
		await page.request.delete(`/api/groups/${id}`)
	}
	for (const key of madeTypes.splice(0)) {
		await page.request.delete(`/api/types/${key}`)
	}
})

test('seeds a hostile post and fits every screen to a phone', async ({ page }) => {
	const created = await page.request.post('/api/content', {
		data: { type: 'post', title: `${LONG_TITLE} ${UNBREAKABLE}` },
	})
	expect(created.status()).toBe(201)
	const group = await groupOfItsOwn(page)

	const screens = [
		'/admin/',
		'/admin/content/post',
		'/admin/content/post?status=trash',
		'/admin/content/page',
		'/admin/content/category',
		'/admin/content-types',
		'/admin/field-groups',
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
	await page.goto(`/admin/field-groups/${group}`)
	const fields = page.getByRole('main').getByRole('region', { name: 'Fields' })
	await expect(fields.getByRole('row').filter({ has: page.getByText('Measure', { exact: true }) })).toBeVisible()
	const grouped = await spills(page)
	expect(grouped.document, 'the page of a group spills the document').toBeLessThanOrEqual(0)
	expect(grouped.canvas, 'the page of a group spills the canvas').toBeLessThanOrEqual(0)
})

test('sets the field count of a group level with its Move up button in the field groups table at 1280', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page)
	const second = await page.request.post(`/api/groups/${group}/fields`, {
		data: { key: 'width', label: 'Width', kind: 'text', many: false, required: false },
	})
	expect(second.status()).toBe(201)
	const title = `${LONG_TITLE} ${RUN}`
	await page.setViewportSize(LAPTOP)

	await page.goto('/admin/field-groups')
	const listed = page.getByRole('row').filter({ has: page.getByRole('link', { name: title, exact: true }) })
	const up = listed.getByRole('button', { name: `Move ${title} up`, exact: true })
	await expect(up).toBeVisible()

	const written = listed.getByText('2 fields', { exact: true })
	const count = Math.abs((await lineMiddleOf(written)) - (await middleOf(up)))
	expect(count, 'the field count sits level with Move up at 1280').toBeLessThanOrEqual(LEVEL_SLACK)
	expect(await linesOf(written), 'the field count takes one line at 1280').toBe(1)
	expect(await sidewaysOf(listed), 'the field groups table scrolls sideways at 1280').toBe(0)
})

test('lists the field groups at 390 with the count under the title, the header hidden and nothing scrolling sideways', async ({
	page,
}) => {
	await groupOfItsOwn(page)
	const title = `${LONG_TITLE} ${RUN}`

	await page.goto('/admin/field-groups')
	const listed = page.getByRole('row').filter({ has: page.getByRole('link', { name: title, exact: true }) })
	const link = listed.getByRole('link', { name: title, exact: true })
	await expect(listed.getByRole('button', { name: `Move ${title} up`, exact: true })).toBeVisible()

	const count = await listed.getByText('1 field', { exact: true }).boundingBox()
	const named = await link.boundingBox()
	expect(count?.y ?? 0, 'the field count sits under the title at 390').toBeGreaterThanOrEqual(
		(named?.y ?? 0) + (named?.height ?? 0),
	)
	expect(await sidewaysOf(listed), 'the field groups list scrolls sideways at 390').toBe(0)
	const header = await listed.evaluate((node) => {
		const box = (node.closest('table') as HTMLTableElement).tHead?.getBoundingClientRect()
		return { width: box?.width, height: box?.height }
	})
	expect(header, 'the column headers hide from sight at 390').toEqual({ width: 1, height: 1 })
})

test('scrolls the field groups table sideways in its region at 1024 beside the sidebar, every Delete in reach and nothing past the page', async ({
	page,
}) => {
	await groupOfItsOwn(page)
	const idle = await page.request.post('/api/groups', {
		data: { title: `Apprenticeshipcoordination ${RUN}`, location: [[{ source: 'content_type', operator: '==', value: 'post' }]] },
	})
	expect(idle.status()).toBe(201)
	const made = (await idle.json()) as { id: number; updated_at: string }
	madeGroups.push(made.id)
	const paused = await page.request.patch(`/api/groups/${made.id}`, { data: { updated_at: made.updated_at, active: false } })
	expect(paused.status()).toBe(200)
	await page.setViewportSize(SMALL_LAPTOP)

	await page.goto('/admin/field-groups')
	const region = page.getByRole('region', { name: 'Field Groups' })
	await expect(region.getByText('Inactive', { exact: true })).toBeVisible()

	expect(await spills(page), 'the field groups table spills past the page at 1024').toEqual({ document: 0, canvas: 0 })
	expect(await sidewaysOf(region), 'the field groups region scrolls sideways at 1024').toBeGreaterThan(0)
	const reached: { button: { left: number; right: number }; region: { left: number; right: number } }[] = []
	for (const button of await region.getByRole('button', { name: 'Delete', exact: true }).all()) {
		await button.scrollIntoViewIfNeeded()
		reached.push({ button: await edgesOf(button), region: await edgesOf(region) })
	}
	expect(
		reached.filter((at) => at.button.left < at.region.left || at.button.right > at.region.right),
		'a Delete button stays outside the field groups region once scrolled to at 1024',
	).toEqual([])
})

test('lists the fields of a group page at 1280 and 390, the label level with the trash and the API name under them', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page)

	for (const size of [LAPTOP, PHONE]) {
		await page.setViewportSize(size)
		await page.goto(`/admin/field-groups/${group}`)
		const fields = page.getByRole('main').getByRole('region', { name: 'Fields' })
		const row = fields.getByRole('row').filter({ has: page.getByText('Measure', { exact: true }) })
		const trash = row.getByRole('button', { name: 'Delete Measure', exact: true })
		await expect(trash).toBeVisible()

		const label = Math.abs((await lineMiddleOf(row.getByText('Measure', { exact: true }))) - (await middleOf(trash)))
		expect(label, `the label sits level with the trash at ${size.width}`).toBeLessThanOrEqual(LEVEL_SLACK)
		const key = await row.getByText('measure', { exact: true }).boundingBox()
		const bin = await trash.boundingBox()
		expect(key?.y ?? 0, `the API name sits under the trash at ${size.width}`).toBeGreaterThanOrEqual(
			(bin?.y ?? 0) + (bin?.height ?? 0),
		)
		expect(await sidewaysOf(row), `the fields table scrolls sideways at ${size.width}`).toBe(0)
	}
})

test('lists the labels a closed container holds under its badge in the Kind cell at 1280 and 390, nothing scrolling sideways', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page)
	const declared = [
		{ above: '', key: 'team', label: 'Team', kind: 'repeater' },
		{ above: '/team', key: 'name', label: 'Name', kind: 'text' },
		{ above: '/team', key: 'role', label: 'Role', kind: 'text' },
	]
	for (const { above, key, label, kind } of declared) {
		const made = await page.request.post(`/api/groups/${group}/fields${above}`, {
			data: { key, label, kind, many: false, required: false },
		})
		expect(made.status(), `${label} is declared`).toBe(201)
	}

	for (const size of [LAPTOP, PHONE]) {
		await page.setViewportSize(size)
		await page.goto(`/admin/field-groups/${group}`)
		const fields = page.getByRole('main').getByRole('region', { name: 'Fields', exact: true })
		const row = fields.getByRole('row').filter({ has: page.getByRole('button', { name: 'Team', exact: true }) })
		const labels = row.getByText('Name, Role', { exact: true })
		await expect(labels, `the labels Team holds show at ${size.width}`).toBeVisible()

		expect(await underCellBox(labels), `the labels sit under the badge in the Kind cell at ${size.width}`).toEqual({
			under: true,
			level: true,
			inside: true,
			size: '12px',
		})
		expect(await linesOf(labels), `the labels take one line at ${size.width}`).toBe(1)
		expect(await sidewaysOf(row), `the fields table scrolls sideways at ${size.width}`).toBe(0)
		const overflow = await spills(page)
		expect(overflow.document, `the labels spill the document at ${size.width}`).toBeLessThanOrEqual(0)
		expect(overflow.canvas, `the labels spill the canvas at ${size.width}`).toBeLessThanOrEqual(0)
	}
})

test('keeps the form adding a field inside an open container in the page at 1280 and 390, nothing scrolling sideways', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page)
	const team = await page.request.post(`/api/groups/${group}/fields`, {
		data: { key: 'team', label: 'Team', kind: 'repeater', many: false, required: false },
	})
	expect(team.status()).toBe(201)

	for (const size of [LAPTOP, PHONE]) {
		await page.setViewportSize(size)
		await page.goto(`/admin/field-groups/${group}`)
		const fields = page.getByRole('main').getByRole('region', { name: 'Fields', exact: true })
		const toggle = fields.getByRole('button', { name: 'Team', exact: true })
		await toggle.click()
		await expect(toggle).toHaveAttribute('aria-expanded', 'true')
		const form = fields.getByRole('form', { name: 'Add field to Team', exact: true })
		await expect(form, `the form adding a field to Team shows at ${size.width}`).toBeVisible()

		const add = form.getByRole('button', { name: 'Add field to Team', exact: true })
		const region = await edgesOf(fields)
		const controls = [
			{ name: 'the form', control: form },
			{ name: 'Label', control: form.getByRole('textbox', { name: 'Label', exact: true }) },
			{ name: 'Kind', control: form.getByRole('combobox', { name: 'Kind', exact: true }) },
			{ name: 'Add field', control: add },
		]
		for (const { name, control } of controls) {
			const edges = await edgesOf(control)
			expect(edges.left, `${name} starts inside the fields region at ${size.width}`).toBeGreaterThanOrEqual(
				region.left,
			)
			expect(edges.right, `${name} ends inside the fields region at ${size.width}`).toBeLessThanOrEqual(
				region.right,
			)
		}
		expect(region.right, `the fields region ends inside the page at ${size.width}`).toBeLessThanOrEqual(size.width)
		expect(await linesOf(add), `Add field takes one line at ${size.width}`).toBe(1)
		expect(await sidewaysOf(form), `the fields region scrolls sideways at ${size.width}`).toBe(0)
		const overflow = await spills(page)
		expect(overflow.document, `the open container spills the document at ${size.width}`).toBeLessThanOrEqual(0)
		expect(overflow.canvas, `the open container spills the canvas at ${size.width}`).toBeLessThanOrEqual(0)
	}
})

test('ends every open field with Close field in a band under a line across the field at 1280 and 390', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page)
	const team = await page.request.post(`/api/groups/${group}/fields`, {
		data: { key: 'team', label: 'Team', kind: 'repeater', many: false, required: false },
	})
	expect(team.status()).toBe(201)

	for (const size of [LAPTOP, PHONE]) {
		await page.setViewportSize(size)
		await page.goto(`/admin/field-groups/${group}`)
		const fields = page.getByRole('main').getByRole('region', { name: 'Fields', exact: true })
		for (const label of ['Measure', 'Team']) {
			const toggle = fields.getByRole('button', { name: label, exact: true })
			await toggle.click()
			await expect(toggle).toHaveAttribute('aria-expanded', 'true')
		}
		const closes = fields.getByRole('button', { name: 'Close field', exact: true })
		await expect(closes).toHaveCount(2)

		for (const close of await closes.all()) {
			expect(await footerOf(close), `the band of Close field at ${size.width}`).toEqual({
				line: '1px solid',
				height: 72,
				centred: true,
				spans: true,
				last: true,
			})
		}
		const form = await fields.getByRole('form', { name: 'Add field to Team', exact: true }).boundingBox()
		const line = await closes.last().evaluate((node) => (node.parentElement as HTMLElement).getBoundingClientRect().top)
		expect(line, `the line over Close field sits under the form adding a field at ${size.width}`).toBeGreaterThanOrEqual(
			(form?.y ?? 0) + (form?.height ?? 0),
		)
	}
})

test('keeps every level of a field three deep inside a flexible field in the page at 390, nothing spilling', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page)
	const levels = [
		{ above: '', key: 'features', label: 'Features', kind: 'flexible' },
		{ above: '/features', key: 'hero', label: 'Hero', kind: 'layout' },
		{ above: '/features.hero', key: 'box', label: 'Box', kind: 'section' },
		{ above: '/features.hero.box', key: 'caption', label: 'Caption', kind: 'text' },
	]
	for (const { above, key, label, kind } of levels) {
		const made = await page.request.post(`/api/groups/${group}/fields${above}`, {
			data: { key, label, kind, many: false, required: false },
		})
		expect(made.status(), `${label} is declared`).toBe(201)
	}

	await page.goto(`/admin/field-groups/${group}`)
	const fields = page.getByRole('main').getByRole('region', { name: 'Fields', exact: true })
	for (const { label } of levels) {
		const toggle = fields.getByRole('button', { name: label, exact: true })
		await toggle.click()
		await expect(toggle).toHaveAttribute('aria-expanded', 'true')
	}
	const caption = fields
		.getByRole('row')
		.filter({ has: page.getByRole('button', { name: 'Caption', exact: true }) })
		.locator('xpath=following-sibling::tr[1]')
	const label = caption.getByRole('textbox', { name: 'Label', exact: true })
	await expect(label).toHaveValue('Caption')

	const adds = ['Add field to Box', 'Add field to Hero', 'Add layout to Features']
	for (const name of adds) {
		await expect(fields.getByRole('form', { name, exact: true }), `the form ${name} shows at 390`).toBeVisible()
	}
	const region = await edgesOf(fields)
	const controls = [
		{ name: 'the Label box of Caption', control: label },
		{ name: 'the tabs of Caption', control: caption.getByRole('tablist') },
		{ name: 'Close field of Caption', control: caption.getByRole('button', { name: 'Close field', exact: true }) },
		...adds.flatMap((name) => {
			const form = fields.getByRole('form', { name, exact: true })
			const box = form.getByRole('textbox', { name: 'Label', exact: true })
			return [
				{ name: `the Label box of ${name}`, control: box },
				{ name, control: form.getByRole('button', { name, exact: true }) },
			]
		}),
	]
	for (const { name, control } of controls) {
		const edges = await edgesOf(control)
		expect(edges.left, `${name} starts inside the fields region at 390`).toBeGreaterThanOrEqual(region.left)
		expect(edges.right, `${name} ends inside the fields region at 390`).toBeLessThanOrEqual(region.right)
	}
	for (const name of ['Fields', 'Layouts of Features', 'Sub fields of Hero', 'Sub fields of Box']) {
		const held = page.getByRole('main').getByRole('region', { name, exact: true })
		expect(await sidewaysOf(held), `${name} scrolls sideways at 390`).toBe(0)
	}
	const overflow = await spills(page)
	expect(overflow.document, 'the open levels spill the document at 390').toBeLessThanOrEqual(0)
	expect(overflow.canvas, 'the open levels spill the canvas at 390').toBeLessThanOrEqual(0)
})

test('keeps the posts table in its card at 1280 beside a long title and the widest date, its actions in view and its listed fields in reach', async ({
	page,
}) => {
	const title = `${LONG_TITLE} ${RUN}`
	const created = await page.request.post('/api/content', { data: { type: 'post', title } })
	expect(created.status()).toBe(201)
	const { id } = (await created.json()) as { id: string }
	await listedAtWidestMoment(page)
	await page.setViewportSize(LAPTOP)

	await page.goto(`/admin/content/post?search=${encodeURIComponent(title)}`)
	const table = page.locator('.dataviews-view-table')
	const link = table.getByRole('link', { name: title, exact: true })
	await expect(link).toBeVisible()
	await expect(table.getByText(WIDEST_MOMENT.reads, { exact: true })).toBeVisible()

	const overflow = await spills(page)
	expect(overflow.document, 'the table spills the document').toBeLessThanOrEqual(0)
	expect(overflow.canvas, 'the table spills the canvas').toBeLessThanOrEqual(0)
	expect(await endsInsideCell(link), 'the title ends in its cell').toBe(true)
	const row = table.getByRole('row').filter({ has: page.getByRole('link', { name: title, exact: true }) })
	const actions = row.locator('.dataviews-view-table__actions-column')
	expect(await insideScroller(actions), 'the actions stay in view').toBe(true)
	const featured = page.getByRole('columnheader', { name: 'Featured' })
	await featured.scrollIntoViewIfNeeded()
	expect(await insideScroller(featured), 'the listed field scrolls into view').toBe(true)
	await page.getByRole('button', { name: 'View options' }).click()
	await expect(page.locator('.dataviews-field-control').getByRole('button', { name: 'Featured' })).toBeVisible()
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

test('lays Add User out in one column on a phone, each field as wide as the form and Send invitation under them', async ({
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
		{ label: 'Role', starts: role, ends: role },
	]
	for (const { label, starts, ends } of rows) {
		const left = (await edgesOf(starts)).left - edges.left
		const right = edges.right - (await edgesOf(ends)).right
		expect(Math.abs(left), `${label} starts at the left of the form`).toBeLessThanOrEqual(1)
		expect(Math.abs(right), `${label} ends at the right of the form`).toBeLessThanOrEqual(1)
	}
	const roleBox = await role.boundingBox()
	const submit = await page.getByRole('button', { name: 'Send invitation', exact: true }).boundingBox()
	expect(submit?.y ?? 0).toBeGreaterThanOrEqual((roleBox?.y ?? 0) + (roleBox?.height ?? 0))
})

for (const { reads, condition, name } of LONG_NAMES) {
	test(`cuts an author name ${reads} with an ellipsis and keeps the posts table in its card at 1280 at the widest date, its actions in view, as WordPress does`, async ({
		page,
		playwright,
	}) => {
		const besideTitle = `A post signed with a short name beside the ${condition} one ${RUN}`
		const created = await page.request.post('/api/content', { data: { type: 'post', title: besideTitle } })
		expect(created.status()).toBe(201)
		const beside = (await created.json()) as { id: string }
		const post = await postWrittenBy(page, playwright.request, condition, name)
		await listedAtWidestMoment(page)
		await page.setViewportSize(LAPTOP)

		for (const { tab, narrowing } of POST_TABS) {
			const short = await postsTableFor(page, narrowing, besideTitle)
			const long = await postsTableFor(page, narrowing, post.title)
			const row = page.getByRole('row').filter({ has: page.getByRole('link', { name: post.title, exact: true }) })
			const author = row.getByText(name, { exact: true })
			await expect(author).toBeVisible()
			await expect(row.getByText(WIDEST_MOMENT.reads, { exact: true })).toBeVisible()

			expect(await cutOf(author), `${tab} cuts the name`).toEqual({
				textOverflow: 'ellipsis',
				clipped: true,
				insideCell: true,
			})
			const overflow = await spills(page)
			expect.soft(overflow.document, `${tab} spills the document`).toBeLessThanOrEqual(0)
			expect.soft(overflow.canvas, `${tab} spills the canvas`).toBeLessThanOrEqual(0)
			const actions = row.locator('.dataviews-view-table__actions-column')
			expect.soft(await insideScroller(actions), `${tab} keeps the actions in view`).toBe(true)
			expect.soft(long.title, `${tab} narrows the title`).toBeGreaterThanOrEqual(short.title)
		}
		await page.request.delete(`/api/content/${post.id}?force=true`)
		await page.request.delete(`/api/content/${beside.id}?force=true`)
	})
}
