// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)

const PHONE = { width: 390, height: 844 }

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
 * Waits until every animation inside an element has finished or been cancelled.
 * @param element - The element whose animations it waits on.
 */
async function settled(element: Locator) {
	await element.evaluate((node) =>
		Promise.allSettled(node.getAnimations({ subtree: true }).map((run) => run.finished)),
	)
}

/**
 * Returns whether a row of controls wraps and stays inside its holder, and how far a box up to it runs sideways.
 * @param row - The row of controls.
 * @returns Whether it takes more than one line, whether every control sits inside the holder, and the widest overrun.
 */
async function fitOf(row: Locator) {
	return row.evaluate((node) => {
		const holder = node.closest('[role="region"], [role="dialog"]') as HTMLElement
		const edges = holder.getBoundingClientRect()
		const controls = [...node.children].map((control) => control.getBoundingClientRect())
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
	await expect(main.getByText('This group holds no fields yet.')).toBeVisible()
	await main.getByLabel('Name', { exact: true }).fill('Duration')
	await main.getByRole('button', { name: 'Add field', exact: true }).click()
	await expect(page.locator('.godmin-toast', { hasText: 'Duration declared.' })).toBeVisible()
	await expect(main.getByRole('listitem', { name: 'Duration' })).toBeVisible()

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

test('wraps the actions of a field on a phone, each button whole on one line inside the row', async ({ page }) => {
	const group = await groupOfItsOwn(page, 'Notice')
	await textFieldIn(page, group.id, 'Duration')
	await page.setViewportSize(PHONE)

	await page.goto(`/admin/field-groups/${group.id}`)
	const field = page.getByRole('main').getByRole('listitem', { name: 'Duration' })
	const rename = field.getByRole('button', { name: 'Rename Duration' })
	await expect(rename).toBeVisible()

	const actions = rename.locator('xpath=..')
	const fit = await actions.evaluate((row) => {
		const edge = row.getBoundingClientRect().right
		const buttons = [...row.querySelectorAll('button')]
		return {
			heights: new Set(buttons.map((button) => Math.round(button.getBoundingClientRect().height))).size,
			inside: buttons.every((button) => Math.round(button.getBoundingClientRect().right) <= Math.round(edge)),
			oneLine: buttons.every((button) => {
				const label = document.createRange()
				label.selectNodeContents(button)
				return new Set([...label.getClientRects()].map((line) => Math.round(line.top))).size === 1
			}),
		}
	})
	expect(fit).toEqual({ heights: 1, inside: true, oneLine: true })
})

test('wraps a location rule and a field condition on a phone, every control inside the details and the dialog', async ({
	page,
}) => {
	const group = await groupOfItsOwn(page, 'Apprenticeship')
	await textFieldIn(page, group.id, 'Duration')
	await fieldShownBy(page, group.id, 'Summary', 'duration')
	await page.setViewportSize(PHONE)

	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const details = main.getByRole('region', { name: 'Field group settings' })
	const rule = details.getByRole('group', { name: 'Rule 1 of set 1' })
	await expect(rule.getByRole('combobox', { name: 'Value' })).toHaveText(`Apprenticeships ${RUN}`)
	expect.soft(await fitOf(rule), 'the rule row fits the details').toEqual({
		wrapped: true,
		inside: true,
		sideways: 0,
	})

	await main.getByRole('button', { name: 'Rules showing Summary' }).click()
	const dialog = page.getByRole('dialog', { name: 'Rules showing Summary' })
	await expect(dialog.getByRole('textbox', { name: 'Value' })).toHaveValue('Two hours')
	await settled(dialog)
	const condition = dialog.getByRole('button', { name: 'Remove', exact: true }).locator('xpath=..')
	expect.soft(await fitOf(condition), 'the condition row fits the dialog').toEqual({
		wrapped: true,
		inside: true,
		sideways: 0,
	})
})
