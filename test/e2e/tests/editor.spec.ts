// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)
const TITLE = `A post the golden path wrote ${RUN}`
const TRASH_TITLE = `A post the golden path trashed ${RUN}`
const READ_TITLE = `A post the golden path read in the trash ${RUN}`
const LAYOUT_TITLE = `A post the golden path asked to trash ${RUN}`
const REFUSED_TITLE = `A post the golden path could not trash ${RUN}`
const FLEXIBLE_TITLE = `A post the golden path built in rows ${RUN}`
const PARAGRAPH = 'The paragraph the golden path typed.'
const HEADING = 'The heading the golden path typed'

/**
 * Returns the canvas the block editor writes into.
 * @param page - The page holding the editor.
 * @returns The canvas frame.
 */
function canvas(page: Page) {
	return page.frameLocator('iframe[name="editor-canvas"]')
}

/**
 * Returns a message shown on the page, ignoring the announcement of it.
 * @param page - The page holding the message.
 * @param message - The words to look for.
 * @returns The message locator.
 */
function shown(page: Page, message: string) {
	return page.locator('#root').getByText(message)
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

/**
 * Starts writing in the canvas, which opens on the empty paragraph it shows before any block is stored.
 * @param page - The page to drive.
 */
async function startWriting(page: Page) {
	await canvas(page).getByRole('document', { name: 'Add default block' }).click()
}

const created: string[] = []

/**
 * Opens a fresh draft in the editor and remembers it for cleanup.
 * @param page - The page to drive.
 */
async function openNewDraft(page: Page) {
	await page.goto('/admin/content/post')
	await page.getByRole('main').getByRole('button', { name: 'Add Post', exact: true }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	const id = page.url().match(/content\/[a-z-]+\/([0-9a-f-]+)\/edit/)?.[1]
	if (id !== undefined) {
		created.push(id)
	}
}

test.afterEach(async ({ page }) => {
	for (const id of created.splice(0)) {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
})

/**
 * Clicks a block just inside its end edge.
 * @param block - The block to click.
 */
async function clickAtEnd(block: Locator) {
	const found = await block.boundingBox()
	if (found === null) {
		throw new Error('the block takes no box on screen')
	}
	await block.click({ position: { x: found.width - 4, y: found.height / 2 } })
}

/**
 * Writes a title, a paragraph and a heading into the open editor.
 * @param page - The page to drive.
 */
async function writeThePost(page: Page, title: string = TITLE) {
	await page.getByRole('textbox', { name: 'Title' }).fill(title)
	await startWriting(page)
	await page.keyboard.type(PARAGRAPH)
	await page.keyboard.press('Enter')
	await page.keyboard.type(`## ${HEADING}`)
	await expect(canvas(page).getByText(PARAGRAPH)).toBeVisible()
}

test('writes, saves, publishes, trashes and restores a post', async ({ page }) => {
	await openNewDraft(page)
	await writeThePost(page)

	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.reload()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(TITLE)
	await expect(canvas(page).getByText(PARAGRAPH)).toBeVisible()
	await expect(canvas(page).getByRole('document', { name: 'Block: Heading 2' })).toHaveText(
		HEADING,
	)

	await page.getByRole('button', { name: 'Publish', exact: true }).click()
	await expect(shown(page, 'Post published.')).toBeVisible()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()

	await page.getByRole('link', { name: 'Back to posts' }).click()
	await statusTab(page, 'Published').click()
	await expect(page.getByRole('link', { name: TITLE })).toBeVisible()
})

test('keeps the words of a draft the author left for the posts list', async ({ page }) => {
	const title = `A draft the golden path left ${RUN}`
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(title)

	await page.getByRole('link', { name: 'Back to posts' }).click()
	await expect(page.getByRole('main').getByRole('button', { name: 'Add Post', exact: true })).toBeVisible()
	await page.goBack()

	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(title)
	await page.getByRole('textbox', { name: 'Title' }).fill(`${title} again`)
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()
})

test('publishes what was written without a draft save first', async ({ page }) => {
	await openNewDraft(page)
	await writeThePost(page)

	await page.getByRole('button', { name: 'Publish', exact: true }).click()
	await expect(shown(page, 'Post published.')).toBeVisible()

	await page.reload()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(TITLE)
	await expect(canvas(page).getByText(PARAGRAPH)).toBeVisible()
})

test('keeps an edit made to an already published post', async ({ page }) => {
	const added = `The words the update added ${RUN}.`
	await openNewDraft(page)
	await writeThePost(page)
	await page.getByRole('button', { name: 'Publish', exact: true }).click()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()

	await clickAtEnd(canvas(page).getByText(PARAGRAPH))
	await page.keyboard.press('End')
	await page.keyboard.press('Enter')
	await page.keyboard.type(added)
	await expect(canvas(page).getByText(added)).toBeVisible()
	await page.getByRole('button', { name: 'Update' }).click()
	await expect(shown(page, 'Post published.')).toBeVisible()

	await page.reload()
	await expect(canvas(page).getByText(added)).toBeVisible()
})

test('round-trips every field of the editor without a full reload', async ({ page }) => {
	const written = `A post the field sweep wrote ${RUN}`
	const edited = {
		title: `A post edited everywhere ${RUN}`,
		slug: `edited-everywhere-${RUN}`,
		excerpt: 'The excerpt the field sweep wrote.',
		paragraph: `The paragraph the field sweep added ${RUN}.`,
	}
	await openNewDraft(page)
	await writeThePost(page, written)
	await page.getByRole('button', { name: 'Publish', exact: true }).click()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()

	await page.getByRole('link', { name: 'Back to posts' }).click()
	await statusTab(page, 'Published').click()
	await page.getByRole('link', { name: written }).click()
	await expect(page.getByRole('combobox', { name: 'Status' })).toHaveText('Published')

	await page.getByRole('textbox', { name: 'Title' }).fill(edited.title)
	await clickAtEnd(canvas(page).getByText(PARAGRAPH))
	await page.keyboard.press('End')
	await page.keyboard.press('Enter')
	await page.keyboard.type(edited.paragraph)
	await page.getByRole('textbox', { name: 'Slug' }).fill(edited.slug)
	await page.getByRole('textbox', { name: 'Excerpt' }).fill(edited.excerpt)
	await page.getByRole('combobox', { name: 'Status' }).click()
	await page.getByRole('option', { name: 'Pending' }).click()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()
	expect(await page.getByRole('button', { name: 'Publish', exact: true }).count()).toBe(0)
	await page.getByRole('button', { name: 'Update' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.getByRole('link', { name: 'Back to posts' }).click()
	await statusTab(page, 'Pending').click()
	await page.getByRole('link', { name: edited.title }).click()

	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(edited.title)
	await expect(page.getByRole('textbox', { name: 'Slug' })).toHaveValue(edited.slug)
	await expect(page.getByRole('textbox', { name: 'Excerpt' })).toHaveValue(edited.excerpt)
	await expect(page.getByRole('combobox', { name: 'Status' })).toHaveText('Pending')
	await expect(page.getByRole('button', { name: 'Publish', exact: true })).toBeVisible()
	await expect(canvas(page).getByText(edited.paragraph)).toBeVisible()

	await page.reload()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(edited.title)
	await expect(page.getByRole('textbox', { name: 'Slug' })).toHaveValue(edited.slug)
	await expect(page.getByRole('textbox', { name: 'Excerpt' })).toHaveValue(edited.excerpt)
	await expect(page.getByRole('combobox', { name: 'Status' })).toHaveText('Pending')
	await expect(canvas(page).getByText(edited.paragraph)).toBeVisible()
})

test('saves twice over without reloading in between', async ({ page }) => {
	const statuses: number[] = []
	page.on('response', (response) => {
		if (response.request().method() === 'PATCH') {
			statuses.push(response.status())
		}
	})
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(`${TITLE} once`)
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.getByRole('textbox', { name: 'Title' }).fill(`${TITLE} twice`)
	await page.getByRole('button', { name: 'Save draft' }).click()

	await expect.poll(() => statuses).toEqual([200, 200])
	await page.reload()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(`${TITLE} twice`)
})

test('takes a post trashed from the editor back out of the trash from the list', async ({ page }) => {
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(TRASH_TITLE)
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.getByRole('button', { name: 'Move to trash' }).click()

	await expect(page.getByRole('main').getByRole('button', { name: 'Add Post', exact: true })).toBeVisible()
	await expect(shown(page, `"${TRASH_TITLE}" moved to the trash.`)).toBeVisible()

	await statusTab(page, 'Trash').click()
	await page.getByRole('row').filter({ hasText: TRASH_TITLE }).getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Restore' }).click()
	await expect(shown(page, `"${TRASH_TITLE}" has been restored.`)).toBeVisible()

	await statusTab(page, 'Draft').click()
	await expect(page.getByRole('link', { name: TRASH_TITLE })).toBeVisible()
})

test('offers the words left in a published post again once its trash from the list is undone', async ({ page }) => {
	const title = `A post the golden path trashed from the list ${RUN}`
	const edited = `${title} with words left unsaved`
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(title)
	await page.getByRole('button', { name: 'Publish', exact: true }).click()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()
	await page.getByRole('textbox', { name: 'Title' }).fill(edited)
	const [parked] = await Promise.all([
		page.waitForResponse((answer) => answer.request().method() === 'POST' && answer.url().endsWith('/autosave')),
		page.getByRole('link', { name: 'Back to posts' }).click(),
	])
	expect(parked.ok()).toBe(true)
	const row = page.getByRole('row').filter({ has: page.getByRole('link', { name: title, exact: true }) })
	await row.getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Trash', exact: true }).click()
	const toast = page.locator('.godmin-toast', { hasText: `"${title}" moved to the trash.` })
	await toast.getByRole('button', { name: 'Undo', exact: true }).click()
	await expect(shown(page, `"${title}" has been restored.`)).toBeVisible()

	await page.getByRole('link', { name: title, exact: true }).click()

	await expect(shown(page, 'The browser kept an unsaved version of this post.')).toBeVisible()
	await page.getByRole('button', { name: 'Restore', exact: true }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(edited)
})

test('lays out Move to trash as the WordPress editor does', async ({ page }) => {
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(LAYOUT_TITLE)
	const trigger = page.getByRole('button', { name: 'Move to trash' })

	const placed = await trigger.evaluate((element) => {
		const box = element.getBoundingClientRect()
		const above = (element.previousElementSibling as HTMLElement).getBoundingClientRect()
		const below = element.nextElementSibling?.getBoundingClientRect()
		const panel = element.closest('.gophenberg-editor__sidebar-panel') as HTMLElement
		const inset = getComputedStyle(panel)
		const room = panel.getBoundingClientRect()
		return {
			gap: Math.round(box.top - above.bottom),
			under: Math.round((below?.top ?? Number.NaN) - box.bottom),
			height: Math.round(box.height),
			left: Math.round(box.left - room.left - Number.parseFloat(inset.paddingLeft)),
			right: Math.round(room.right - Number.parseFloat(inset.paddingRight) - box.right),
		}
	})
	expect(placed).toEqual({ gap: 16, under: 16, height: 40, left: 0, right: 0 })
	await expect(trigger).toHaveCSS('color', 'rgb(204, 24, 24)')
	await expect(trigger).toHaveCSS('justify-content', 'center')
	const field = await page.getByLabel('On sale').boundingBox()
	const button = await trigger.boundingBox()
	expect(field?.y ?? 0).toBeGreaterThan((button?.y ?? 0) + (button?.height ?? 0))
})

test('shows Move to trash busy at once and reports a trash the server refused in a toast', async ({ page }) => {
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(REFUSED_TITLE)
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()
	let answer: () => void = () => undefined
	const held = new Promise<void>((release) => {
		answer = release
	})
	await page.route('**/api/content/*', async (route) => {
		if (route.request().method() !== 'DELETE') {
			await route.fallback()
			return
		}
		await held
		await route.fulfill({ status: 500, json: {} })
	})
	const trigger = page.getByRole('button', { name: 'Move to trash' })

	await trigger.click()

	await expect(trigger).toHaveAttribute('aria-disabled', 'true')
	await expect(trigger).toHaveClass(/\bis-busy\b/)
	answer()
	await expect(shown(page, 'The item could not be moved to the trash.')).toBeVisible()
	await expect(trigger).not.toHaveClass(/\bis-busy\b/)
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(REFUSED_TITLE)
})

test('reads a trashed post and restores it into the editor', async ({ page }) => {
	const statuses: number[] = []
	page.on('response', (response) => {
		if (response.request().method() === 'PATCH') {
			statuses.push(response.status())
		}
	})
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(READ_TITLE)
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()
	await page.getByRole('button', { name: 'Move to trash' }).click()
	await expect(shown(page, `"${READ_TITLE}" moved to the trash.`)).toBeVisible()

	await statusTab(page, 'Trash').click()
	await page.getByRole('link', { name: READ_TITLE }).click()

	await expect(shown(page, 'This item is in the trash. Restore it first to work on it again.')).toBeVisible()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveCount(0)
	await expect(page.getByRole('button', { name: 'Move to trash' })).toHaveCount(0)

	await page.getByRole('button', { name: 'Restore' }).click()

	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(READ_TITLE)
	await page.getByRole('textbox', { name: 'Title' }).fill(`${READ_TITLE} again`)
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect.poll(() => statuses).toEqual([200, 200])
})

test('walks an edit back with undo', async ({ page }) => {
	await openNewDraft(page)
	await startWriting(page)
	await page.keyboard.type('First words.')
	await expect(canvas(page).getByText('First words.')).toBeVisible()

	await page.getByRole('button', { name: 'Undo' }).click()

	await expect(canvas(page).getByText('First words.')).toBeHidden()
})

test('tells an author where to type in an empty post', async ({ page }) => {
	await openNewDraft(page)

	const hint = canvas(page).locator('[data-rich-text-placeholder]')

	await expect(hint).toBeAttached()
	expect(await hint.evaluate((node) => getComputedStyle(node, '::after').content)).toBe(
		'"Type / to choose a block"',
	)
})

test('moves focus into the block list when it opens', async ({ page }) => {
	await openNewDraft(page)
	await writeThePost(page)

	await page.getByRole('button', { name: 'List view' }).click()

	await expect(page.locator('.gophenberg-editor__outline :focus')).toHaveCount(1)
})

test('paints the editor in the design system accent', async ({ page }) => {
	await openNewDraft(page)

	const accent = await page.evaluate(() => {
		const root = getComputedStyle(document.documentElement)
		return {
			admin: root.getPropertyValue('--wp-admin-theme-color').trim(),
			brand: root.getPropertyValue('--wpds-color-background-interactive-brand-strong').trim(),
		}
	})

	expect(accent.admin).toBe(accent.brand)
})

test('shows a field only while the rule it stands on holds', async ({ page }) => {
	await openNewDraft(page)

	await expect(page.getByLabel('Sale note')).toBeHidden()

	await page.getByLabel('On sale').check()
	await page.getByLabel('Sale note').fill('Half price all week')
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.getByLabel('On sale').uncheck()
	await expect(page.getByLabel('Sale note')).toBeHidden()

	await page.getByLabel('On sale').check()
	await expect(page.getByLabel('Sale note')).toHaveValue('Half price all week')
})

test('keeps the rows a flexible content field holds under the layouts they take', async ({ page }) => {
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(FLEXIBLE_TITLE)

	await page.getByRole('button', { name: 'Add Hero' }).click()
	await page.getByLabel('Headline').fill('Everything starts with a block')
	await page.getByRole('button', { name: 'Add Quote' }).click()
	await page.getByLabel('Saying').fill('Worth keeping around')
	await page.getByLabel('Said by').fill('Maria Perez')
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.reload()

	await expect(page.getByLabel('Headline')).toHaveValue('Everything starts with a block')
	await expect(page.getByLabel('Saying')).toHaveValue('Worth keeping around')
	await expect(page.getByLabel('Said by')).toHaveValue('Maria Perez')
})

test('lists the posts filed under a category through the field reading them', async ({ page }) => {
	await page.goto('/admin/content/category')
	await page.getByRole('link', { name: 'News' }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue('News')

	const filed = page.getByRole('list', { name: 'Posts filed here' })

	await expect(filed.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeVisible()
})
