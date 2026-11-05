// SPDX-License-Identifier: Apache-2.0

import { writeFileSync } from 'node:fs'

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

import { credentials } from '../env'

const RUN = Math.random().toString(36).slice(2, 8)
const PICTURE = `harbor-e2e-${RUN}`
const POST_TITLE = `A post carrying a picture ${RUN}`
const LIBRARY = `library-e2e-${RUN}`

const PHONE = { width: 390, height: 844 }

const LAPTOP = { width: 1280, height: 900 }

const DESKTOP = { width: 1920, height: 1080 }

/** How far the media library picker sits from each edge of a window of each size. */
const PICKER_INSETS = [
	{ size: DESKTOP, inset: 40 },
	{ size: LAPTOP, inset: 40 },
	{ size: { width: 700, height: 900 }, inset: 16 },
	{ size: PHONE, inset: 0 },
]

/** The box an element takes on screen. */
type Box = { x: number, y: number, width: number, height: number }

// PHOTO_BASE64 is an eight by eight JPEG painted for this suite.
const PHOTO_BASE64 =
	'/9j/2wCEAAYEBQYFBAYGBQYHBwYIChAKCgkJChQODwwQFxQYGBcUFhYaHSUfGhsjHBYWICwgIyYnKSopGR8tMC0oMCUo' +
	'KSgBBwcHCggKEwoKEygaFhooKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKP/A' +
	'ABEIAAgACAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQEC' +
	'AwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldY' +
	'WVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY' +
	'2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncA' +
	'AQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVW' +
	'V1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV' +
	'1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/AMTwt8P/ALn7n9K6r/hX/wD0x/Suo8LfwV1VedjM3xPt' +
	'HqXw5nuL+pR94//Z'

/**
 * Returns the painted photo as an upload payload.
 * @param name - The file name the upload carries.
 * @returns The file Playwright hands a chooser.
 */
function photo(name: string) {
	return { name, mimeType: 'image/jpeg', buffer: Buffer.from(PHOTO_BASE64, 'base64') }
}

/** A text file, which no image block takes. */
const NOTES = { name: `notes-e2e-${RUN}.txt`, mimeType: 'text/plain', buffer: Buffer.from('Notes on the harbor.') }

/** An Image block that holds no picture yet, as the editor stores it. */
const EMPTY_IMAGE_BLOCK = '<!-- wp:image -->\n<figure class="wp-block-image"><img alt=""/></figure>\n<!-- /wp:image -->'

/**
 * Returns the canvas the block editor writes into.
 * @param page - The page holding the editor.
 * @returns The canvas frame.
 */
function canvas(page: Page) {
	return page.frameLocator('iframe[name="editor-canvas"]')
}

const createdPosts: string[] = []

test.afterEach(async ({ page }) => {
	for (const id of createdPosts.splice(0)) {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
	const listed = await page.request.get(`/api/media?search=e2e-${RUN}`)
	const body = (await listed.json()) as { items: { id: number }[] }
	for (const item of body.items) {
		await page.request.delete(`/api/media/${item.id}`)
	}
})

/**
 * Stores the painted photo through the API the admin uses, its title taken from its file name.
 * @param page - The page whose session uploads the photo.
 * @param title - The title the photo takes, its file name less the extension.
 * @returns The id of the stored photo.
 */
async function stored(page: Page, title: string): Promise<number> {
	const answer = await page.request.post('/api/media', { multipart: { file: photo(`${title}.jpg`) } })
	expect(answer.status()).toBe(201)
	return ((await answer.json()) as { id: number }).id
}

/**
 * Opens a new post in the editor, keeping its id for the clean up.
 * @param page - The page to drive.
 */
async function newPost(page: Page) {
	await page.goto('/admin/content/post')
	await page.getByRole('main').getByRole('button', { name: 'Add Post', exact: true }).click()
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	const postId = page.url().match(/content\/[a-z-]+\/([0-9a-f-]+)\/edit/)?.[1]
	if (postId !== undefined) {
		createdPosts.push(postId)
	}
}

/**
 * Adds an image block to the open post.
 * @param page - The page holding the editor.
 */
async function addImageBlock(page: Page) {
	await canvas(page).getByRole('document', { name: 'Add default block' }).click()
	await page.keyboard.type('/image')
	await page.keyboard.press('Enter')
}

/**
 * Opens a new post, adds an image block and opens the media library picker from it.
 * @param page - The page to drive.
 * @param size - The window the block opens the picker in, the test's own when absent.
 * @returns The picker dialog.
 */
async function openPicker(page: Page, size?: { width: number, height: number }): Promise<Locator> {
	await newPost(page)
	await addImageBlock(page)
	if (size !== undefined) {
		await page.setViewportSize(size)
	}
	await canvas(page).getByRole('button', { name: 'Media Library' }).click()
	return page.getByRole('dialog', { name: 'Select Media' })
}

/**
 * Picks a file through the Upload media button of the picker.
 * @param library - The picker dialog.
 * @param file - The file to pick, or the files of one pick.
 */
async function chooseFile(library: Locator, file: ReturnType<typeof photo> | ReturnType<typeof photo>[]) {
	const chooser = library.page().waitForEvent('filechooser')
	await library.getByRole('button', { name: 'Upload media' }).click()
	await (await chooser).setFiles(file)
}

/**
 * Returns a drag carrying a file, as one from the desktop carries it.
 * @param page - The page the drag crosses.
 * @param file - The file it carries.
 * @returns The handle a drag event takes as its data.
 */
function dragCarrying(page: Page, file: ReturnType<typeof photo>) {
	return page.evaluateHandle(
		([base64, name, type]) => {
			const bytes = Uint8Array.from(atob(base64), (char) => char.charCodeAt(0))
			const carried = new DataTransfer()
			carried.items.add(new File([bytes], name, { type }))
			return carried
		},
		[file.buffer.toString('base64'), file.name, file.mimeType],
	)
}

/**
 * Returns how far each side of a box sits inside another, rounded to the pixel.
 * @param inner - The box inside.
 * @param outer - The box around it.
 * @returns The gap at each side, below zero where the inner box passes the outer one.
 */
function gaps(inner: Box, outer: Box) {
	return {
		left: Math.round(inner.x - outer.x),
		right: Math.round(outer.x + outer.width - inner.x - inner.width),
		top: Math.round(inner.y - outer.y),
		bottom: Math.round(outer.y + outer.height - inner.y - inner.height),
	}
}

/**
 * Returns the box an element takes on screen.
 * @param element - The element to measure.
 * @returns Its box.
 */
async function boxOf(element: Locator) {
	return (await element.boundingBox()) as Box
}

/**
 * Returns how far each side of an element sits inside another on screen.
 * @param element - The element inside.
 * @param around - The element around it.
 * @returns The gap at each side.
 */
async function gapsWithin(element: Locator, around: Locator) {
	return gaps(await boxOf(element), await boxOf(around))
}

/**
 * Returns the card of the library grid showing a title, matched whole.
 * @param page - The page holding the library.
 * @param title - The title the card shows.
 * @returns The card locator.
 */
function cardOf(page: Page, title: string): Locator {
	return page.locator('.dataviews-view-grid__card').filter({ has: page.getByText(title, { exact: true }) })
}

/**
 * Opens the library narrowed to this run's files and the actions menu of the card showing a title.
 * @param page - The page to drive.
 * @param title - The title the card shows.
 */
async function openCardMenu(page: Page, title: string) {
	await page.goto(`/admin/media?search=${LIBRARY}`)
	await cardOf(page, title).getByRole('button', { name: 'Actions' }).click()
}

/**
 * Holds every listing of the library for 800ms from now on, as a real server over a network answers.
 * @param page - The page whose listings are held.
 */
async function slowListings(page: Page) {
	await page.route('**/api/media?*', async (route) => {
		await new Promise((resolve) => setTimeout(resolve, 800))
		await route.continue()
	})
}

/** How fast the server lists the library again after a save, at once from localhost or held as over a network. */
const LISTINGS = [
	{ speed: 'slow', hold: slowListings },
	{ speed: 'quick', hold: async () => {} },
]

/**
 * Returns the box of the open modal frame once its opening animation ends, rounded to the pixel.
 * @param page - The page holding the modal.
 * @param side - The edge or size of the box to read.
 * @returns The poll that reads it until it holds still.
 */
function frameBox(page: Page, side: 'width' | 'bottom') {
	return expect.poll(async () => {
		const box = await page.locator('.components-modal__frame').boundingBox()
		return Math.round(side === 'width' ? (box?.width ?? 0) : (box?.y ?? 0) + (box?.height ?? 0))
	})
}

/**
 * Returns the id of the account the page is signed in as.
 * @param page - The signed in page.
 * @returns The account id.
 */
async function sessionId(page: Page): Promise<string> {
	const answer = await page.request.get('/api/auth/session')
	return ((await answer.json()) as { id: string }).id
}

/**
 * Waits for the request listing the library that carries every given parameter.
 * @param page - The page that sends it.
 * @param wanted - The parameters and the values the request must carry.
 * @returns The wait.
 */
function listingAsking(page: Page, wanted: Record<string, string>) {
	return page.waitForRequest((request) => {
		const url = new URL(request.url())
		return url.pathname === '/api/media' && Object.entries(wanted).every(([key, value]) => url.searchParams.get(key) === value)
	})
}

/**
 * Stores a draft holding two empty Image blocks, keeping its id for the clean up, and opens it in the editor.
 * @param page - The page to drive.
 * @returns The two blocks in the canvas.
 */
async function openTwoImageBlocks(page: Page): Promise<Locator> {
	const answer = await page.request.post('/api/content', {
		data: { type: 'post', title: POST_TITLE, content: `${EMPTY_IMAGE_BLOCK}\n\n${EMPTY_IMAGE_BLOCK}` },
	})
	expect(answer.status()).toBe(201)
	const { id } = (await answer.json()) as { id: string }
	createdPosts.push(id)
	await page.goto(`/admin/content/post/${id}/edit`)
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
	return canvas(page).locator('[data-type="core/image"]')
}

/**
 * Selects an empty Image block by its corner and opens the picker from it.
 * @param block - The block.
 */
async function openPickerFrom(block: Locator) {
	await block.click({ position: { x: 4, y: 4 } })
	await block.getByRole('button', { name: 'Media Library' }).click()
}

/**
 * Opens View options in the open picker.
 * @param library - The picker dialog.
 * @returns The Sort by control.
 */
async function sortControl(library: Locator): Promise<Locator> {
	await library.getByRole('button', { name: 'View options' }).click()
	return library.getByRole('combobox', { name: 'Sort by' })
}

test('uploads two pictures chosen at once, counts them in a toast and serves them publicly', async ({ page }) => {
	await page.goto('/admin/media')
	await expect(page.getByRole('heading', { name: 'Media' })).toBeVisible()

	const chooser = page.waitForEvent('filechooser')
	await page.getByRole('main').getByRole('button', { name: 'Add Media File' }).click()
	await (await chooser).setFiles([photo(`${PICTURE}.jpg`), photo(`${PICTURE}-second.jpg`)])

	await expect(page.locator('#root').getByText('Uploaded 2 files.')).toBeVisible()
	const listed = await page.request.get(`/api/media?search=${PICTURE}`)
	const body = (await listed.json()) as { items: { file: string }[] }
	expect(body.items).toHaveLength(2)

	const served = await page.request.get(`/media/${body.items[0].file}`)
	expect(served.status()).toBe(200)
	expect(served.headers()['content-type']).toContain('image/jpeg')
})

test('uploads a picture dropped on the library, over a zone reading Drop files to upload', async ({ page }) => {
	const name = `${PICTURE}-dropped.jpg`
	await page.goto('/admin/media')
	await expect(page.getByRole('heading', { name: 'Media' })).toBeVisible()
	const transfer = await dragCarrying(page, photo(name))

	await page.dispatchEvent('main', 'dragenter', { dataTransfer: transfer })
	const zone = page.locator('.godmin-list-overlay')
	await zone.dispatchEvent('dragenter', { dataTransfer: transfer })
	await expect(zone).toHaveClass(/is-dragging-over-element/)
	await expect(zone.locator('.components-drop-zone__content')).toHaveCSS('opacity', '1')
	await expect(zone.locator('.components-drop-zone__content-inner')).toHaveCSS('opacity', '1')
	await expect(zone.getByText('Drop files to upload')).toBeVisible()
	await zone.dispatchEvent('drop', { dataTransfer: transfer })

	await expect(page.locator('#root').getByText(`"${name}" uploaded.`)).toBeVisible()
})

test('places a picture already in the library through the media library picker', async ({
	page,
}) => {
	await stored(page, `${PICTURE}-stored`)

	const library = await openPicker(page)

	await expect(library.getByRole('option', { name: `${PICTURE}-stored` })).toBeVisible()
	await library.getByRole('option', { name: `${PICTURE}-stored` }).click()
	await library.getByRole('button', { name: 'Select' }).click()

	await expect(canvas(page).locator('img[src^="/media/"]')).toBeVisible()
})

test('frames the media library search box when an image block opens the library', async ({ page }) => {
	const library = await openPicker(page)

	await library.getByRole('searchbox', { name: 'Search media…' }).focus()
	const frame = library.locator('.components-input-control__backdrop')

	await expect(frame).toHaveCSS('position', 'absolute')
	expect((await frame.boundingBox())?.height).toBeGreaterThan(0)
})

test('places an uploaded picture in a post and publishes it', async ({ page }) => {
	await newPost(page)
	const postId = createdPosts.at(-1)
	await page.getByRole('textbox', { name: 'Title' }).fill(POST_TITLE)

	await addImageBlock(page)
	const chooserOpens = page.waitForEvent('filechooser')
	await canvas(page).getByRole('button', { name: 'Upload' }).click()
	const chooser = await chooserOpens
	await chooser.setFiles(photo(`${PICTURE}-placed.jpg`))
	await expect(canvas(page).locator('img[src^="/media/"]')).toBeVisible()

	await page.getByRole('button', { name: 'Save draft' }).click()
	await expect(page.locator('#root').getByText('Draft saved.')).toBeVisible()
	await page.reload()
	const placed = canvas(page).locator('img[src^="/media/"]')
	await expect(placed).toBeVisible()

	await placed.click()
	await expect(placed).toBeVisible()

	await page.getByRole('button', { name: 'Publish', exact: true }).click()
	await expect(page.locator('#root').getByText('Post published.')).toBeVisible()

	const addressed = await page.request.get(`/api/content/${postId}`)
	const post = (await addressed.json()) as { path: string }
	await page.goto(`/${post.path}`)
	const published = page.locator(`img[src^="/media/"]`)
	await expect(published).toBeVisible()
	const src = await published.getAttribute('src')
	const served = await page.request.get(src as string)
	expect(served.status()).toBe(200)
})

test('lists the library as a grid of cards, each title bold above the name of who uploaded it', async ({ page }) => {
	const title = `${LIBRARY}-grid`
	await stored(page, title)
	await page.setViewportSize(LAPTOP)

	await page.goto(`/admin/media?search=${LIBRARY}`)

	const card = cardOf(page, title)
	await expect(card).toBeVisible()
	await expect(card.locator('.dataviews-title-field')).toHaveCSS('font-weight', '600')
	await expect(card.getByText(credentials.name, { exact: true })).toBeVisible()
	await expect(page.getByRole('button', { name: /layout/i })).toHaveCount(0)
})

test('keeps a sort and a Type filter the reader picks across a reload', async ({ page }) => {
	const title = `${LIBRARY}-kept`
	await stored(page, title)
	await page.goto(`/admin/media?search=${LIBRARY}`)
	await page.getByRole('button', { name: 'View options' }).click()
	await page.getByRole('combobox', { name: 'Sort by' }).selectOption('title')
	await page.keyboard.press('Escape')
	await page.getByRole('button', { name: 'Add filter' }).click()
	await page.getByRole('menuitem', { name: 'Type', exact: true }).click()
	await page.getByRole('option', { name: 'Images' }).click()
	await expect(page).toHaveURL(/sort=title/)
	await expect(page).toHaveURL(/media_type/)

	await page.reload()

	await expect(cardOf(page, title)).toBeVisible()
	await page.getByRole('button', { name: 'Filter', exact: true }).click()
	await expect(page.locator('.dataviews-filters__summary-chip')).toContainText('Images')
	await page.getByRole('button', { name: 'View options' }).click()
	await expect(page.getByRole('combobox', { name: 'Sort by' })).toHaveValue('title')
})

test('opens Edit media from the title of a card in a 560 pixel dialog that saves and says so', async ({ page }) => {
	const title = `${LIBRARY}-edited`
	const id = await stored(page, title)
	await page.setViewportSize(LAPTOP)
	await page.goto(`/admin/media?search=${LIBRARY}`)

	await cardOf(page, title).getByText(title, { exact: true }).click()
	const dialog = page.getByRole('dialog', { name: 'Edit media' })
	await expect(dialog.getByRole('textbox', { name: 'Title' })).toBeFocused()
	await frameBox(page, 'width').toBe(560)
	await dialog.getByRole('textbox', { name: 'Alt text' }).fill('Boats leaving the harbor')
	await dialog.getByRole('button', { name: 'Save' }).click()

	await expect(dialog).toHaveCount(0)
	await expect(page.locator('.godmin-toast', { hasText: 'Media file updated.' })).toBeVisible()
	const read = await page.request.get(`/api/media/${id}`)
	expect(((await read.json()) as { alt_text: string }).alt_text).toBe('Boats leaving the harbor')
})

test('renames a file from its card menu in a 560 pixel dialog, the Name box focused', async ({ page }) => {
	const title = `${LIBRARY}-named`
	await stored(page, title)
	await openCardMenu(page, title)

	await page.getByRole('menuitem', { name: 'Rename…' }).click()
	const dialog = page.getByRole('dialog', { name: 'Rename' })
	await expect(dialog.getByRole('textbox', { name: 'Name' })).toBeFocused()
	await frameBox(page, 'width').toBe(560)
	await dialog.getByRole('textbox', { name: 'Name' }).fill(`${LIBRARY}-renamed`)
	await dialog.getByRole('button', { name: 'Save' }).click()

	await expect(dialog).toHaveCount(0)
	await expect(page.locator('.godmin-toast', { hasText: 'Name updated.' })).toBeVisible()
	await expect(cardOf(page, `${LIBRARY}-renamed`)).toBeVisible()
})

test('gives the focus back to the card menu once a dialog it opened is closed', async ({ page }) => {
	const title = `${LIBRARY}-returned`
	await stored(page, title)
	await openCardMenu(page, title)

	await page.getByRole('menuitem', { name: 'Edit…' }).click()
	const dialog = page.getByRole('dialog', { name: 'Edit media' })
	await expect(dialog.getByRole('textbox', { name: 'Title' })).toBeFocused()
	await dialog.getByRole('button', { name: 'Cancel' }).click()

	await expect(dialog).toHaveCount(0)
	await expect(cardOf(page, title).getByRole('button', { name: 'Actions' })).toBeFocused()
})

test('gives the focus back to the card menu once Edit saves and a slow server lists the library again', async ({
	page,
}) => {
	const title = `${LIBRARY}-saved`
	await stored(page, title)
	await openCardMenu(page, title)

	await page.getByRole('menuitem', { name: 'Edit…' }).click()
	const dialog = page.getByRole('dialog', { name: 'Edit media' })
	await dialog.getByRole('textbox', { name: 'Alt text' }).fill('Boats leaving the harbor')
	await slowListings(page)
	await dialog.getByRole('button', { name: 'Save' }).click()

	await expect(page.locator('.godmin-toast', { hasText: 'Media file updated.' })).toBeVisible()
	await expect(cardOf(page, title).getByRole('button', { name: 'Actions' })).toBeFocused()
})

for (const { speed, hold } of LISTINGS) {
	test(`gives the focus back to the card title Edit media opened from once it saves and a ${speed} server lists again`, async ({
		page,
	}) => {
		const title = `${LIBRARY}-title-${speed}`
		await stored(page, title)
		await page.setViewportSize(LAPTOP)
		await page.goto(`/admin/media?search=${LIBRARY}`)
		const trigger = cardOf(page, title).locator('.dataviews-title-field')

		await trigger.click()
		const dialog = page.getByRole('dialog', { name: 'Edit media' })
		await dialog.getByRole('textbox', { name: 'Alt text' }).fill('Boats leaving the harbor')
		await hold(page)
		await dialog.getByRole('button', { name: 'Save' }).click()

		await expect(page.locator('.godmin-toast', { hasText: 'Media file updated.' })).toBeVisible()
		await expect(trigger).toBeFocused()
	})
}

test('scrolls the fields of Edit media at 1280 by 900 from the keyboard, through a pane named Scrollable section', async ({
	page,
}) => {
	const title = `${LIBRARY}-scrolled`
	await stored(page, title)
	await page.setViewportSize(LAPTOP)
	await openCardMenu(page, title)
	await page.getByRole('menuitem', { name: 'Edit…' }).click()
	const dialog = page.getByRole('dialog', { name: 'Edit media' })
	const pane = dialog.locator('.gophenberg-media__edit-fields')
	const dimensions = pane.getByText('Dimensions', { exact: true })
	const header = dialog.locator('.components-modal__header')
	await dialog.getByRole('textbox', { name: 'Description' }).focus()
	await expect(dimensions).not.toBeInViewport()
	await expect(header).toHaveCSS('border-bottom-color', 'rgba(0, 0, 0, 0)')

	for (const stop of ['Cancel', 'Save', 'Close']) {
		await page.keyboard.press('Tab')
		await expect(dialog.getByRole('button', { name: stop })).toBeFocused()
	}
	await page.keyboard.press('Tab')
	await expect(dialog.getByRole('document', { name: 'Scrollable section' })).toBeFocused()
	await expect(pane).toHaveCSS('box-shadow', /inset$/)
	await page.keyboard.press('End')

	await expect(dimensions).toBeInViewport()
	await expect(header).not.toHaveCSS('border-bottom-color', 'rgba(0, 0, 0, 0)')
	await expect.poll(() => gapUnder(dimensions)).toBeGreaterThanOrEqual(24)
})

/**
 * Measures the space between the bottom of a field and the bottom edge of the Edit pane it scrolls in.
 * @param field - A field inside the pane.
 * @returns The gap in CSS pixels.
 */
function gapUnder(field: Locator): Promise<number> {
	return field.evaluate((element) => {
		const pane = element.closest('.gophenberg-media__edit-fields') as Element
		return Math.round(pane.getBoundingClientRect().bottom - element.getBoundingClientRect().bottom)
	})
}

for (const size of [LAPTOP, DESKTOP]) {
	test(`keeps Cancel and Save of Edit media in view at ${size.width} by ${size.height}, the fields scrolling above them`, async ({
		page,
	}) => {
		const title = `${LIBRARY}-tall-${size.width}`
		await stored(page, title)
		await page.setViewportSize(size)
		await openCardMenu(page, title)

		await page.getByRole('menuitem', { name: 'Edit…' }).click()
		const dialog = page.getByRole('dialog', { name: 'Edit media' })
		await expect(dialog.getByRole('textbox', { name: 'Title' })).toBeFocused()

		await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeInViewport({ ratio: 1 })
		await expect(dialog.getByRole('button', { name: 'Save' })).toBeInViewport({ ratio: 1 })
	})
}

test('deletes a file behind a headerless 560 pixel confirm, Cancel focused', async ({ page }) => {
	const title = `${LIBRARY}-deleted`
	await stored(page, title)
	await openCardMenu(page, title)

	await page.getByRole('menuitem', { name: 'Permanently delete…' }).click()
	const dialog = page.getByRole('dialog')
	await expect(dialog.getByText(`Delete "${title}" for good? This cannot be undone.`)).toBeVisible()
	await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
	await expect(dialog.getByRole('heading')).toHaveCount(0)
	await frameBox(page, 'width').toBe(560)
	await dialog.getByRole('button', { name: 'Delete permanently' }).click()

	await expect(page.locator('.godmin-toast', { hasText: `"${title}" permanently deleted.` })).toBeVisible()
	await expect(cardOf(page, title)).toHaveCount(0)
})

test('keeps a Rename opened on the last card of a row, with what was typed, once an upload lands a file before it', async ({
	page,
}) => {
	for (const at of ['a', 'b', 'c', 'd', 'e', 'f']) {
		await stored(page, `${LIBRARY}-row-${at}`)
	}
	await page.setViewportSize(LAPTOP)
	await page.goto(`/admin/media?search=${LIBRARY}`)
	const last = page.locator('.dataviews-view-grid__row').first().locator('.dataviews-view-grid__card').last()
	const title = (await last.locator('.dataviews-title-field').textContent()) as string
	let release = () => {}
	const held = new Promise<void>((resolve) => {
		release = resolve
	})
	await page.route('**/api/media', async (route) => {
		if (route.request().method() === 'POST') {
			await held
		}
		await route.continue()
	})
	const chooser = page.waitForEvent('filechooser')
	await page.getByRole('main').getByRole('button', { name: 'Add Media File' }).click()
	await (await chooser).setFiles(photo(`${LIBRARY}-row-landed.jpg`))
	await last.getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Rename…' }).click()
	const name = page.getByRole('dialog', { name: 'Rename' }).getByRole('textbox', { name: 'Name' })
	await expect(name).toBeFocused()
	await name.press('End')
	await name.pressSequentially(' kept')

	release()
	await expect(page.locator('.godmin-toast', { hasText: `"${LIBRARY}-row-landed.jpg" uploaded.` })).toBeVisible()

	await expect(name).toHaveValue(`${title} kept`)
	await expect(name).toBeFocused()
})

test('narrows and sorts the media library picker with the pointer, then places the picked picture', async ({
	page,
}) => {
	const title = `${LIBRARY}-pointed`
	await stored(page, title)
	const library = await openPicker(page)
	await expect(library.getByRole('option', { name: title })).toBeVisible()

	await library.getByRole('button', { name: 'Add filter' }).click()
	await page.getByRole('menuitem', { name: 'Author' }).click()
	const narrowed = listingAsking(page, { author: await sessionId(page) })
	await library.getByRole('option', { name: credentials.name }).click()
	await narrowed
	await page.keyboard.press('Escape')
	await library.getByRole('button', { name: 'View options' }).click()
	const sorted = listingAsking(page, { orderby: 'modified' })
	await library.getByRole('combobox', { name: 'Sort by' }).selectOption('modified')
	await sorted
	await page.keyboard.press('Escape')
	await library.getByRole('option', { name: title }).click()
	await library.getByRole('button', { name: 'Select' }).click()

	await expect(library).toHaveCount(0)
	await expect(canvas(page).locator('img[src^="/media/"]')).toBeVisible()
})

test('searches, narrows, sorts and places from the media library picker by keyboard, each popover inside it', async ({
	page,
}) => {
	const title = `${LIBRARY}-typed`
	await stored(page, title)
	const library = await openPicker(page)
	await library.getByRole('searchbox', { name: 'Search media…' }).focus()
	await page.keyboard.type(title)
	await expect(library.getByRole('option', { name: title })).toBeVisible()

	await library.getByRole('button', { name: 'Add filter' }).focus()
	await page.keyboard.press('Enter')
	await expect(page.getByRole('menuitem', { name: 'Author' })).toBeFocused()
	await page.keyboard.press('Enter')
	await expect(library.getByRole('combobox', { name: 'Conditions' })).toBeFocused()
	const authors = library.getByRole('listbox', { name: 'List of: Author' })
	await expect(async () => {
		await page.keyboard.press('Tab')
		await expect(authors).toBeFocused({ timeout: 500 })
	}).toPass()
	const mine = authors.getByRole('option', { name: credentials.name })
	for (let step = 0; step < 10 && (await mine.getAttribute('data-active-item')) === null; step++) {
		await page.keyboard.press('ArrowDown')
	}
	const narrowed = listingAsking(page, { author: await sessionId(page) })
	await page.keyboard.press('Enter')
	await narrowed
	await expect(authors).toBeFocused()
	await page.keyboard.press('Escape')
	await library.getByRole('button', { name: 'View options' }).focus()
	await page.keyboard.press('Enter')
	await expect(library.getByRole('combobox', { name: 'Sort by' })).toBeFocused()
	const sorted = page.waitForResponse((answer) => new URL(answer.url()).searchParams.get('orderby') === 'modified')
	await page.keyboard.press('ArrowDown')
	await sorted
	await page.keyboard.press('Escape')
	await expect(library.getByRole('button', { name: 'View options' })).toBeFocused()
	await expect(library.getByRole('option', { name: title })).toBeVisible()
	await library.getByRole('button', { name: 'Reset', exact: true }).focus()
	await page.keyboard.press('Tab')
	await page.keyboard.press('Space')
	await page.keyboard.press('Tab')
	await expect(library.getByRole('button', { name: 'Select' })).toBeFocused()
	await page.keyboard.press('Enter')

	await expect(library).toHaveCount(0)
	await expect(canvas(page).locator('img[src^="/media/"]')).toBeVisible()
})

for (const { size, inset } of PICKER_INSETS) {
	test(`fills a window ${size.width} pixels wide but for ${inset} pixels at each edge, under a 72 pixel header, with no ring or border on its frame`, async ({
		page,
	}) => {
		const library = await openPicker(page, size)

		await expect
			.poll(async () => gaps(await boxOf(library), { x: 0, y: 0, ...size }))
			.toEqual({ left: inset, right: inset, top: inset, bottom: inset })
		const header = await boxOf(library.locator('.gophenberg-media-picker__header'))
		expect(Math.round(header.height)).toBe(72)
		const upload = await boxOf(library.getByRole('button', { name: 'Upload media' }))
		expect(Math.round(upload.height)).toBe(40)
		await expect(library.getByRole('heading', { name: 'Select Media' })).toHaveCSS('line-height', '20px')
		await expect(library).toBeFocused()
		await expect(library).toHaveCSS('outline-style', 'none')
		await expect(library).toHaveCSS('border-top-width', '0px')
	})
}

test('never scrolls the picker sideways when a crowded footer pushes Select past its edge and Select takes the focus', async ({
	page,
}) => {
	const title = `${LIBRARY}-past-the-edge`
	await stored(page, title)
	await page.route('**/api/media?*', async (route) => {
		const answer = await route.fetch()
		const body = (await answer.json()) as Record<string, unknown>
		await route.fulfill({ response: answer, json: { ...body, total: 120 } })
	})
	const library = await openPicker(page, { width: 320, height: PHONE.height })
	await library.getByRole('option', { name: title }).click()

	await library.getByRole('button', { name: 'Select' }).focus()

	const content = library.locator('.gophenberg-media-picker__content')
	expect(await content.evaluate((element) => element.scrollLeft)).toBe(0)
})

test('says in a toast why the server refused a file an Image block sent, the block left empty', async ({
	page,
}, testInfo) => {
	const name = `unknown-e2e-${RUN}.zzz`
	const path = testInfo.outputPath(name)
	writeFileSync(path, 'Notes on the harbor.')
	await newPost(page)
	await addImageBlock(page)

	const chooser = page.waitForEvent('filechooser')
	await canvas(page).getByRole('button', { name: 'Upload' }).click()
	await (await chooser).setFiles(path)

	const reason =
		'Gophenberg does not accept zzz files. Upload a picture, a video, an audio file, a PDF or a zip instead.'
	await expect(page.locator('.godmin-toast', { hasText: `${name}: ${reason}` })).toBeVisible()
	await expect(canvas(page).getByRole('button', { name: 'Media Library' })).toBeVisible()
})

test('closes only the filter panel on Escape in the Author search box, past ten authors, the tick kept', async ({
	page,
}) => {
	const title = `${LIBRARY}-escaped`
	await stored(page, title)
	await page.route('**/api/authors*', async (route) => {
		const answer = await route.fetch()
		const body = (await answer.json()) as { items: unknown[] }
		const samples = Array.from({ length: 12 }, (_, at) => ({
			id: `00000000-0000-4000-8000-0000000000${String(at + 10)}`,
			name: `Sample Author ${String(at)}`,
		}))
		await route.fulfill({ response: answer, json: { ...body, items: [...body.items, ...samples] } })
	})
	const library = await openPicker(page)
	await library.getByRole('option', { name: title }).click()
	await library.getByRole('button', { name: 'Add filter' }).click()
	await page.getByRole('menuitem', { name: 'Author' }).click()
	const search = library.getByRole('combobox', { name: 'Search items' })
	await search.click()
	await page.keyboard.type('Sample')

	await page.keyboard.press('Escape')

	await expect(search).toHaveCount(0)
	await expect(library).toBeVisible()
	await expect(library.getByRole('option', { name: title, selected: true })).toBeVisible()
	await expect(library.locator('.dataviews-filters__summary-chip')).toBeFocused()
})

test('uploads from the picker, ticks the new card, raises a toast that takes a press and places the picture', async ({
	page,
}) => {
	const title = `${PICTURE}-picked`
	const library = await openPicker(page)

	await chooseFile(library, photo(`${title}.jpg`))

	await expect(library.getByRole('option', { name: title, selected: true })).toBeVisible()
	const toast = library.locator('.godmin-toast', { hasText: `"${title}.jpg" uploaded.` })
	await toast.click()
	await expect(toast).toHaveCount(0)
	await library.getByRole('button', { name: 'Select' }).click()

	await expect(library).toHaveCount(0)
	await expect(canvas(page).locator('img[src^="/media/"]')).toBeVisible()
})

test('uploads a picture dropped on the picker, over a zone filling its frame and reading Drop files to upload', async ({
	page,
}) => {
	const name = `${PICTURE}-carried.jpg`
	const library = await openPicker(page)
	const transfer = await dragCarrying(page, photo(name))
	const zone = library.locator('.components-drop-zone')

	await library.dispatchEvent('dragenter', { dataTransfer: transfer })
	await zone.dispatchEvent('dragenter', { dataTransfer: transfer })
	await expect(zone).toHaveClass(/is-dragging-over-element/)
	await expect(zone.locator('.components-drop-zone__content-inner')).toHaveCSS('opacity', '1')
	await expect(zone.getByText('Drop files to upload')).toBeVisible()
	await expect(zone.getByText('Drop files to upload')).toHaveCSS('line-height', '18.2px')
	await expect.poll(() => gapsWithin(zone, library)).toEqual({ left: 0, right: 0, top: 0, bottom: 0 })
	await zone.dispatchEvent('drop', { dataTransfer: transfer })

	await expect(library.locator('.godmin-toast', { hasText: `"${name}" uploaded.` })).toBeVisible()
})

test('lists a landed file in a panel of tight rows beside a refused one, its trigger in the accent colour while open', async ({
	page,
}) => {
	const name = `${PICTURE}-listed.jpg`
	const library = await openPicker(page)

	await chooseFile(library, [photo(name), NOTES])

	const panel = page.locator('.gophenberg-media-picker__panel')
	await expect(panel.getByText(name, { exact: true })).toHaveCSS('line-height', '16.8px')
	await expect(panel.getByRole('heading')).toHaveCSS('line-height', '18.2px')
	const trigger = library.locator('.gophenberg-media-picker__status').getByRole('button')
	await expect(trigger).toHaveCSS('color', 'rgb(56, 88, 233)')
})

test('names a refused file in a 320 pixel panel, its notice filling the row and the status 24 pixels in', async ({
	page,
}) => {
	const library = await openPicker(page)

	await chooseFile(library, NOTES)

	const panel = page.locator('.gophenberg-media-picker__panel')
	await expect(panel).toContainText(`${NOTES.name}: Sorry, this file type is not supported here.`)
	await expect(panel).not.toContainText(`${NOTES.name}: ${NOTES.name}`)
	await expect.poll(async () => Math.round((await boxOf(panel)).width)).toBe(320)
	const notice = panel.locator('.gophenberg-media-picker__error')
	await expect
		.poll(() => gapsWithin(notice, panel.locator('.gophenberg-media-picker__row')))
		.toMatchObject({ left: 16, right: 16 })
	const status = library.locator('.gophenberg-media-picker__status')
	await expect(status).toContainText('1 upload error')
	await expect
		.poll(() => gapsWithin(status, library.locator('.gophenberg-media-picker__footer')))
		.toMatchObject({ left: 24 })
})

for (const size of [DESKTOP, PHONE]) {
	test(`keeps a failed Select read inside the picker at ${size.width} by ${size.height}, the notice 24 pixels from its sides`, async ({
		page,
	}) => {
		const title = `${LIBRARY}-unread-${size.width}`
		await stored(page, title)
		const library = await openPicker(page, size)
		await library.getByRole('option', { name: title }).click()
		await page.route('**/api/media?ids=*', (route) => route.fulfill({ status: 500 }))
		const select = library.getByRole('button', { name: 'Select' })

		await select.click()

		const notice = library.getByRole('alert')
		await expect(notice).toHaveText('The media library could not be loaded.')
		await expect
			.poll(() => gapsWithin(notice, library.locator('.gophenberg-media-picker__content')))
			.toMatchObject({ left: 24, right: 24 })
		const room = await gapsWithin(select, library)
		expect(room.right).toBeGreaterThanOrEqual(0)
		expect(room.bottom).toBeGreaterThanOrEqual(0)
	})
}

test('holds a sort from one Image block to the next in a post, and opens newest first again after a reload', async ({
	page,
}) => {
	const blocks = await openTwoImageBlocks(page)
	const library = page.getByRole('dialog', { name: 'Select Media' })
	await openPickerFrom(blocks.first())
	const sorted = listingAsking(page, { orderby: 'title' })
	await (await sortControl(library)).selectOption('title')
	await sorted
	await page.keyboard.press('Escape')
	await library.getByRole('button', { name: 'Close', exact: true }).click()
	await expect(library).toHaveCount(0)

	await openPickerFrom(blocks.last())
	await expect(await sortControl(library)).toHaveValue('title')

	await page.reload()
	const newest = listingAsking(page, { orderby: 'date', order: 'desc' })
	await openPickerFrom(blocks.last())
	await newest
	await expect(await sortControl(library)).toHaveValue('date')
})

test.describe('on a phone', () => {
	test.use({ viewport: PHONE, isMobile: true, hasTouch: true })

	test('lists the library one file per row and opens Edit media as a sheet from a tap on a row', async ({ page }) => {
		const title = `${LIBRARY}-phone`
		await stored(page, title)

		await page.goto(`/admin/media?search=${LIBRARY}`)

		await expect(page.locator('.dataviews-view-list')).toBeVisible()
		await expect(page.locator('.dataviews-view-grid')).toHaveCount(0)
		await page.getByRole('button', { name: new RegExp(`^${title}`) }).tap()
		await expect(page.getByRole('dialog', { name: 'Edit media' })).toBeVisible()
		await frameBox(page, 'width').toBe(PHONE.width)
		await frameBox(page, 'bottom').toBe(PHONE.height)
	})

	test('keeps Cancel and Save of Edit media in view in the sheet, the fields scrolling above them', async ({ page }) => {
		const title = `${LIBRARY}-sheet`
		await stored(page, title)
		await page.goto(`/admin/media?search=${LIBRARY}`)

		await page.getByRole('button', { name: new RegExp(`^${title}`) }).tap()
		const dialog = page.getByRole('dialog', { name: 'Edit media' })
		await frameBox(page, 'bottom').toBe(PHONE.height)

		await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeInViewport({ ratio: 1 })
		await expect(dialog.getByRole('button', { name: 'Save' })).toBeInViewport({ ratio: 1 })
	})

	for (const { speed, hold } of LISTINGS) {
		test(`gives the focus back to the tapped row once Edit media saves and a ${speed} server lists again`, async ({
			page,
		}) => {
			const title = `${LIBRARY}-tap-${speed}`
			await stored(page, title)
			await page.goto(`/admin/media?search=${LIBRARY}`)
			const trigger = page.getByRole('button', { name: new RegExp(`^${title}`) })

			await trigger.tap()
			const dialog = page.getByRole('dialog', { name: 'Edit media' })
			await dialog.getByRole('textbox', { name: 'Alt text' }).fill('Boats leaving the harbor')
			await hold(page)
			await dialog.getByRole('button', { name: 'Save' }).tap()

			await expect(page.locator('.godmin-toast', { hasText: 'Media file updated.' })).toBeVisible()
			await expect(trigger).toBeFocused()
		})
	}

	test('gives the focus back to the tapped row once Edit media fails to save and a slow server lists again', async ({
		page,
	}) => {
		const title = `${LIBRARY}-tap-failed`
		await stored(page, title)
		await page.goto(`/admin/media?search=${LIBRARY}`)
		const trigger = page.getByRole('button', { name: new RegExp(`^${title}`) })

		await trigger.tap()
		const dialog = page.getByRole('dialog', { name: 'Edit media' })
		await dialog.getByRole('textbox', { name: 'Alt text' }).fill('Boats leaving the harbor')
		await page.route('**/api/media/*', (route) => (route.request().method() === 'PATCH' ? route.abort() : route.continue()))
		await slowListings(page)
		await dialog.getByRole('button', { name: 'Save' }).tap()

		await expect(page.locator('.godmin-toast', { hasText: 'Could not save image.' })).toBeVisible()
		await expect(trigger).toBeFocused()
	})
})
