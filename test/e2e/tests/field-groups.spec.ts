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

/** A field group a test registered through the API. */
interface CreatedGroup {
	id: number
	title: string
}

/**
 * Registers a type and a group located on that type alone through the API.
 * @param page - The page whose session registers them.
 * @param noun - The noun the type key, its labels and the group title are made from.
 * @returns The created group.
 */
async function createIsolatedGroup(page: Page, noun: string): Promise<CreatedGroup> {
	const key = `${noun.toLowerCase()}-${RUN}`
	const typeResponse = await page.request.post('/api/types', {
		data: {
			key,
			singular_label: `${noun} ${RUN}`,
			plural_label: `${noun}s ${RUN}`,
			description: '',
			route_word: `${noun.toLowerCase()}s-${RUN}`,
		},
	})
	expect(typeResponse.status()).toBe(201)
	madeTypes.push(key)
	const title = `${noun} details ${RUN}`
	const groupResponse = await page.request.post('/api/groups', {
		data: { title, location: [[{ source: 'content_type', operator: '==', value: key }]] },
	})
	expect(groupResponse.status()).toBe(201)
	const { id } = (await groupResponse.json()) as { id: number }
	madeGroups.push(id)
	return { id, title }
}

/**
 * Declares an optional text field in a group through the API.
 * @param page - The page whose session declares it.
 * @param group - The group the field belongs to.
 * @param label - The name of the field, its key made from it.
 */
async function textFieldIn(page: Page, group: CreatedGroup, label: string) {
	const response = await page.request.post(`/api/groups/${group.id}/fields`, {
		data: { key: label.toLowerCase(), label, kind: 'text', many: false, required: false },
	})
	expect(response.status()).toBe(201)
}

/**
 * Declares an optional text field in a group, shown only while its source field reads Two hours.
 * @param page - The page whose session declares it.
 * @param group - The group the field belongs to.
 * @param label - The name of the field, its key made from it.
 * @param sourceKey - The key of the field whose value shows it.
 */
async function conditionalFieldIn(page: Page, group: CreatedGroup, label: string, sourceKey: string) {
	const response = await page.request.post(`/api/groups/${group.id}/fields`, {
		data: {
			key: label.toLowerCase(),
			label,
			kind: 'text',
			many: false,
			required: false,
			settings: { conditions: [[{ source: sourceKey, operator: '==', value: 'Two hours' }]] },
		},
	})
	expect(response.status()).toBe(201)
}

/**
 * Declares a section in a group through the API, with one optional text field inside it.
 * @param page - The page whose session declares them.
 * @param group - The group the section belongs to.
 * @param label - The name of the section, its key made from it.
 * @param childLabel - The name of the text field inside it, its key made from it.
 */
async function sectionIn(page: Page, group: CreatedGroup, label: string, childLabel: string) {
	const sectionResponse = await page.request.post(`/api/groups/${group.id}/fields`, {
		data: { key: label.toLowerCase(), label, kind: 'section', many: false, required: false },
	})
	expect(sectionResponse.status()).toBe(201)
	const childResponse = await page.request.post(`/api/groups/${group.id}/fields/${label.toLowerCase()}`, {
		data: { key: childLabel.toLowerCase(), label: childLabel, kind: 'text', many: false, required: false },
	})
	expect(childResponse.status()).toBe(201)
}

/**
 * Declares a container with no fields in a group through the API.
 * @param page - The page whose session declares it.
 * @param group - The group the container belongs to.
 * @param label - The name of the container, its key made from it.
 * @param kind - The kind of the container.
 */
async function containerIn(page: Page, group: CreatedGroup, label: string, kind: string) {
	const response = await page.request.post(`/api/groups/${group.id}/fields`, {
		data: { key: label.toLowerCase(), label, kind, many: false, required: false },
	})
	expect(response.status()).toBe(201)
}

/**
 * Returns the text of each button and badge beside the label in every Label cell, with the lines each takes.
 * @param page - The group page.
 * @returns The text and the count of lines of each.
 */
async function labelCellLines(page: Page) {
	return page.getByRole('main').evaluate((main) => {
		const badgeTexts = ['Required', 'Unsaved changes', 'Shadowed']
		const cells = [...main.querySelectorAll('table.godmin-table > tbody > tr:first-child > td:first-child')]
		const buttonAndBadgeNodes = cells.flatMap((cell) =>
			[...cell.querySelectorAll('*')].filter(
				(node) =>
					(node.tagName === 'BUTTON' && !node.classList.contains('godmin-table__toggle')) ||
					(node.children.length === 0 && badgeTexts.includes(node.textContent ?? '')),
			),
		)
		return buttonAndBadgeNodes.map((node) => {
			const range = document.createRange()
			range.selectNodeContents(node)
			const lineTops = new Set([...range.getClientRects()].map((lineRect) => Math.round(lineRect.top)))
			return { text: node.textContent, lineCount: lineTops.size }
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
 * @returns Whether it takes more than one line, whether every control sits inside the container, and the widest overrun.
 */
async function fitOf(row: Locator) {
	return row.evaluate((node) => {
		const container = node.closest('[role="tabpanel"], [role="region"]') as HTMLElement
		const containerRect = container.getBoundingClientRect()
		const controlRects = [...node.querySelectorAll('[role="combobox"], input, button')]
			.filter((control) => control.closest('[aria-hidden="true"]') === null)
			.map((control) => control.getBoundingClientRect())
		let maxOverflow = container.scrollWidth - container.clientWidth
		for (let ancestor = node; ancestor !== container; ancestor = ancestor.parentElement as HTMLElement) {
			maxOverflow = Math.max(maxOverflow, ancestor.scrollWidth - ancestor.clientWidth)
		}
		return {
			isWrapped: new Set(controlRects.map((controlRect) => Math.round(controlRect.bottom))).size > 1,
			isInside: controlRects.every(
				(controlRect) =>
					Math.round(controlRect.left) >= Math.round(containerRect.left) &&
					Math.round(controlRect.right) <= Math.round(containerRect.right),
			),
			maxOverflow,
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
	const group = await createIsolatedGroup(page, 'Lesson')
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
	await expect(row.getByRole('cell', { name: '1 field', exact: true })).toBeVisible()
})

test('opens a group page from its typed address and names an address no group holds', async ({ page }) => {
	const group = await createIsolatedGroup(page, 'Recipe')

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
	const group = await createIsolatedGroup(page, 'Course')
	const newTitle = `Course outline ${RUN}`
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const details = main.getByRole('region', { name: 'Field group settings' })

	await details.getByRole('textbox', { name: 'Title' }).fill(newTitle)
	await expect(main.getByRole('heading', { level: 2, name: newTitle })).toBeVisible()
	await main.getByRole('button', { name: 'Save', exact: true }).click()
	await expect(page.locator('.godmin-toast', { hasText: `${newTitle} updated.` })).toBeVisible()
	await page.reload()

	await expect(main.getByRole('heading', { level: 2, name: newTitle })).toBeVisible()
	await expect(details.getByRole('textbox', { name: 'Title' })).toHaveValue(newTitle)
})

test('asks before a draft leaves by the breadcrumb, stays on Keep editing and lands on the list on Discard', async ({
	page,
}) => {
	const group = await createIsolatedGroup(page, 'Workshop')
	const draftTitle = `Workshop draft ${RUN}`
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const titleInput = main.getByRole('region', { name: 'Field group settings' }).getByRole('textbox', { name: 'Title' })
	await titleInput.fill(draftTitle)

	await backToList(main).click()
	const discardDialog = page.getByRole('alertdialog', { name: 'Discard changes?' })
	await discardDialog.getByRole('button', { name: 'Keep editing', exact: true }).click()
	await expect(discardDialog).toBeHidden()
	await expect(page).toHaveURL(new RegExp(`/admin/field-groups/${group.id}$`))
	await expect(titleInput).toHaveValue(draftTitle)

	await backToList(main).click()
	await discardDialog.getByRole('button', { name: 'Discard', exact: true }).click()

	await expect(page).toHaveURL(/\/admin\/field-groups$/)
	const list = page.getByRole('region', { name: 'Field Groups' })
	await expect(list.getByRole('link', { name: group.title, exact: true })).toBeVisible()
})

test('moves a field at once and stores the newest order once the order on its way is answered', async ({ page }) => {
	const group = await createIsolatedGroup(page, 'Seminar')
	await textFieldIn(page, group, 'Duration')
	await textFieldIn(page, group, 'Summary')
	const orderBodies: unknown[] = []
	let release = () => {}
	const releasePromise = new Promise<void>((resolve) => {
		release = resolve
	})
	await page.route(`**/api/groups/${group.id}/fields/order`, async (route) => {
		orderBodies.push(route.request().postDataJSON())
		await releasePromise
		await route.continue()
	})
	await page.goto(`/admin/field-groups/${group.id}`)
	const fieldsRegion = page.getByRole('main').getByRole('region', { name: 'Fields' })
	const deleteButtons = fieldsRegion.getByRole('button', { name: /^Delete / })
	await expect(deleteButtons).toHaveCount(2)

	const moveDownButton = fieldRow(page, 'Duration').getByRole('button', { name: 'Move Duration down' })
	await moveDownButton.focus()
	await page.keyboard.press('Enter')
	await expect(deleteButtons.first()).toHaveAccessibleName('Delete Summary')
	await expect(moveDownButton).toBeFocused()
	await fieldRow(page, 'Duration').getByRole('button', { name: 'Move Duration up' }).click()
	await expect(deleteButtons.first()).toHaveAccessibleName('Delete Duration')
	await expect.poll(() => orderBodies).toEqual([{ order: ['summary', 'duration'] }])
	const groupsResponse = page.waitForResponse(
		(response) => response.request().method() === 'GET' && new URL(response.url()).pathname === '/api/groups',
	)
	release()

	await expect.poll(() => orderBodies).toEqual([{ order: ['summary', 'duration'] }, { order: ['duration', 'summary'] }])
	await groupsResponse
	await expect(page.locator('.godmin-toast')).toHaveCount(0)
	await page.reload()
	await expect(deleteButtons).toHaveCount(2)
	await expect(deleteButtons.first()).toHaveAccessibleName('Delete Duration')
})

test('opens a field in place, saves it from Enter and from the header, and reads its badge after a reload', async ({
	page,
}) => {
	const group = await createIsolatedGroup(page, 'Contact')
	await textFieldIn(page, group, 'Phone')
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const saveButton = main.getByRole('button', { name: 'Save', exact: true })

	const detailRow = await openField(page, 'Phone')
	await expect(fieldRow(page, 'Phone').getByRole('button', { name: 'Phone', exact: true })).toBeFocused()
	const labelInput = detailRow.getByRole('textbox', { name: 'Label' })
	await labelInput.fill('Phone number')
	await labelInput.press('Enter')
	await expect(page.locator('.godmin-toast', { hasText: `${group.title} updated.` })).toBeVisible()
	const renamedToggle = fieldRow(page, 'Phone number').getByRole('button', { name: 'Phone number', exact: true })
	await expect(renamedToggle).toHaveAttribute('aria-expanded', 'false')
	await expect(renamedToggle).toBeFocused()
	const renamedDetailRow = await openField(page, 'Phone number')
	await renamedDetailRow.getByRole('tab', { name: 'Validation' }).click()
	await renamedDetailRow.getByRole('checkbox', { name: 'Required' }).click()
	const fieldPath = `/api/groups/${group.id}/fields/phone`
	const fieldSaveResponse = page.waitForResponse(
		(response) => response.request().method() === 'PATCH' && new URL(response.url()).pathname === fieldPath,
	)
	await saveButton.click()
	expect((await fieldSaveResponse).ok()).toBe(true)
	await expect(saveButton).toHaveAttribute('aria-disabled', 'true')
	await page.reload()

	const row = fieldRow(page, 'Phone number')
	await expect(row.getByText('Required', { exact: true })).toBeVisible()
	await expect(row.getByRole('button', { name: 'Phone number', exact: true })).toHaveAttribute('aria-expanded', 'false')
})

test('keeps a field whose save was rejected open, the server words under the setting it named', async ({ page }) => {
	const group = await createIsolatedGroup(page, 'Library')
	await textFieldIn(page, group, 'Phone')
	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')

	const detailRow = await openField(page, 'Phone')
	await detailRow.getByRole('textbox', { name: 'Default' }).fill('Maria Perez')
	await detailRow.getByRole('tab', { name: 'Validation' }).click()
	await detailRow.getByRole('spinbutton', { name: 'Longest' }).fill('5')
	await main.getByRole('button', { name: 'Save', exact: true }).click()

	const rejectionMessage =
		'The setting default falls outside the others. Keep min below max and the default inside the bounds.'
	await expect(page.locator('.godmin-toast', { hasText: `Could not save Phone. ${rejectionMessage}` })).toBeVisible()
	await expect(detailRow.getByRole('tab', { name: 'General' })).toHaveAttribute('aria-selected', 'true')
	const defaultInput = detailRow.getByRole('textbox', { name: 'Default' })
	await expect(defaultInput).toHaveAccessibleDescription(rejectionMessage)
	await expect(fieldRow(page, 'Phone').getByRole('button', { name: 'Phone', exact: true })).toHaveAttribute(
		'aria-expanded',
		'true',
	)
	await defaultInput.fill('Maria')
	await expect(defaultInput).toHaveAccessibleDescription('')
})

test('puts the focus on the trash of the field taking the place of a deleted one', async ({ page }) => {
	const group = await createIsolatedGroup(page, 'Gallery')
	await textFieldIn(page, group, 'Duration')
	await textFieldIn(page, group, 'Summary')
	await page.goto(`/admin/field-groups/${group.id}`)

	await fieldRow(page, 'Duration').getByRole('button', { name: 'Delete Duration' }).click()
	const deleteDialog = page.getByRole('alertdialog', { name: 'Delete "Duration"?' })
	await deleteDialog.getByRole('button', { name: 'Delete the field', exact: true }).click()

	await expect(fieldRow(page, 'Duration')).toHaveCount(0)
	await expect(fieldRow(page, 'Summary').getByRole('button', { name: 'Delete Summary' })).toBeFocused()
})

test('adds Name and Role inside an open repeater from its own form, each row closed and the form ready for the next', async ({
	page,
}) => {
	const group = await createIsolatedGroup(page, 'Squad')
	await containerIn(page, group, 'Team', 'repeater')
	await page.goto(`/admin/field-groups/${group.id}`)

	const team = await openField(page, 'Team')
	await expect(team.getByText('No sub fields yet.', { exact: true })).toBeVisible()
	const form = team.getByRole('form', { name: 'Add field to Team', exact: true })
	const addButton = form.getByRole('button', { name: 'Add field to Team', exact: true })
	const labelInput = form.getByRole('textbox', { name: 'Label', exact: true })
	await labelInput.fill('Name')
	await labelInput.press('Enter')
	await expect(page.locator('.godmin-toast', { hasText: 'Name declared.' })).toBeVisible()
	const subFields = team.getByRole('region', { name: 'Sub fields of Team' })
	await expect(subFields.getByRole('button', { name: 'Name', exact: true })).toHaveAttribute('aria-expanded', 'false')
	await expect(labelInput).toHaveValue('')
	await expect(labelInput).toBeFocused()

	await labelInput.fill('Role')
	await addButton.click()
	await expect(page.locator('.godmin-toast', { hasText: 'Role declared.' })).toBeVisible()
	await expect(subFields.getByRole('button', { name: 'Role', exact: true })).toHaveAttribute('aria-expanded', 'false')
	await expect(labelInput).toHaveValue('')
	await expect(addButton).toBeFocused()
	await expect(page.getByRole('dialog')).toHaveCount(0)
	await page.reload()

	const reloadedSubFields = (await openField(page, 'Team')).getByRole('region', { name: 'Sub fields of Team' })
	const deleteButtons = reloadedSubFields.getByRole('button', { name: /^Delete / })
	await expect(deleteButtons).toHaveCount(2)
	await expect(deleteButtons.first()).toHaveAccessibleName('Delete Name')
	await expect(deleteButtons.last()).toHaveAccessibleName('Delete Role')
})

test('adds a Hero layout from the form of an open flexible field, then a Title from the form of Hero', async ({
	page,
}) => {
	const group = await createIsolatedGroup(page, 'Landing')
	await containerIn(page, group, 'Features', 'flexible')
	await page.goto(`/admin/field-groups/${group.id}`)

	const features = await openField(page, 'Features')
	const layoutsHeading = features.getByRole('heading', { name: 'Layouts of Features', exact: true })
	await expect(layoutsHeading.getByText('Layouts', { exact: true })).toBeVisible()
	await expect(features.getByText('No layouts yet.', { exact: true })).toBeVisible()
	const layoutForm = features.getByRole('form', { name: 'Add layout to Features', exact: true })
	const addLayoutButton = layoutForm.getByRole('button', { name: 'Add layout to Features', exact: true })
	await expect(layoutForm.getByRole('combobox')).toHaveCount(0)
	await layoutForm.getByRole('textbox', { name: 'Label', exact: true }).fill('Hero')
	await addLayoutButton.click()
	await expect(page.locator('.godmin-toast', { hasText: 'Hero declared.' })).toBeVisible()
	const layouts = features.getByRole('region', { name: 'Layouts of Features' })
	const toggle = layouts.getByRole('button', { name: 'Hero', exact: true })
	await expect(toggle).toHaveAttribute('aria-expanded', 'false')

	await toggle.click()
	await expect(toggle).toHaveAttribute('aria-expanded', 'true')
	const hero = layouts
		.getByRole('row')
		.filter({ has: page.getByRole('button', { name: 'Hero', exact: true }) })
		.locator('xpath=following-sibling::tr[1]')
	await hero.getByRole('tab', { name: 'Validation', exact: true }).click()
	await expect(hero.getByRole('spinbutton', { name: 'Fewest rows', exact: true })).toBeVisible()
	await expect(hero.getByRole('spinbutton', { name: 'Most rows', exact: true })).toBeVisible()
	await expect(hero.getByText('No sub fields yet.', { exact: true })).toBeVisible()
	const labelInput = hero
		.getByRole('form', { name: 'Add field to Hero', exact: true })
		.getByRole('textbox', { name: 'Label', exact: true })
	await labelInput.fill('Title')
	await labelInput.press('Enter')
	await expect(page.locator('.godmin-toast', { hasText: 'Title declared.' })).toBeVisible()
	const heroSubFields = hero.getByRole('region', { name: 'Sub fields of Hero' })
	await expect(heroSubFields.getByRole('button', { name: 'Title', exact: true })).toHaveAttribute(
		'aria-expanded',
		'false',
	)
	await page.reload()

	const reloadedFeatures = await openField(page, 'Features')
	const reloadedLayouts = reloadedFeatures.getByRole('region', { name: 'Layouts of Features' })
	await reloadedLayouts.getByRole('button', { name: 'Hero', exact: true }).click()
	const reloadedHeroSubFields = reloadedFeatures.getByRole('region', { name: 'Sub fields of Hero' })
	await expect(reloadedHeroSubFields.getByRole('button', { name: 'Title', exact: true })).toBeVisible()
})

test('keeps an open field inside its region on a phone, its open part padded 16px', async ({ page }) => {
	const group = await createIsolatedGroup(page, 'Clinic')
	await textFieldIn(page, group, 'Phone')
	await page.setViewportSize(PHONE)
	await page.goto(`/admin/field-groups/${group.id}`)

	const detailRow = await openField(page, 'Phone')

	const detailBody = detailRow.locator('.godmin-table__detail-body')
	await expect(detailBody.getByRole('textbox', { name: 'Label' })).toHaveValue('Phone')
	await expect
		.poll(
			() =>
				detailBody.evaluate((node) => {
					const regionRect = (node.closest('[role="region"]') as HTMLElement).getBoundingClientRect()
					const bodyRect = node.getBoundingClientRect()
					const style = getComputedStyle(node)
					return {
						isInside:
							Math.round(bodyRect.left) >= Math.round(regionRect.left) &&
							Math.round(bodyRect.right) <= Math.round(regionRect.right),
						padding: [style.paddingTop, style.paddingRight, style.paddingBottom, style.paddingLeft],
						pageOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
					}
				}),
			{ message: 'the open part fits its region' },
		)
		.toEqual({ isInside: true, padding: ['16px', '16px', '16px', '16px'], pageOverflow: 0 })
})

test('keeps each button and badge beside a label on one line, in a container too, on a desktop and a phone', async ({
	page,
}) => {
	const group = await createIsolatedGroup(page, 'Studio')
	await sectionIn(page, group, 'Author', 'Byline')
	await page.goto(`/admin/field-groups/${group.id}`)
	await openField(page, 'Author')
	const authorSubFields = page.getByRole('region', { name: 'Sub fields of Author' })
	const byline = authorSubFields.getByRole('row').filter({ has: page.getByText('Byline', { exact: true }) })
	const toggle = byline.getByRole('button', { name: 'Byline', exact: true })
	await toggle.click()
	await expect(toggle).toHaveAttribute('aria-expanded', 'true')
	await byline.locator('xpath=following-sibling::tr[1]').getByRole('textbox', { name: 'Label' }).fill('Byline line')
	await expect(byline.getByText('Unsaved changes', { exact: true })).toBeVisible()

	for (const size of [DESKTOP, PHONE]) {
		await page.setViewportSize(size)
		const labelTexts = (await labelCellLines(page)).map((cellLine) => cellLine.text)
		expect(labelTexts).toEqual(expect.arrayContaining(['Move', 'Unsaved changes']))
		await expect
			.poll(async () => (await labelCellLines(page)).filter((cellLine) => cellLine.lineCount !== 1), {
				message: `every button and badge beside a label takes one line at ${size.width}`,
			})
			.toEqual([])
	}
})

test('wraps a location rule and a field condition on a phone, every control inside the details and the tab', async ({
	page,
}) => {
	const group = await createIsolatedGroup(page, 'Apprenticeship')
	await textFieldIn(page, group, 'Duration')
	await conditionalFieldIn(page, group, 'Summary', 'duration')
	await page.setViewportSize(PHONE)

	await page.goto(`/admin/field-groups/${group.id}`)
	const main = page.getByRole('main')
	const details = main.getByRole('region', { name: 'Field group settings' })
	const rule = ruleOf(details.getByRole('button', { name: 'Remove rule 1 of set 1' }))
	await expect(rule).toHaveAccessibleName('Rule 1 of set 1')
	await expect(rule.getByRole('combobox', { name: 'Value' })).toHaveText(`Apprenticeships ${RUN}`)
	expect.soft(await fitOf(rule), 'the rule row fits the details').toEqual({
		isWrapped: true,
		isInside: true,
		maxOverflow: 0,
	})

	const summary = await openField(page, 'Summary')
	const tab = summary.getByRole('tab', { name: /^Conditional logic/ })
	const activeMark = tab.getByText('Active', { exact: true })
	await expect
		.poll(async () => {
			const markRect = await activeMark.boundingBox()
			return markRect && { width: markRect.width, height: markRect.height }
		}, { message: 'the Active mark draws as a dot at 390' })
		.toEqual({ width: 8, height: 8 })
	await expect(tab).toHaveAccessibleName('Conditional logic Active')
	await tab.click()
	const conditionsPanel = summary.getByRole('tabpanel', { name: /^Conditional logic/ })
	await expect(conditionsPanel.getByRole('textbox', { name: 'Value' })).toHaveValue('Two hours')
	const condition = ruleOf(conditionsPanel.getByRole('button', { name: 'Remove rule 1 of set 1' }))
	await expect(condition).toHaveAccessibleName('Rule 1 of set 1')
	await expect
		.poll(() => fitOf(condition), { message: 'the condition row fits the tab at 390' })
		.toEqual({ isWrapped: true, isInside: true, maxOverflow: 0 })
	await expect
		.poll(() => page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), {
			message: 'the open field with its four tabs leaves the page unscrolled at 390',
		})
		.toBe(0)

	await page.setViewportSize(DESKTOP)
	await expect
		.poll(async () => {
			const { isInside, maxOverflow } = await fitOf(condition)
			return { isInside, maxOverflow }
		}, { message: 'the condition row fits the tab at 1280' })
		.toEqual({ isInside: true, maxOverflow: 0 })
})
