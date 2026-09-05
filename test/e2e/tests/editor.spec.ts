// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)
const TITLE = `A post the golden path wrote ${RUN}`
const TRASH_TITLE = `A post the golden path trashed ${RUN}`
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
 * Starts writing in the canvas, which opens with no block to type into.
 * @param page - The page to drive.
 */
async function startWriting(page: Page) {
	await canvas(page).getByRole('button', { name: 'Add default block' }).click()
}

const created: string[] = []

/**
 * Opens a fresh draft in the editor and remembers it for cleanup.
 * @param page - The page to drive.
 */
async function openNewDraft(page: Page) {
	await page.goto('/admin/content/post')
	await page.getByRole('button', { name: 'Add New' }).click()
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

	await page.getByRole('button', { name: 'Publish' }).click()
	await expect(shown(page, 'Post published.')).toBeVisible()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()

	await page.getByRole('link', { name: 'Back to posts' }).click()
	await page.getByRole('button', { name: /^Published/ }).click()
	await expect(page.getByRole('link', { name: TITLE })).toBeVisible()
})

test('publishes what was written without a draft save first', async ({ page }) => {
	await openNewDraft(page)
	await writeThePost(page)

	await page.getByRole('button', { name: 'Publish' }).click()
	await expect(shown(page, 'Post published.')).toBeVisible()

	await page.reload()
	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(TITLE)
	await expect(canvas(page).getByText(PARAGRAPH)).toBeVisible()
})

test('keeps an edit made to an already published post', async ({ page }) => {
	const added = `The words the update added ${RUN}.`
	await openNewDraft(page)
	await writeThePost(page)
	await page.getByRole('button', { name: 'Publish' }).click()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()

	await canvas(page).getByText(PARAGRAPH).click()
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
	await page.getByRole('button', { name: 'Publish' }).click()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()

	await page.getByRole('link', { name: 'Back to posts' }).click()
	await page.getByRole('button', { name: /^Published/ }).click()
	await page.getByRole('link', { name: written }).click()
	await expect(page.getByRole('combobox', { name: 'Status' })).toHaveText('Published')

	await page.getByRole('textbox', { name: 'Title' }).fill(edited.title)
	await canvas(page).getByText(PARAGRAPH).click()
	await page.keyboard.press('End')
	await page.keyboard.press('Enter')
	await page.keyboard.type(edited.paragraph)
	await page.getByRole('textbox', { name: 'Slug' }).fill(edited.slug)
	await page.getByRole('textbox', { name: 'Excerpt' }).fill(edited.excerpt)
	await page.getByRole('combobox', { name: 'Status' }).click()
	await page.getByRole('option', { name: 'Pending' }).click()
	await expect(page.getByRole('button', { name: 'Update' })).toBeVisible()
	expect(await page.getByRole('button', { name: 'Publish' }).count()).toBe(0)
	await page.getByRole('button', { name: 'Update' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.getByRole('link', { name: 'Back to posts' }).click()
	await page.getByRole('button', { name: /^Pending/ }).click()
	await page.getByRole('link', { name: edited.title }).click()

	await expect(page.getByRole('textbox', { name: 'Title' })).toHaveValue(edited.title)
	await expect(page.getByRole('textbox', { name: 'Slug' })).toHaveValue(edited.slug)
	await expect(page.getByRole('textbox', { name: 'Excerpt' })).toHaveValue(edited.excerpt)
	await expect(page.getByRole('combobox', { name: 'Status' })).toHaveText('Pending')
	await expect(page.getByRole('button', { name: 'Publish' })).toBeVisible()
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

test('takes a post back out of the trash from the editor', async ({ page }) => {
	await openNewDraft(page)
	await page.getByRole('textbox', { name: 'Title' }).fill(TRASH_TITLE)
	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(shown(page, 'Draft saved.')).toBeVisible()

	await page.getByRole('button', { name: 'Move to trash' }).click()
	await page.getByRole('alertdialog').getByRole('button', { name: 'Move to trash' }).click()

	await expect(page.getByRole('button', { name: 'Add New' })).toBeVisible()
	await expect(shown(page, 'Moved to the trash.')).toBeVisible()

	await page.getByRole('button', { name: 'Undo' }).click()

	await page.getByRole('button', { name: /^Draft/ }).click()
	await expect(page.getByRole('link', { name: TRASH_TITLE })).toBeVisible()
})

test('walks an edit back with undo', async ({ page }) => {
	await openNewDraft(page)
	await startWriting(page)
	await page.keyboard.type('First words.')
	await expect(canvas(page).getByText('First words.')).toBeVisible()

	await page.getByRole('button', { name: 'Undo' }).click()

	await expect(canvas(page).getByText('First words.')).toBeHidden()
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
