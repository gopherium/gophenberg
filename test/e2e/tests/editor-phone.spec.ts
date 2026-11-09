// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

const RUN = Math.random().toString(36).slice(2, 8)

const PHONE = { width: 390, height: 844 }

const DESKTOP = { width: 1280, height: 900 }

const SIDEBAR = '.gophenberg-editor__sidebar'

const SIDEBAR_WIDTH = 280

const OUTLINE = '.gophenberg-editor__outline'

const PARAGRAPHS = ['The first paragraph the phone spec wrote.', 'The second paragraph the phone spec wrote.']

const CONTENT = PARAGRAPHS.map((words) => `<!-- wp:paragraph -->\n<p>${words}</p>\n<!-- /wp:paragraph -->`).join('\n\n')

const LONG_TITLE = `A draft whose title runs on far past the room the header keeps for its name ${RUN}`

const created: string[] = []

/**
 * Stores a draft holding two paragraphs and opens it in the editor.
 * @param page - The page to drive.
 * @param title - The title of the draft.
 */
async function openDraft(page: Page, title = `A draft the phone spec opened ${RUN}`) {
	const answer = await page.request.post('/api/content', { data: { type: 'post', title, content: CONTENT } })
	expect(answer.status()).toBe(201)
	const { id } = (await answer.json()) as { id: string }
	created.push(id)
	await page.goto(`/admin/content/post/${id}/edit`)
	await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
}

/**
 * Returns the canvas the block editor writes into.
 * @param page - The page holding the editor.
 * @returns The canvas frame.
 */
function canvas(page: Page) {
	return page.frameLocator('iframe[name="editor-canvas"]')
}

/**
 * Returns the box an element takes on screen, in whole pixels.
 * @param locator - The element to measure.
 * @returns Its left, top, width and height.
 */
async function box(locator: Locator) {
	const found = await locator.boundingBox()
	if (found === null) {
		throw new Error('the element takes no box on screen')
	}
	return {
		x: Math.round(found.x),
		y: Math.round(found.y),
		width: Math.round(found.width),
		height: Math.round(found.height),
	}
}

/**
 * Returns the left and right edges of an element, unrounded.
 * @param locator - The element to measure.
 * @returns Its left and right edges.
 */
async function edges(locator: Locator) {
	const found = await locator.boundingBox()
	if (found === null) {
		throw new Error('the element takes no box on screen')
	}
	return { left: found.x, right: found.x + found.width }
}

/**
 * Clicks a control, then reads each frame for 400ms where a panel's left edge sits and how wide the page grows.
 * @param control - The control to click.
 * @param panel - The selector of the panel the click moves.
 * @returns The left edges the panel showed at, frame by frame, and the widest the page grew.
 */
function watchMove(control: Locator, panel: string) {
	return control.evaluate(
		(element, selector) =>
			new Promise<{ lefts: number[], widest: number }>((done) => {
				const lefts: number[] = []
				let widest = 0
				const start = performance.now()
				const sample = () => {
					const held = document.querySelector(selector)?.getBoundingClientRect()
					if (held !== undefined && held.width > 0) {
						lefts.push(Math.round(held.left))
					}
					widest = Math.max(widest, document.documentElement.scrollWidth)
					if (performance.now() - start < 400) {
						requestAnimationFrame(sample)
						return
					}
					done({ lefts, widest })
				}
				const target = element as HTMLElement
				target.click()
				requestAnimationFrame(sample)
			}),
		panel,
	)
}

/**
 * Records, from the first paint of every page load, each transition the settings sidebar starts.
 * @param page - The page to watch.
 */
async function recordSidebarTransitions(page: Page) {
	await page.addInitScript((selector) => {
		const runs: string[] = []
		Object.assign(window, { sidebarRuns: runs })
		document.addEventListener(
			'transitionrun',
			(event) => {
				if (event.target instanceof Element && event.target.matches(selector)) {
					runs.push(event.propertyName)
				}
			},
			true,
		)
	}, SIDEBAR)
}

/**
 * Returns the transitions the settings sidebar started since the page loaded, once two more frames have drawn.
 * @param page - The page being watched.
 * @returns The names of the properties that started a transition.
 */
function sidebarTransitions(page: Page) {
	return page.evaluate(
		() =>
			new Promise<string[]>((done) => {
				requestAnimationFrame(() =>
					requestAnimationFrame(() => done((window as unknown as { sidebarRuns: string[] }).sidebarRuns)),
				)
			}),
	)
}

/**
 * Tells whether any of the left edges a panel showed at lies strictly between two edges.
 * @param lefts - The left edges, frame by frame.
 * @param from - One end of the way.
 * @param to - The other end of the way.
 * @returns Whether the panel was caught on its way.
 */
function caughtBetween(lefts: number[], from: number, to: number) {
	return lefts.some((left) => left > Math.min(from, to) && left < Math.max(from, to))
}

/**
 * Returns the bottom edge of the editor header.
 * @param page - The page holding the editor.
 * @returns The bottom of the header in whole pixels.
 */
async function headerBottom(page: Page) {
	const header = await box(page.locator('.gophenberg-editor__header'))
	return header.y + header.height
}

/**
 * Tells whether the element drawn at a point of the screen sits inside the region.
 * @param page - The page to read.
 * @param region - The selector of the region.
 * @param x - The left of the point.
 * @param y - The top of the point.
 * @returns Whether the region holds the topmost element at that point.
 */
function drawnBy(page: Page, region: string, x: number, y: number) {
	return page.evaluate(
		({ selector, left, top }) => document.elementFromPoint(left, top)?.closest(selector) !== null,
		{ selector: region, left: x, top: y },
	)
}

/**
 * Returns the settings button of the editor header.
 * @param page - The page holding the editor.
 * @returns The button.
 */
function settings(page: Page) {
	return page.getByRole('button', { name: 'Settings', exact: true })
}

test.afterEach(async ({ page }) => {
	for (const id of created.splice(0)) {
		await page.request.delete(`/api/content/${id}?force=true`)
	}
})

test.describe('on a phone', () => {
	test.use({ viewport: PHONE, isMobile: true, hasTouch: true })

	test('holds the page to the width of the screen', async ({ page }) => {
		await openDraft(page)

		const widths = await page.evaluate(() => ({
			inner: window.innerWidth,
			scroll: document.documentElement.scrollWidth,
		}))

		expect(widths).toEqual({ inner: PHONE.width, scroll: PHONE.width })
	})

	test('keeps every header control on screen with its words on one line', async ({ page }) => {
		await openDraft(page)
		const header = await box(page.locator('.gophenberg-editor__header'))

		const controls = [
			page.getByRole('link', { name: 'Back to posts' }),
			page.getByRole('button', { name: 'Add block' }),
			page.getByRole('button', { name: 'List view' }),
			page.getByRole('button', { name: 'Saved', exact: true }),
			settings(page),
			page.getByRole('button', { name: 'Publish', exact: true }),
		]

		for (const control of controls) {
			await expect(control).toBeVisible()
			const at = await box(control)
			expect(at.x).toBeGreaterThanOrEqual(0)
			expect(at.x + at.width).toBeLessThanOrEqual(PHONE.width)
			expect(at.height).toBeLessThanOrEqual(header.height)
		}
	})

	test('sets the actions against the end of the header', async ({ page }) => {
		await openDraft(page)

		const header = await edges(page.locator('.gophenberg-editor__header'))
		const publish = await edges(page.getByRole('button', { name: 'Publish', exact: true }))

		expect(Math.abs(publish.right - (header.right - 8))).toBeLessThan(1)
	})

	test('leaves undo, redo, the preview select, the state word and the document name out of the header', async ({
		page,
	}) => {
		await openDraft(page)
		const header = page.locator('.gophenberg-editor__header')

		await expect(header.getByRole('button', { name: 'Undo' })).toHaveCount(0)
		await expect(header.getByRole('button', { name: 'Redo' })).toHaveCount(0)
		await expect(header.getByRole('combobox', { name: 'Preview' })).toHaveCount(0)
		await expect(header.getByText('Saved', { exact: true })).toHaveCount(0)
		await expect(page.getByTestId('document-bar')).toHaveCount(0)
		await expect(page.locator('.gophenberg-editor__foot')).toHaveCount(0)
	})

	test('names the save icon after the state it shows', async ({ page }) => {
		await openDraft(page)
		await expect(page.getByRole('button', { name: 'Saved', exact: true })).toHaveAttribute('aria-disabled', 'true')

		await page.getByRole('textbox', { name: 'Title' }).fill(`A draft the save icon renamed ${RUN}`)

		const save = page.getByRole('button', { name: 'Save draft' })
		await expect(save).toBeVisible()
		await expect(save).not.toHaveAttribute('aria-disabled', 'true')
		await expect(save).toHaveText('')
	})

	test('opens the settings over the writing area at the full width of the screen', async ({ page }) => {
		await openDraft(page)
		const sidebar = page.locator('.gophenberg-editor__sidebar')
		await expect(sidebar).toBeHidden()
		await expect(settings(page)).toHaveAttribute('aria-expanded', 'false')

		await settings(page).tap()

		await expect(sidebar).toBeVisible()
		const top = await headerBottom(page)
		expect(await box(sidebar)).toEqual({ x: 0, y: top, width: PHONE.width, height: PHONE.height - top })
		await expect(page.getByRole('button', { name: 'Publish', exact: true })).toBeInViewport()
	})

	test('closes the settings from their own header and hands focus back to the settings button', async ({
		page,
	}) => {
		await openDraft(page)
		await settings(page).tap()

		await page.getByRole('button', { name: 'Close Settings' }).tap()

		await expect(page.locator('.gophenberg-editor__sidebar')).toBeHidden()
		await expect(settings(page)).toBeFocused()
	})

	test('holds a disabled block toolbar under the header before any block is selected', async ({ page }) => {
		await openDraft(page)
		const toolbar = page.getByRole('toolbar', { name: 'Block tools' })

		await expect(toolbar).toBeVisible()
		for (const name of ['Paragraph', 'Move up', 'Move down', 'Options']) {
			await expect(toolbar.getByRole('button', { name, exact: true })).toHaveAttribute('aria-disabled', 'true')
		}
		expect(await box(toolbar)).toEqual({ x: 0, y: await headerBottom(page), width: PHONE.width, height: 46 })
	})

	test('fills the block toolbar row once a paragraph is selected, the canvas staying put', async ({ page }) => {
		await openDraft(page)
		const frame = page.locator('iframe[name="editor-canvas"]')
		await expect(canvas(page).getByText(PARAGRAPHS[1])).toBeVisible()
		const before = await box(frame)

		await canvas(page).getByText(PARAGRAPHS[1]).tap()

		const toolbar = page.getByRole('toolbar', { name: 'Block tools' })
		await expect(toolbar.getByRole('button', { name: 'Move up' })).not.toHaveAttribute('aria-disabled', 'true')
		await expect(toolbar).toHaveCount(1)
		expect((await box(toolbar)).y).toBe(await headerBottom(page))
		expect((await box(frame)).y).toBe(before.y)
	})

	test('lays the settings over the block toolbar', async ({ page }) => {
		await openDraft(page)
		const row = await box(page.getByRole('toolbar', { name: 'Block tools' }))

		await settings(page).tap()
		await expect(page.locator('.gophenberg-editor__sidebar')).toBeVisible()

		expect(await drawnBy(page, '.gophenberg-editor__sidebar', row.x + row.width / 2, row.y + row.height / 2)).toBe(
			true,
		)
	})

	test('pads the title and the paragraphs thirty pixels in from each edge', async ({ page }) => {
		await openDraft(page)
		await expect(canvas(page).getByText(PARAGRAPHS[0])).toBeVisible()

		const title = await box(page.getByRole('textbox', { name: 'Title' }))
		const paragraph = await box(canvas(page).getByText(PARAGRAPHS[0]))

		expect([title.x, title.width]).toEqual([30, PHONE.width - 60])
		expect([paragraph.x, paragraph.width]).toEqual([30, PHONE.width - 60])
	})

	test('opens the list view over the writing area and lets the settings lie over it', async ({ page }) => {
		await openDraft(page)
		const outline = page.locator('.gophenberg-editor__outline')

		await page.getByRole('button', { name: 'List view' }).tap()

		await expect(outline).toBeVisible()
		const top = await headerBottom(page)
		await expect.poll(() => box(outline)).toEqual({ x: 0, y: top, width: PHONE.width, height: PHONE.height - top })
		await expect.poll(() => outline.evaluate((held) => held.contains(document.activeElement))).toBe(true)

		await settings(page).tap()
		await expect(page.locator('.gophenberg-editor__sidebar')).toBeVisible()
		expect(await drawnBy(page, '.gophenberg-editor__sidebar', PHONE.width / 2, PHONE.height / 2)).toBe(true)

		await page.getByRole('button', { name: 'Close Settings' }).tap()
		await expect(outline).toBeVisible()
	})

	test('slides the list view in from the start edge without widening the page', async ({ page }) => {
		await openDraft(page)

		const moved = await watchMove(page.getByRole('button', { name: 'List view' }), OUTLINE)

		expect(caughtBetween(moved.lefts, -PHONE.width, 0)).toBe(true)
		expect(moved.lefts.at(-1)).toBe(0)
		expect(moved.widest).toBe(PHONE.width)
	})

	test.describe('under reduced motion', () => {
		test.use({ reducedMotion: 'reduce' })

		test('raises the saved message across the foot of the screen on one line', async ({ page }) => {
			await openDraft(page)
			await page.getByRole('textbox', { name: 'Title' }).fill(`A draft the phone toast saved ${RUN}`)

			await page.getByRole('button', { name: 'Save draft' }).tap()

			const toast = page.locator('#root').getByText('Draft saved.')
			await expect(toast).toBeVisible()
			const placed = () => toast.evaluate((element) => {
				const held = element.closest('.godmin-toast') as HTMLElement
				const style = getComputedStyle(held)
				const inset = ['padding-top', 'padding-bottom', 'border-top-width', 'border-bottom-width'].reduce(
					(total, side) => total + Number.parseFloat(style.getPropertyValue(side)),
					0,
				)
				const card = held.getBoundingClientRect()
				return {
					centre: Math.round((card.left + card.right) / 2),
					gap: Math.round(window.innerHeight - card.bottom),
					lines: Math.round((card.height - inset) / Number.parseFloat(style.lineHeight)),
				}
			})
			await expect.poll(placed).toEqual({ centre: PHONE.width / 2, gap: 32, lines: 1 })
		})
	})

	test('opens the block inserter inside the screen', async ({ page }) => {
		await openDraft(page)

		await page.getByRole('button', { name: 'Add block' }).tap()

		const search = page.getByRole('searchbox')
		await expect(search).toBeVisible()
		const panel = await box(page.locator('.block-editor-inserter__popover .components-popover__content'))
		expect(panel.x).toBeGreaterThanOrEqual(0)
		expect(panel.x + panel.width).toBeLessThanOrEqual(PHONE.width)
	})
})

test.describe('across widths', () => {
	test('drops the type from the document name at 600 and brings it back at 601', async ({ page }) => {
		await page.setViewportSize({ width: 600, height: 900 })
		await openDraft(page)
		const bar = page.getByTestId('document-bar')
		await expect(bar).toContainText('A draft the phone spec opened')
		await expect(bar).not.toContainText('·')

		await page.setViewportSize({ width: 601, height: 900 })

		await expect(bar).toContainText('·')
	})

	test('keeps the document name at 404 and drops it at 403', async ({ page }) => {
		await page.setViewportSize({ width: 404, height: 900 })
		await openDraft(page)
		await expect(page.getByTestId('document-bar')).toBeVisible()

		await page.setViewportSize({ width: 403, height: 900 })

		await expect(page.getByTestId('document-bar')).toHaveCount(0)
	})

	test('names the save words after the state they show from 600 and saves through an icon below', async ({ page }) => {
		await page.setViewportSize({ width: 600, height: 900 })
		await openDraft(page)
		await expect(page.getByRole('button', { name: 'Saved', exact: true })).toHaveText('Saved')

		await page.getByRole('textbox', { name: 'Title' }).fill(`A draft the save words renamed ${RUN}`)
		await expect(page.getByRole('button', { name: 'Save draft' })).toHaveText('Save draft')
		await page.getByRole('button', { name: 'Save draft' }).click()

		await expect(page.getByRole('button', { name: 'Saved', exact: true })).toHaveText('Saved')
		await page.setViewportSize({ width: 599, height: 900 })
		await expect(page.getByRole('button', { name: 'Saved', exact: true })).toHaveText('')
	})

	test('leaves undo out at 781 and offers it from 782', async ({ page }) => {
		await page.setViewportSize({ width: 781, height: 900 })
		await openDraft(page)
		await expect(page.getByRole('button', { name: 'Undo' })).toHaveCount(0)

		await page.setViewportSize({ width: 782, height: 900 })

		await expect(page.getByRole('button', { name: 'Undo' })).toBeVisible()
	})

	test('opens the settings on load at 782 and keeps them shut on load at 781', async ({ page }) => {
		await page.setViewportSize({ width: 782, height: 900 })
		await openDraft(page)
		await expect(page.locator('.gophenberg-editor__sidebar')).toBeVisible()

		await page.setViewportSize({ width: 781, height: 900 })
		await page.reload()

		await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible()
		await expect(page.locator('.gophenberg-editor__sidebar')).toBeHidden()
	})

	test('closes the settings as the window narrows and opens them again as it widens', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 900 })
		await openDraft(page)
		const sidebar = page.locator('.gophenberg-editor__sidebar')
		await expect(sidebar).toBeVisible()

		await page.setViewportSize(PHONE)
		await expect(sidebar).toBeHidden()

		await page.setViewportSize({ width: 1280, height: 900 })
		await expect(sidebar).toBeVisible()
	})

	test('keeps settings closed by hand closed through a narrow window and back', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 900 })
		await openDraft(page)
		const sidebar = page.locator('.gophenberg-editor__sidebar')
		await page.getByRole('button', { name: 'Close Settings' }).click()
		await expect(sidebar).toBeHidden()
		await expect(settings(page)).toBeFocused()

		await page.setViewportSize(PHONE)
		await page.setViewportSize({ width: 1280, height: 900 })

		await expect(sidebar).toBeHidden()
		await expect(settings(page)).toHaveAttribute('aria-pressed', 'false')
	})

	test('slides the settings over 300ms once they move at a desktop width, and at once on a phone', async ({ page }) => {
		await page.setViewportSize(DESKTOP)
		await openDraft(page)
		const sidebar = page.locator(SIDEBAR)
		const duration = () => sidebar.evaluate((held) => getComputedStyle(held).transitionDuration)
		await page.getByRole('button', { name: 'Close Settings' }).click()
		await expect(sidebar).toBeHidden()

		expect(await duration()).toContain('0.3s')

		await page.setViewportSize(PHONE)

		await expect.poll(duration).toBe('0s')
	})

	test('paints settings open on load where they rest, with no slide', async ({ page }) => {
		await page.setViewportSize(DESKTOP)
		await recordSidebarTransitions(page)
		await openDraft(page)
		await expect(page.locator(SIDEBAR)).toBeVisible()

		expect(await sidebarTransitions(page)).toEqual([])
	})

	test('slides the settings out and in at a desktop width without widening the page', async ({ page }) => {
		await page.setViewportSize(DESKTOP)
		await openDraft(page)
		const resting = DESKTOP.width - SIDEBAR_WIDTH

		const closing = await watchMove(page.getByRole('button', { name: 'Close Settings' }), SIDEBAR)
		await expect(page.locator(SIDEBAR)).toBeHidden()
		const opening = await watchMove(settings(page), SIDEBAR)

		expect(caughtBetween(closing.lefts, resting, DESKTOP.width), 'caught sliding out').toBe(true)
		expect(caughtBetween(opening.lefts, resting, DESKTOP.width), 'caught sliding in').toBe(true)
		expect(opening.lefts.at(-1)).toBe(resting)
		expect([closing.widest, opening.widest]).toEqual([DESKTOP.width, DESKTOP.width])
	})

	test('centres the document name in the room the header groups leave at 1280', async ({ page }) => {
		await page.setViewportSize(DESKTOP)
		await openDraft(page)

		const pill = await edges(page.getByTestId('document-bar'))
		const tools = await edges(page.locator('.gophenberg-editor__tools'))
		const actions = await edges(page.locator('.gophenberg-editor__actions'))

		expect(Math.abs((pill.left + pill.right) / 2 - (tools.right + actions.left) / 2)).toBeLessThan(1)
	})

	test('keeps the header words on one line at 1280 while changes wait to be saved', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 900 })
		await openDraft(page)
		await page.getByRole('textbox', { name: 'Title' }).fill(`A draft whose changes wait ${RUN}`)
		const header = page.locator('.gophenberg-editor__header')
		const state = header.getByText('Unsaved changes', { exact: true })
		await expect(state).toBeVisible()

		const lines = await state.evaluate(
			(element) => element.getBoundingClientRect().height / Number.parseFloat(getComputedStyle(element).lineHeight),
		)
		const save = await box(page.getByRole('button', { name: 'Save draft' }))
		const publish = await box(page.getByRole('button', { name: 'Publish', exact: true }))

		expect(Math.round(lines)).toBe(1)
		expect(save.height).toBe(publish.height)
		expect(save.height).toBeLessThan((await box(header)).height)
	})

	test('keeps a long document name a header gap away from both header groups from 782 to 900', async ({ page }) => {
		await page.setViewportSize({ width: 782, height: 900 })
		await openDraft(page, LONG_TITLE)

		for (const width of [782, 800, 900]) {
			await page.setViewportSize({ width, height: 900 })
			const pill = await edges(page.getByTestId('document-bar'))
			const tools = await edges(page.locator('.gophenberg-editor__tools'))
			const actions = await edges(page.locator('.gophenberg-editor__actions'))

			expect(pill.left - tools.right, `the room before the name at ${width}`).toBeGreaterThanOrEqual(7.5)
			expect(actions.left - pill.right, `the room after the name at ${width}`).toBeGreaterThanOrEqual(7.5)
		}
	})
})

test.describe('under reduced motion', () => {
	test.use({ reducedMotion: 'reduce' })

	test('shows and hides the settings at once at a desktop width', async ({ page }) => {
		await page.setViewportSize(DESKTOP)
		await openDraft(page)
		const resting = DESKTOP.width - SIDEBAR_WIDTH

		const closing = await watchMove(page.getByRole('button', { name: 'Close Settings' }), SIDEBAR)
		await expect(page.locator(SIDEBAR)).toBeHidden()
		const opening = await watchMove(settings(page), SIDEBAR)

		expect(caughtBetween(closing.lefts, resting, DESKTOP.width), 'caught sliding out').toBe(false)
		expect(caughtBetween(opening.lefts, resting, DESKTOP.width), 'caught sliding in').toBe(false)
		expect(opening.lefts.at(-1)).toBe(resting)
	})

	test('opens the list view at once on a phone', async ({ page }) => {
		await page.setViewportSize(PHONE)
		await openDraft(page)

		const moved = await watchMove(page.getByRole('button', { name: 'List view' }), OUTLINE)

		expect(caughtBetween(moved.lefts, -PHONE.width, 0)).toBe(false)
		expect(moved.lefts.at(-1)).toBe(0)
	})
})
