// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)

const PHONE = { width: 390, height: 844 }

const DESKTOP = { width: 1280, height: 720 }

const madeGroups: number[] = []

const madeTypes: string[] = []

test.afterEach(async ({ page }) => {
	for (const id of madeGroups.splice(0)) {
		await page.request.delete(`/api/groups/${id}`)
	}
	for (const key of madeTypes.splice(0)) {
		await page.request.delete(`/api/types/${key}`)
	}
})

/**
 * Registers a type of its own and a group placed on that type alone through the API, so no other spec meets its fields.
 * @param page - The page whose session registers them.
 * @param word - The word the type key, its names and the group title are made from.
 * @returns The id and the title of the group.
 */
async function groupOfItsOwn(page: Page, word: string) {
	const key = `${word.toLowerCase()}-${RUN}`
	const typed = await page.request.post('/api/types', {
		data: {
			key,
			singular_label: `${word} ${RUN}`,
			plural_label: `${word}s ${RUN}`,
			description: '',
			route_word: `${word.toLowerCase()}s-${RUN}`,
		},
	})
	expect(typed.status()).toBe(201)
	madeTypes.push(key)
	const title = `${word} details ${RUN}`
	const grouped = await page.request.post('/api/groups', {
		data: { title, location: [[{ source: 'content_type', operator: '==', value: key }]] },
	})
	expect(grouped.status()).toBe(201)
	const { id } = (await grouped.json()) as { id: number }
	madeGroups.push(id)
	return { id, title }
}

/**
 * Declares an optional text field in a group through the API.
 * @param page - The page whose session declares it.
 * @param group - The id of the group that holds it.
 * @param label - The name of the field, its key made from it.
 */
async function textFieldIn(page: Page, group: number, label: string) {
	const answer = await page.request.post(`/api/groups/${group}/fields`, {
		data: { key: label.toLowerCase(), label, kind: 'text', many: false, required: false },
	})
	expect(answer.status()).toBe(201)
}

/**
 * Declares an optional text field in a group, shown only while its source field reads Two hours.
 * @param page - The page whose session declares it.
 * @param group - The id of the group that holds it.
 * @param label - The name of the field, its key made from it.
 * @param source - The key of the field whose value shows it.
 */
async function fieldShownBy(page: Page, group: number, label: string, source: string) {
	const answer = await page.request.post(`/api/groups/${group}/fields`, {
		data: {
			key: label.toLowerCase(),
			label,
			kind: 'text',
			many: false,
			required: false,
			settings: { conditions: [[{ source, operator: '==', value: 'Two hours' }]] },
		},
	})
	expect(answer.status()).toBe(201)
}

/**
 * Declares a section in a group through the API, holding one optional text field.
 * @param page - The page whose session declares them.
 * @param group - The id of the group that holds the section.
 * @param label - The name of the section, its key made from it.
 * @param inside - The name of the text field inside it, its key made from it.
 */
async function sectionIn(page: Page, group: number, label: string, inside: string) {
	const section = await page.request.post(`/api/groups/${group}/fields`, {
		data: { key: label.toLowerCase(), label, kind: 'section', many: false, required: false },
	})
	expect(section.status()).toBe(201)
	const field = await page.request.post(`/api/groups/${group}/fields/${label.toLowerCase()}`, {
		data: { key: inside.toLowerCase(), label: inside, kind: 'text', many: false, required: false },
	})
	expect(field.status()).toBe(201)
}

/**
 * Returns the words of each button and badge beside the label in every Label cell, with the lines each takes.
 * @param page - The group page.
 * @returns The words and the count of lines of each.
 */
async function labelCellLines(page: Page) {
	return page.getByRole('main').evaluate((main) => {
		const badges = ['Required', 'Unsaved changes', 'Shadowed']
		const cells = [...main.querySelectorAll('table.godmin-table > tbody > tr:first-child > td:first-child')]
		const held = cells.flatMap((cell) =>
			[...cell.querySelectorAll('*')].filter(
				(node) =>
					(node.tagName === 'BUTTON' && !node.classList.contains('godmin-table__toggle')) ||
					(node.children.length === 0 && badges.includes(node.textContent ?? '')),
			),
		)
		return held.map((node) => {
			const words = document.createRange()
			words.selectNodeContents(node)
			const tops = new Set([...words.getClientRects()].map((line) => Math.round(line.top)))
			return { words: node.textContent, lines: tops.size }
		})
	})
}

/**
 * Returns the group of the rule row a trash takes away.
 * @param trash - The trash of the row.
 * @returns The row's group.
 */
function ruleOf(trash: Locator): Locator {
	return trash.locator('xpath=ancestor::*[@role="group"][1]')
}

/**
 * Returns whether a row of controls wraps and stays inside its tab panel or region, and how far a box up to it runs
 * sideways.
 * @param row - The row of controls.
 * @returns Whether it takes more than one line, whether every control sits inside the holder, and the widest overrun.
 */
async function fitOf(row: Locator) {
	return row.evaluate((node) => {
		const holder = node.closest('[role="tabpanel"], [role="region"]') as HTMLElement
		const edges = holder.getBoundingClientRect()
		const controls = [...node.querySelectorAll('[role="combobox"], input, button')]
			.filter((control) => control.closest('[aria-hidden="true"]') === null)
			.map((control) => control.getBoundingClientRect())
		let sideways = holder.scrollWidth - holder.clientWidth
		for (let held = node; held !== holder; held = held.parentElement as HTMLElement) {
			sideways = Math.max(sideways, held.scrollWidth - held.clientWidth)
		}
		return {
			wrapped: new Set(controls.map((control) => Math.round(control.bottom))).size > 1,
			inside: controls.every(
				(control) =>
					Math.round(control.left) >= Math.round(edges.left) && Math.round(control.right) <= Math.round(edges.right),
			),
			sideways,
		}
	})
}

/**
 * Returns the row of the fields table on a group page that shows a field's label.
 * @param page - The group page.
 * @param label - The label the row shows.
 * @returns The row.
 */
function fieldRow(page: Page, label: string): Locator {
	const fields = page.getByRole('main').getByRole('region', { name: 'Fields' })
	return fields.getByRole('row').filter({ has: page.getByText(label, { exact: true }) })
}

/**
 * Opens a field in place from the label in its row and returns the row that opened under it.
 * @param page - The group page.
 * @param label - The label the field shows.
 * @returns The row holding the open part.
 */
async function openField(page: Page, label: string): Promise<Locator> {
	const toggle = fieldRow(page, label).getByRole('button', { name: label, exact: true })
	await toggle.click()
	await expect(toggle).toHaveAttribute('aria-expanded', 'true')
	return fieldRow(page, label).locator('xpath=following-sibling::tr[1]')
}

/**
 * Returns the link back to the Field Groups list in the breadcrumbs of a group page.
 * @param main - The main work of the page.
 * @returns The breadcrumb link.
 */
function backToList(main: Locator): Locator {
	return main.getByRole('navigation', { name: 'Breadcrumbs' }).getByRole('link', { name: 'Field Groups' })
}

test('walks from the list to a group page by its title, adds a field there and comes back by the breadcrumb', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page, 'Lesson')
	await page.goto('/admin/field-groups')

	const list = page.getByRole('region', { name: 'Field Groups' })
	await list.getByRole('link', { name: group.title, exact: true }).click()

	await expect(page).toHaveURL(new RegExp(`/admin/field-groups/${group.id}$`))
	const main = page.getByRole('main')
	await expect(main.getByRole('heading', { level: 2, name: group.title })).toBeVisible()
	await expect(main.getByRole('heading', { name: 'No fields yet', exact: true })).toBeVisible()
	await main.getByLabel('Label', { exact: true }).fill('Duration')
	await main.getByRole('button', { name: 'Add field', exact: true }).click()
	await expect(page.locator('.godmin-toast', { hasText: 'Duration declared.' })).toBeVisible()
	await expect(fieldRow(page, 'Duration')).toBeVisible()

	await backToList(main).click()

	await expect(page).toHaveURL(/\/admin\/field-groups$/)
	const row = list.getByRole('row', { name: new RegExp(group.title) })
	await expect(row.getByRole('cell', { name: '1', exact: true })).toBeVisible()
})

test('opens a group page from its typed address and names an address no group holds', async ({ page }) => {
	const group = await groupOfItsOwn(page, 'Recipe')

	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	await expect(main.getByRole('heading', { level: 2, name: group.title })).toBeVisible()
	await expect(backToList(main)).toHaveAttribute('href', '/admin/field-groups')

	await page.goto('/admin/field-groups/2147483647')
	await expect(main.getByRole('heading', { level: 2, name: 'Field Group', exact: true })).toBeVisible()
	await expect(main.getByRole('alert')).toHaveText(
		'This field group does not exist, and it may have been deleted. Go back to the list to see what is there.',
	)
})

test('renames a group in its details, saves it from the header and reads the new title after a reload', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page, 'Course')
	const renamed = `Course outline ${RUN}`
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const details = main.getByRole('region', { name: 'Field group settings' })

	await details.getByRole('textbox', { name: 'Title' }).fill(renamed)
	await expect(main.getByRole('heading', { level: 2, name: renamed })).toBeVisible()
	await main.getByRole('button', { name: 'Save', exact: true }).click()
	await expect(page.locator('.godmin-toast', { hasText: `${renamed} updated.` })).toBeVisible()
	await page.reload()

	await expect(main.getByRole('heading', { level: 2, name: renamed })).toBeVisible()
	await expect(details.getByRole('textbox', { name: 'Title' })).toHaveValue(renamed)
})

test('asks before a draft leaves by the breadcrumb, stays on Keep editing and lands on the list on Discard', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page, 'Workshop')
	const drafted = `Workshop draft ${RUN}`
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const title = main.getByRole('region', { name: 'Field group settings' }).getByRole('textbox', { name: 'Title' })
	await title.fill(drafted)

	await backToList(main).click()
	const asking = page.getByRole('alertdialog', { name: 'Discard changes?' })
	await asking.getByRole('button', { name: 'Keep editing', exact: true }).click()
	await expect(asking).toBeHidden()
	await expect(page).toHaveURL(new RegExp(`/admin/field-groups/${group.id}$`))
	await expect(title).toHaveValue(drafted)

	await backToList(main).click()
	await asking.getByRole('button', { name: 'Discard', exact: true }).click()

	await expect(page).toHaveURL(/\/admin\/field-groups$/)
	const list = page.getByRole('region', { name: 'Field Groups' })
	await expect(list.getByRole('link', { name: group.title, exact: true })).toBeVisible()
})

test('moves a field at once and stores the newest order once the order on its way is answered', async ({ page }) => {
	const group = await groupOfItsOwn(page, 'Seminar')
	await textFieldIn(page, group.id, 'Duration')
	await textFieldIn(page, group.id, 'Summary')
	const sent: unknown[] = []
	let release = () => {}
	const held = new Promise<void>((resolve) => {
		release = resolve
	})
	await page.route(`**/api/groups/${group.id}/fields/order`, async (route) => {
		sent.push(route.request().postDataJSON())
		await held
		await route.continue()
	})
	await page.goto(`/admin/field-groups/${group.id}`)
	const trashes = page.getByRole('main').getByRole('region', { name: 'Fields' }).getByRole('button', { name: /^Delete / })
	await expect(trashes).toHaveCount(2)

	const down = fieldRow(page, 'Duration').getByRole('button', { name: 'Move Duration down' })
	await down.focus()
	await page.keyboard.press('Enter')
	await expect(trashes.first()).toHaveAccessibleName('Delete Summary')
	await expect(down).toBeFocused()
	await fieldRow(page, 'Duration').getByRole('button', { name: 'Move Duration up' }).click()
	await expect(trashes.first()).toHaveAccessibleName('Delete Duration')
	await expect.poll(() => sent).toEqual([{ order: ['summary', 'duration'] }])
	const reread = page.waitForResponse(
		(response) => response.request().method() === 'GET' && new URL(response.url()).pathname === '/api/groups',
	)
	release()

	await expect.poll(() => sent).toEqual([{ order: ['summary', 'duration'] }, { order: ['duration', 'summary'] }])
	await reread
	await expect(page.locator('.godmin-toast')).toHaveCount(0)
	await page.reload()
	await expect(trashes).toHaveCount(2)
	await expect(trashes.first()).toHaveAccessibleName('Delete Duration')
})

test('opens a field in place, saves it from Enter and from the header, and reads its badge after a reload', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page, 'Contact')
	await textFieldIn(page, group.id, 'Phone')
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const save = main.getByRole('button', { name: 'Save', exact: true })

	const part = await openField(page, 'Phone')
	await expect(fieldRow(page, 'Phone').getByRole('button', { name: 'Phone', exact: true })).toBeFocused()
	const label = part.getByRole('textbox', { name: 'Label' })
	await label.fill('Phone number')
	await label.press('Enter')
	await expect(page.locator('.godmin-toast', { hasText: `${group.title} updated.` })).toBeVisible()
	const renamed = fieldRow(page, 'Phone number').getByRole('button', { name: 'Phone number', exact: true })
	await expect(renamed).toHaveAttribute('aria-expanded', 'false')
	await expect(renamed).toBeFocused()
	const again = await openField(page, 'Phone number')
	await again.getByRole('tab', { name: 'Validation' }).click()
	await again.getByRole('checkbox', { name: 'Required' }).click()
	await save.click()
	await expect(save).toHaveAttribute('aria-disabled', 'true')
	await page.reload()

	const row = fieldRow(page, 'Phone number')
	await expect(row.getByText('Required', { exact: true })).toBeVisible()
	await expect(row.getByRole('button', { name: 'Phone number', exact: true })).toHaveAttribute('aria-expanded', 'false')
})

test('keeps a field whose save was rejected open, the server words under the setting it named', async ({ page }) => {
	const group = await groupOfItsOwn(page, 'Library')
	await textFieldIn(page, group.id, 'Phone')
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')

	const part = await openField(page, 'Phone')
	await part.getByRole('textbox', { name: 'Default' }).fill('Maria Perez')
	await part.getByRole('tab', { name: 'Validation' }).click()
	await part.getByRole('spinbutton', { name: 'Longest' }).fill('5')
	await main.getByRole('button', { name: 'Save', exact: true }).click()

	const said = 'The setting default falls outside the others. Keep min below max and the default inside the bounds.'
	await expect(page.locator('.godmin-toast', { hasText: `Could not save Phone. ${said}` })).toBeVisible()
	await expect(part.getByRole('tab', { name: 'General' })).toHaveAttribute('aria-selected', 'true')
	const fallback = part.getByRole('textbox', { name: 'Default' })
	await expect(fallback).toHaveAccessibleDescription(said)
	await expect(fieldRow(page, 'Phone').getByRole('button', { name: 'Phone', exact: true })).toHaveAttribute(
		'aria-expanded',
		'true',
	)
	await fallback.fill('Maria')
	await expect(fallback).toHaveAccessibleDescription('')
})

test('puts the focus on the trash of the field taking the place of a deleted one', async ({ page }) => {
	const group = await groupOfItsOwn(page, 'Gallery')
	await textFieldIn(page, group.id, 'Duration')
	await textFieldIn(page, group.id, 'Summary')
	await page.goto(`/admin/field-groups/${group.id}`)

	await fieldRow(page, 'Duration').getByRole('button', { name: 'Delete Duration' }).click()
	const asking = page.getByRole('alertdialog', { name: 'Delete "Duration"?' })
	await asking.getByRole('button', { name: 'Delete the field', exact: true }).click()

	await expect(fieldRow(page, 'Duration')).toHaveCount(0)
	await expect(fieldRow(page, 'Summary').getByRole('button', { name: 'Delete Summary' })).toBeFocused()
})

test('keeps an open field inside its region on a phone, its open part padded 16px', async ({ page }) => {
	const group = await groupOfItsOwn(page, 'Clinic')
	await textFieldIn(page, group.id, 'Phone')
	await page.setViewportSize(PHONE)
	await page.goto(`/admin/field-groups/${group.id}`)

	const part = await openField(page, 'Phone')

	const body = part.locator('.godmin-table__detail-body')
	await expect(body.getByRole('textbox', { name: 'Label' })).toHaveValue('Phone')
	await expect
		.poll(
			() =>
				body.evaluate((node) => {
					const region = (node.closest('[role="region"]') as HTMLElement).getBoundingClientRect()
					const box = node.getBoundingClientRect()
					const style = getComputedStyle(node)
					return {
						inside: Math.round(box.left) >= Math.round(region.left) && Math.round(box.right) <= Math.round(region.right),
						padding: [style.paddingTop, style.paddingRight, style.paddingBottom, style.paddingLeft],
						spill: document.documentElement.scrollWidth - document.documentElement.clientWidth,
					}
				}),
			{ message: 'the open part fits its region' },
		)
		.toEqual({ inside: true, padding: ['16px', '16px', '16px', '16px'], spill: 0 })
})

test('keeps each button and badge beside a label on one line, in a container too, on a desktop and a phone', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page, 'Studio')
	await sectionIn(page, group.id, 'Author', 'Byline')
	await page.goto(`/admin/field-groups/${group.id}`)
	await openField(page, 'Author')
	const inside = page.getByRole('region', { name: 'Sub fields of Author' })
	const byline = inside.getByRole('row').filter({ has: page.getByText('Byline', { exact: true }) })
	const toggle = byline.getByRole('button', { name: 'Byline', exact: true })
	await toggle.click()
	await expect(toggle).toHaveAttribute('aria-expanded', 'true')
	await byline.locator('xpath=following-sibling::tr[1]').getByRole('textbox', { name: 'Label' }).fill('Byline line')
	await expect(byline.getByText('Unsaved changes', { exact: true })).toBeVisible()

	for (const size of [DESKTOP, PHONE]) {
		await page.setViewportSize(size)
		const words = (await labelCellLines(page)).map((held) => held.words)
		expect(words).toEqual(expect.arrayContaining(['Move', 'Unsaved changes']))
		await expect
			.poll(async () => (await labelCellLines(page)).filter((held) => held.lines !== 1), {
				message: `every button and badge beside a label takes one line at ${size.width}`,
			})
			.toEqual([])
	}
})

test('wraps a location rule and a field condition on a phone, every control inside the details and the tab', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page, 'Apprenticeship')
	await textFieldIn(page, group.id, 'Duration')
	await fieldShownBy(page, group.id, 'Summary', 'duration')
	await page.setViewportSize(PHONE)

	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const details = main.getByRole('region', { name: 'Field group settings' })
	const rule = ruleOf(details.getByRole('button', { name: 'Remove rule 1 of set 1' }))
	await expect(rule).toHaveAccessibleName('Rule 1 of set 1')
	await expect(rule.getByRole('combobox', { name: 'Value' })).toHaveText(`Apprenticeships ${RUN}`)
	expect.soft(await fitOf(rule), 'the rule row fits the details').toEqual({
		wrapped: true,
		inside: true,
		sideways: 0,
	})

	const summary = await openField(page, 'Summary')
	const tab = summary.getByRole('tab', { name: /^Conditional logic/ })
	await expect(tab.getByText('Active', { exact: true })).toBeVisible()
	await expect(tab).toHaveAccessibleName('Conditional logic Active')
	await tab.click()
	const rules = summary.getByRole('tabpanel', { name: /^Conditional logic/ })
	await expect(rules.getByRole('textbox', { name: 'Value' })).toHaveValue('Two hours')
	const condition = ruleOf(rules.getByRole('button', { name: 'Remove rule 1 of set 1' }))
	await expect(condition).toHaveAccessibleName('Rule 1 of set 1')
	await expect
		.poll(() => fitOf(condition), { message: 'the condition row fits the tab at 390' })
		.toEqual({ wrapped: true, inside: true, sideways: 0 })
	await expect
		.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), {
			message: 'the open field with its four tabs leaves the page unscrolled at 390',
		})
		.toBe(0)

	await page.setViewportSize(DESKTOP)
	await expect
		.poll(async () => {
			const { inside, sideways } = await fitOf(condition)
			return { inside, sideways }
		}, { message: 'the condition row fits the tab at 1280' })
		.toEqual({ inside: true, sideways: 0 })
})
