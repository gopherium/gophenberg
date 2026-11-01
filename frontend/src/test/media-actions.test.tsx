// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses, setViewport, stubLayout } from '@gopherium/godmin/testing'
import { seedSession } from '@gopherium/react-auth/testing'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { bulkBar } from './bulkBar'
import { gate } from './gate'
import { expectUnmarked, holdReads, waitMarked } from './reloads'
import { adminUser, renderAt } from './render'
import { siteSettings } from './siteSettings'
import { clickDimmedPage, findToast, menuItems, waitBusy } from './typeFixtures'

beforeAll(async () => {
	await import('../media/MediaScreen')
})

const FOREIGN_AUTHOR = '019fb000-0000-7000-8000-0000000000ee'

/** The version the list read each file at. */
const LISTED_VERSION = '2026-08-12T10:00:00Z'

/** The version the server holds each file at once another write landed after the list was read. */
const CURRENT_VERSION = '2026-08-13T08:30:00Z'

/** An author, who may change only the files it uploaded. */
const AUTHOR = { ...adminUser, role: 'author', capabilities: [] }

const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor-at-dawn-from-the-pier.jpg',
	title: 'Harbor at dawn',
	alt_text: 'Boats at sunrise',
	caption: 'Before the market opens',
	description: 'From the eastern pier',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {},
	author_id: adminUser.id,
	author_name: 'Maria Perez',
	author_avatar: '',
	created_at: '2026-08-10T10:00:00Z',
	updated_at: LISTED_VERSION,
}

const MANUAL = {
	...HARBOR,
	id: 8,
	type: 'file',
	file: '2026/08/manual.pdf',
	title: 'Manual',
	alt_text: '',
	caption: '',
	description: '',
	mime_type: 'application/pdf',
	width: 0,
	height: 0,
	filesize: 12000,
}

/** The reason the server gives for a file the library no longer holds, in the reader's words. */
const NOT_FOUND = [
	'This media item is not in the library, and someone may have deleted it.',
	'Reload the page to see what is there now.',
].join(' ')

/** The cell a card shows the title of its file in, or anything inside it. */
const TITLE_CELL = '.dataviews-title-field, .dataviews-title-field *'

/** The files the server holds, by id, changed by every edit and delete it lets through. */
const stored = new Map<number, Record<string, unknown>>()

/** The body of every edit the server took. */
const patched: Record<string, unknown>[] = []

/** The id of every file whose version was read. */
const versionReads: number[] = []

/** The id of every file deleted. */
const deleted: number[] = []

/** How many times the library was listed. */
let listings = 0

beforeEach(() => {
	stored.clear()
	stored.set(HARBOR.id, { ...HARBOR })
	stored.set(MANUAL.id, { ...MANUAL })
	patched.length = 0
	versionReads.length = 0
	deleted.length = 0
	listings = 0
	server.use(
		http.get('/api/media', () => {
			listings += 1
			const items = [...stored.values()]
			return HttpResponse.json({ items, total: items.length, per_page: siteSettings.list_page_size })
		}),
		http.get('/api/media/:id', ({ params }) => {
			versionReads.push(Number(params.id))
			return HttpResponse.json({ ...stored.get(Number(params.id)), updated_at: CURRENT_VERSION })
		}),
		http.patch('/api/media/:id', async ({ params, request }) => kept(Number(params.id), await request.json())),
		http.delete('/api/media/:id', ({ params }) => {
			deleted.push(Number(params.id))
			stored.delete(Number(params.id))
			return new HttpResponse(null, { status: 204 })
		}),
		http.get('/api/authors', () => HttpResponse.json({ items: [{ id: adminUser.id, name: 'Maria Perez' }] })),
	)
})

/**
 * Stores an edit the server took and answers with the file as it now stands.
 * @param id - The file edited.
 * @param body - The body of the edit.
 * @returns The answer.
 */
function kept(id: number, body: unknown): Response {
	patched.push(body as Record<string, unknown>)
	const next = { ...stored.get(id), ...(body as Record<string, unknown>) }
	stored.set(id, next)
	return HttpResponse.json(next)
}

/**
 * Answers a request as the server answers for a file the library no longer holds.
 * @returns The refusal.
 */
function gone(): Response {
	return HttpResponse.json({ error: 'gone', code: 'media_not_found' }, { status: 404 })
}

/**
 * Refuses every edit until the returned switch lets them through, as a server that cannot find the file.
 * @returns The switch letting edits through.
 */
function refuseEdits(): { allow: () => void } {
	let refusing = true
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) =>
			refusing ? gone() : kept(Number(params.id), await request.json()),
		),
	)
	return { allow: () => (refusing = false) }
}

/**
 * Holds the first edit until the returned gate opens, then refuses it, letting every later edit through.
 * @returns The gate the first edit waits on and the count of edits sent so far.
 */
function holdFirstEditThenRefuse() {
	const first = gate()
	const sent = { writes: 0 }
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) => {
			sent.writes += 1
			if (sent.writes === 1) {
				await first.held
				return gone()
			}
			return kept(Number(params.id), await request.json())
		}),
	)
	return { first, sent }
}

/**
 * Returns how many milliseconds pass, so a write that does not wait its turn has the time to go out.
 * @returns A promise settling once the time has passed.
 */
function aMoment() {
	return act(() => new Promise((resolve) => setTimeout(resolve, 50)))
}

/** Finds the element of a card a dialog on its file opens from. */
type CardTrigger = (card: Element) => HTMLElement | null

/** Finds the button of a card opening its actions menu. */
const menuTrigger: CardTrigger = (card) => card.querySelector('[aria-haspopup="menu"]')

/** Finds the title of a card a click opens Edit from. */
const titleTrigger: CardTrigger = (card) => card.querySelector('.dataviews-title-field[role="button"]')

/** Finds the picture of a card a click opens Edit from. */
const pictureTrigger: CardTrigger = (card) => card.querySelector('.dataviews-view-grid__media[role="button"]')

/**
 * Returns the card of the grid holding the given title.
 * @param title - The title the card shows.
 * @returns The card.
 */
async function cardHolding(title: string): Promise<HTMLElement> {
	const shown = await screen.findByText(title, { selector: TITLE_CELL })
	return shown.closest('.dataviews-view-grid__card') as HTMLElement
}

/**
 * Returns the queries of the card of the grid holding the given title.
 * @param title - The title the card shows.
 * @returns The queries bound to the card.
 */
async function cardOf(title: string) {
	return within(await cardHolding(title))
}

/**
 * Opens the actions menu of the card holding the given title.
 * @param title - The title the card shows.
 */
async function openCardMenu(title: string) {
	const card = await cardOf(title)
	await userEvent.click(card.getByRole('button', { name: 'Actions' }))
	await screen.findByRole('menu')
}

/**
 * Picks an item of the actions menu of a card and returns the dialog it opens.
 * @param title - The title the card shows.
 * @param item - The words of the menu item.
 * @returns The dialog.
 */
async function openCardDialog(title: string, item: string): Promise<HTMLElement> {
	await openCardMenu(title)
	await userEvent.click(await screen.findByRole('menuitem', { name: item }))
	return screen.findByRole('dialog')
}

/**
 * Types new text into a box of an open dialog, in place of what it held.
 * @param dialog - The dialog.
 * @param label - The label of the box.
 * @param text - The new text.
 */
async function retype(dialog: HTMLElement, label: string, text: string) {
	const box = within(dialog).getByRole('textbox', { name: label })
	await userEvent.clear(box)
	await userEvent.type(box, text)
}

/**
 * Presses Save in an open dialog and waits for the dialog to close.
 * @param dialog - The dialog.
 */
async function save(dialog: HTMLElement) {
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

/**
 * Returns the value each box of an open Edit holds, by its label.
 * @param dialog - The Edit dialog.
 * @returns The values.
 */
function boxes(dialog: HTMLElement): Record<string, string> {
	const shown = within(dialog).getAllByRole('textbox') as HTMLInputElement[]
	return Object.fromEntries(shown.map((box) => [box.labels?.[0]?.textContent ?? '', box.value]))
}

/**
 * Returns the labels of the rows a reader only reads in an open Edit, in order.
 * @param dialog - The Edit dialog.
 * @returns The labels.
 */
function readOnlyRows(dialog: HTMLElement): (string | null)[] {
	return [...dialog.querySelectorAll('.dataforms-layouts-panel__field-label')].map((label) => label.textContent)
}

test('offers View, Edit…, Rename… and Permanently delete…, in that order, on a file one may change', async () => {
	renderAt('/media')

	await openCardMenu('Harbor at dawn')

	expect(menuItems()).toEqual(['View', 'Edit…', 'Rename…', 'Permanently delete…'])
})

test('offers an author only View on a file another account uploaded', async () => {
	stored.set(HARBOR.id, { ...HARBOR, author_id: FOREIGN_AUTHOR })
	renderAt('/media', AUTHOR)

	await openCardMenu('Harbor at dawn')

	expect(menuItems()).toEqual(['View'])
})

test('offers every action on a file another account uploaded once the session may change others work', async () => {
	stored.set(HARBOR.id, { ...HARBOR, author_id: FOREIGN_AUTHOR })
	const client = renderAt('/media', AUTHOR)
	await openCardMenu('Harbor at dawn')
	expect(menuItems()).toEqual(['View'])
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())

	act(() => seedSession(client, adminUser))
	await openCardMenu('Harbor at dawn')

	expect(menuItems()).toEqual(['View', 'Edit…', 'Rename…', 'Permanently delete…'])
})

test('opens the file in a new tab from View', async () => {
	const opened = vi.spyOn(window, 'open').mockImplementation(() => null)
	renderAt('/media')
	await openCardMenu('Manual')

	await userEvent.click(screen.getByRole('menuitem', { name: 'View' }))

	expect(opened).toHaveBeenCalledWith('/media/2026/08/manual.pdf', '_blank')
})

test('offers View as a button of its own on a phone row, opening the file in a new tab', async () => {
	setViewport({ width: 639 })
	const opened = vi.spyOn(window, 'open').mockImplementation(() => null)
	renderAt('/media')

	const views = await screen.findAllByRole('button', { name: 'View' })
	await userEvent.click(views[1])

	expect(views).toHaveLength(2)
	expect(opened).toHaveBeenCalledWith('/media/2026/08/manual.pdf', '_blank')
})

test('names View in the words the catalogue gives the verb', async () => {
	setLocaleData({ '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, 'verb\u0004View': ['Open'] }, 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/media')

	await openCardMenu('Harbor at dawn')

	expect(menuItems()[0]).toBe('Open')
})

test('edits in a medium dialog headed Edit media, the Title box focused', async () => {
	stubLayout()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	expect(dialog).toHaveClass('has-size-medium')
	expect(within(dialog).getByRole('heading', { name: 'Edit media' })).toBeInTheDocument()
	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Title' })).toHaveFocus())
})

test('lays out the four boxes a reader writes in, then the six rows a reader only reads', async () => {
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	expect(boxes(dialog)).toEqual({
		Title: 'Harbor at dawn',
		'Alt text': 'Boats at sunrise',
		Caption: 'Before the market opens',
		Description: 'From the eastern pier',
	})
	expect(readOnlyRows(dialog)).toEqual(['Date added', 'Author', 'File name', 'File type', 'File size', 'Dimensions'])
	const last = within(dialog).getByRole('textbox', { name: 'Description' })
	const firstRow = dialog.querySelector('.dataforms-layouts-panel__field-label') as HTMLElement
	expect(last.compareDocumentPosition(firstRow) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(within(dialog).queryByRole('button', { name: /^Edit / })).not.toBeInTheDocument()
})

test('writes the alt text and the caption in boxes two lines tall, the description five lines tall', async () => {
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	const tall = ['Alt text', 'Caption', 'Description'].map((label) => {
		const box = within(dialog).getByRole('textbox', { name: label })
		return [box.tagName, box.getAttribute('rows')]
	})
	expect(tall).toEqual([
		['TEXTAREA', '2'],
		['TEXTAREA', '2'],
		['TEXTAREA', '5'],
	])
})

test('shows a long file name whole in Edit', async () => {
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	expect(within(dialog).getByText('harbor-at-dawn-from-the-pier.jpg')).toBeInTheDocument()
})

test('leaves out the alt text and the dimensions of a file that is no picture', async () => {
	renderAt('/media')

	const dialog = await openCardDialog('Manual', 'Edit…')

	expect(Object.keys(boxes(dialog))).toEqual(['Title', 'Caption', 'Description'])
	expect(readOnlyRows(dialog)).toEqual(['Date added', 'Author', 'File name', 'File type', 'File size'])
})

test('keeps Save on as Edit opens, after a Cancel drawn as accent text, as WordPress draws a row edit', async () => {
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	const form = within(dialog).getByRole('textbox', { name: 'Title' }).closest('form') as HTMLElement
	expect([...form.classList]).toEqual(['godmin-modal-body'])
	const [cancel, submit, ...rest] = within(form).getAllByRole('button')
	expect([cancel.textContent, submit.textContent, rest]).toEqual(['Cancel', 'Save', []])
	expect([...cancel.classList]).toEqual(buttonClasses('minimal'))
	expect(submit).toHaveAttribute('type', 'submit')
	expect(submit).not.toHaveAttribute('aria-disabled', 'true')
})

test('scrolls the fields of Edit in a pane above Cancel and Save, as WordPress scrolls its media editor', async () => {
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	const title = within(dialog).getByRole('textbox', { name: 'Title' })
	const pane = title.closest('.gophenberg-media__edit-fields') as HTMLElement
	expect(within(pane).getByText('Dimensions')).toBeInTheDocument()
	expect(within(pane).queryAllByRole('button', { name: /^(Cancel|Save)$/ })).toEqual([])
	expect(getComputedStyle(pane).overflowY).toBe('auto')
})

/**
 * Makes the fields pane of Edit report fields taller than itself, as a short window lays them out, until the test ends.
 */
function overflowFields() {
	const pane = (element: Element) => element.classList.contains('gophenberg-media__edit-fields')
	const tall = vi.spyOn(Element.prototype, 'scrollHeight', 'get').mockImplementation(function (this: Element) {
		return pane(this) ? 900 : 0
	})
	const shown = vi.spyOn(Element.prototype, 'clientHeight', 'get').mockImplementation(function (this: Element) {
		return pane(this) ? 600 : 0
	})
	onTestFinished(() => {
		tall.mockRestore()
		shown.mockRestore()
	})
}

/**
 * Returns the pane the fields of an open Edit scroll in.
 * @param dialog - The Edit dialog.
 * @returns The pane.
 */
function fieldsPane(dialog: HTMLElement): HTMLElement {
	return within(dialog).getByRole('textbox', { name: 'Title' }).closest('.gophenberg-media__edit-fields') as HTMLElement
}

/**
 * Tells whether the header of an open dialog draws the line the Modal draws under it once its content scrolled.
 * @param dialog - The dialog.
 * @returns Whether the bottom border of the header takes the colour of a neutral surface stroke.
 */
function headerLine(dialog: HTMLElement): boolean {
	const header = dialog.querySelector('.components-modal__header') as HTMLElement
	return /^var\(--wpds-color-stroke-surface-neutral\b/.test(getComputedStyle(header).borderBottomColor)
}

test('names the fields pane of Edit Scrollable section while its fields overflow it, as the Modal does', async () => {
	overflowFields()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	const pane = within(dialog).getByRole('document', { name: 'Scrollable section' })
	expect(pane).toBe(fieldsPane(dialog))
	expect(pane).toHaveClass('is-scrollable')
})

test('focuses the Title box as Edit opens on overflowing fields, then makes their pane a tab stop', async () => {
	stubLayout()
	overflowFields()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Title' })).toHaveFocus())
	expect(fieldsPane(dialog)).toHaveAttribute('tabindex', '0')
})

test('leaves the Edit fields pane unnamed, out of the tab order, no line above, while its fields fit', async () => {
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	const pane = fieldsPane(dialog)
	expect(pane).not.toHaveAttribute('aria-label')
	expect(pane).not.toHaveAttribute('tabindex')
	expect(pane).not.toHaveClass('is-scrollable')
	expect(headerLine(dialog)).toBe(false)
})

test('draws the line under the Edit header once the fields pane scrolled, gone again back at the top', async () => {
	overflowFields()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	const pane = fieldsPane(dialog)
	expect(headerLine(dialog)).toBe(false)

	for (const { top, drawn } of [{ top: 40, drawn: true }, { top: 0, drawn: false }]) {
		Object.defineProperty(pane, 'scrollTop', { configurable: true, value: top })
		fireEvent.scroll(pane)
		expect(headerLine(dialog)).toBe(drawn)
	}
})

/**
 * Replaces ResizeObserver with one that records what it watches and what it stops, until the test ends.
 * @returns The callback each watched element reports to, and the callbacks disconnected.
 */
function watchResizes() {
	const watched = new Map<Element, ResizeObserverCallback>()
	const stopped: ResizeObserverCallback[] = []
	vi.stubGlobal(
		'ResizeObserver',
		class {
			seen: ResizeObserverCallback
			constructor(seen: ResizeObserverCallback) {
				this.seen = seen
			}
			observe(target: Element) {
				watched.set(target, this.seen)
			}
			unobserve() {}
			disconnect() {
				stopped.push(this.seen)
			}
		},
	)
	onTestFinished(() => {
		vi.unstubAllGlobals()
	})
	return { watched, stopped }
}

test('names the Edit fields pane once a shorter window makes it overflow, its fields the same size', async () => {
	const { watched } = watchResizes()
	stubLayout()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	const pane = fieldsPane(dialog)
	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Title' })).toHaveFocus())
	expect(pane).not.toHaveAttribute('aria-label')
	expect(watched.has(pane)).toBe(true)

	overflowFields()
	act(() => (watched.get(pane) as ResizeObserverCallback)([], {} as ResizeObserver))

	expect(pane).toHaveAttribute('aria-label', 'Scrollable section')
	expect(pane).toHaveAttribute('tabindex', '0')
})

test('names the Edit fields pane once its fields grow past it, watched as the Modal watches content', async () => {
	const { watched, stopped } = watchResizes()
	stubLayout()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	const pane = fieldsPane(dialog)
	const watching = [...watched].find(([target]) => target.parentElement === pane)
	const [content, seen] = watching as [Element, ResizeObserverCallback]
	const title = within(dialog).getByRole('textbox', { name: 'Title' })
	expect(content).toContainElement(title)
	await waitFor(() => expect(title).toHaveFocus())
	expect(pane).not.toHaveAttribute('aria-label')
	expect(pane).not.toHaveAttribute('tabindex')

	overflowFields()
	act(() => seen([], {} as ResizeObserver))

	expect(pane).toHaveAttribute('aria-label', 'Scrollable section')
	expect(pane).toHaveAttribute('tabindex', '0')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	expect(stopped).toContain(seen)
})

test('names the fields pane of Edit in the words the catalogue the Modal reads gives Scrollable section', async () => {
	setLocaleData({ 'Scrollable section': ['Scrolling fields'] })
	onTestFinished(() => resetLocaleData({}))
	overflowFields()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

	expect(fieldsPane(dialog)).toHaveAttribute('aria-label', 'Scrolling fields')
})

test('opens the same Edit from a click on the title of a card', async () => {
	stubLayout()
	renderAt('/media')
	const card = await cardOf('Harbor at dawn')

	await userEvent.click(card.getByText('Harbor at dawn'))

	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	expect(dialog).toHaveClass('has-size-medium')
	expect(boxes(dialog)).toMatchObject({ Title: 'Harbor at dawn', Caption: 'Before the market opens' })
	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Title' })).toHaveFocus())
})

test('closes the Edit opened from a title as Save is pressed, then says Media file updated.', async () => {
	renderAt('/media')
	await userEvent.click((await cardOf('Harbor at dawn')).getByText('Harbor at dawn'))
	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	await retype(dialog, 'Title', 'Harbor at noon')

	await save(dialog)

	expect(await findToast('Media file updated.')).toBeInTheDocument()
	expect(patched).toEqual([{ updated_at: CURRENT_VERSION, title: 'Harbor at noon' }])
})

test('closes the Edit opened from a title through its Cancel, sending nothing', async () => {
	renderAt('/media')
	await userEvent.click((await cardOf('Harbor at dawn')).getByText('Harbor at dawn'))
	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(patched).toEqual([])
})

test('closes Edit as Save is pressed, before the write answers, then says Media file updated.', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) => {
			const body = await request.json()
			await answer.held
			return kept(Number(params.id), body)
		}),
	)
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(dialog, 'Title', 'Harbor at noon')

	await save(dialog)

	expect(screen.queryByText('Media file updated.')).not.toBeInTheDocument()
	answer.release()
	expect(await findToast('Media file updated.')).toBeInTheDocument()
	expect(await cardOf('Harbor at noon')).toBeTruthy()
})

test('writes over the version read just before the write, sending only the fields the reader changed', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(dialog, 'Title', 'Harbor at noon')
	await retype(dialog, 'Caption', 'After the market closes')

	await save(dialog)

	await waitFor(() =>
		expect(patched).toEqual([
			{ updated_at: CURRENT_VERSION, title: 'Harbor at noon', caption: 'After the market closes' },
		]),
	)
	expect(versionReads).toEqual([HARBOR.id])
})

test('sends Edit from Enter in the Title box', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(dialog, 'Title', 'Harbor at noon')

	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(patched).toEqual([{ updated_at: CURRENT_VERSION, title: 'Harbor at noon' }]))
})

test.each([
	{ kind: 'a picture', title: 'Harbor at dawn', words: 'Could not save image.' },
	{ kind: 'a file that is no picture', title: 'Manual', words: 'Could not save media.' },
])('says $words with the reason in a toast when the edit of $kind fails', async ({ title, words }) => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog(title, 'Edit…')

	await save(dialog)

	expect(await findToast(`${words} ${NOT_FOUND}`)).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('says the server could not be reached when the network drops an edit', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch('/api/media/:id', () => HttpResponse.error()))
	renderAt('/media')
	const dialog = await openCardDialog('Manual', 'Edit…')

	await save(dialog)

	expect(await findToast('Could not save media. The server could not be reached, so nothing was saved.')).toBeTruthy()
})

test('says Media file updated as the write ends and marks the list until the rows come back', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(dialog, 'Title', 'Harbor at noon')
	holdReads('/api/media')

	await save(dialog)

	await findToast('Media file updated.')
	await waitMarked()
})

test('reads the list again after a failed edit, so the card holds the newest version', async () => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	const before = listings

	await save(dialog)

	await findToast(`Could not save image. ${NOT_FOUND}`)
	await waitFor(() => expect(listings).toBeGreaterThan(before))
})

test('marks nothing while the list reads again after a failed edit', async () => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	holdReads('/api/media')

	await save(dialog)

	await findToast(`Could not save image. ${NOT_FOUND}`)
	expectUnmarked()
})

test('offers the fields a failed edit left unsaved when Edit opens again, through a Cancel', async () => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(dialog, 'Title', 'Harbor at noon')
	await retype(dialog, 'Alt text', 'Boats at noon')
	await save(dialog)
	await findToast(`Could not save image. ${NOT_FOUND}`)
	const kept = await openCardDialog('Harbor at dawn', 'Edit…')
	await userEvent.click(within(kept).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	const again = await openCardDialog('Harbor at dawn', 'Edit…')

	expect(boxes(again)).toMatchObject({
		Title: 'Harbor at noon',
		'Alt text': 'Boats at noon',
		Caption: 'Before the market opens',
	})
})

test('offers the fields a failed edit left unsaved only on the file they were typed for', async () => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(dialog, 'Caption', 'After the market closes')
	await save(dialog)
	await findToast(`Could not save image. ${NOT_FOUND}`)

	const other = await openCardDialog('Manual', 'Edit…')

	expect(boxes(other)).toMatchObject({ Caption: '' })
})

test('opens Edit on what a later save stored, dropping what an earlier failed edit kept', async () => {
	const edits = refuseEdits()
	renderAt('/media')
	const failing = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(failing, 'Caption', 'A failed caption')
	await save(failing)
	await findToast(`Could not save image. ${NOT_FOUND}`)
	edits.allow()
	const retried = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(retried, 'Caption', 'A saved caption')
	await save(retried)
	await findToast('Media file updated.')

	const again = await openCardDialog('Harbor at dawn', 'Edit…')

	expect(boxes(again)).toMatchObject({ Caption: 'A saved caption' })
})

test('opens Edit on the saved fields when an earlier edit fails after a later one was sent', async () => {
	const { first, sent } = holdFirstEditThenRefuse()
	renderAt('/media')
	const failing = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(failing, 'Title', 'A failed title')
	await save(failing)
	await waitFor(() => expect(sent.writes).toBe(1))
	const later = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(later, 'Title', 'A saved title')
	await save(later)

	first.release()

	await findToast(`Could not save image. ${NOT_FOUND}`)
	await findToast('Media file updated.')
	const again = await openCardDialog('A saved title', 'Edit…')
	expect(boxes(again)).toMatchObject({ Title: 'A saved title' })
})

test('keeps nothing of an edit that fails after a later rename of the file was sent', async () => {
	const { first, sent } = holdFirstEditThenRefuse()
	renderAt('/media')
	const edit = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(edit, 'Alt text', 'Boats at noon')
	await save(edit)
	await waitFor(() => expect(sent.writes).toBe(1))
	const rename = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(rename, 'Name', 'Harbor renamed')
	await save(rename)

	first.release()

	await findToast(`Could not save image. ${NOT_FOUND}`)
	await findToast('Name updated.')
	const again = await openCardDialog('Harbor renamed', 'Edit…')
	expect(boxes(again)).toMatchObject({ Title: 'Harbor renamed', 'Alt text': 'Boats at sunrise' })
})

test('drops the title a failed edit kept once a rename is sent, keeping its alt text', async () => {
	const edits = refuseEdits()
	renderAt('/media')
	const edit = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(edit, 'Title', 'A failed title')
	await retype(edit, 'Alt text', 'Boats at noon')
	await save(edit)
	await findToast(`Could not save image. ${NOT_FOUND}`)
	edits.allow()
	const rename = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(rename, 'Name', 'Harbor renamed')
	await save(rename)
	await findToast('Name updated.')

	const again = await openCardDialog('Harbor renamed', 'Edit…')

	expect(boxes(again)).toMatchObject({ Title: 'Harbor renamed', 'Alt text': 'Boats at noon' })
})

test('drops the name a failed rename kept once an edit of the title is sent', async () => {
	const edits = refuseEdits()
	renderAt('/media')
	const rename = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(rename, 'Name', 'A failed name')
	await save(rename)
	await findToast(NOT_FOUND)
	edits.allow()
	const edit = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(edit, 'Title', 'Harbor at noon')
	await save(edit)
	await findToast('Media file updated.')

	const again = await openCardDialog('Harbor at noon', 'Rename…')

	expect(within(again).getByRole('textbox', { name: 'Name' })).toHaveValue('Harbor at noon')
})

test('keeps the name a failed rename kept when an edit leaves the title alone', async () => {
	const edits = refuseEdits()
	renderAt('/media')
	const rename = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(rename, 'Name', 'A failed name')
	await save(rename)
	await findToast(NOT_FOUND)
	edits.allow()
	const edit = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(edit, 'Caption', 'After the market closes')
	await save(edit)
	await findToast('Media file updated.')

	const again = await openCardDialog('Harbor at dawn', 'Rename…')

	expect(within(again).getByRole('textbox', { name: 'Name' })).toHaveValue('A failed name')
})

test('renames in a medium dialog headed Rename, its Name box holding the stored title and focused', async () => {
	stubLayout()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')

	expect(dialog).toHaveClass('has-size-medium')
	expect(within(dialog).getByRole('heading', { name: 'Rename' })).toBeInTheDocument()
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('Harbor at dawn')
	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveFocus())
})

test('keeps Save of Rename off while the Name is blank or spaces, after a Cancel drawn as accent text', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	const save = within(dialog).getByRole('button', { name: 'Save' })
	expect([...within(dialog).getByRole('button', { name: 'Cancel' }).classList]).toEqual(buttonClasses('minimal'))
	expect(save).not.toHaveAttribute('aria-disabled', 'true')

	await retype(dialog, 'Name', '   ')
	expect(save).toHaveAttribute('aria-disabled', 'true')
	await userEvent.clear(within(dialog).getByRole('textbox', { name: 'Name' }))

	expect(save).toHaveAttribute('aria-disabled', 'true')
})

test('holds an empty Name for a file with no title', async () => {
	stored.set(HARBOR.id, { ...HARBOR, title: '' })
	renderAt('/media')

	const dialog = await openCardDialog('(no title)', 'Rename…')

	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('')
})

test('closes Rename as Save is pressed, writes over the version read first, then says Name updated.', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(dialog, 'Name', 'Harbor renamed')

	await save(dialog)

	expect(await findToast('Name updated.')).toBeInTheDocument()
	expect(patched).toEqual([{ updated_at: CURRENT_VERSION, title: 'Harbor renamed' }])
	expect(versionReads).toEqual([HARBOR.id])
	expect(await cardOf('Harbor renamed')).toBeTruthy()
})

test('says Name updated as the write ends and marks the list until the rows come back', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(dialog, 'Name', 'Harbor renamed')
	holdReads('/api/media')

	await save(dialog)

	await findToast('Name updated.')
	await waitMarked()
})

test('shows the reason a refused rename gave in a toast and offers the typed name again', async () => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(dialog, 'Name', 'Harbor renamed')
	await save(dialog)

	expect(await findToast(NOT_FOUND)).toBeInTheDocument()
	const again = await openCardDialog('Harbor at dawn', 'Rename…')
	expect(within(again).getByRole('textbox', { name: 'Name' })).toHaveValue('Harbor renamed')
})

test('says the name could not be updated when the network drops a rename', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch('/api/media/:id', () => HttpResponse.error()))
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')

	await save(dialog)

	expect(await findToast('The name could not be updated.')).toBeInTheDocument()
})

test('asks before deleting a file for good under no header, naming it in quotes, Cancel focused', async () => {
	stubLayout()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…')

	expect(within(dialog).getByText('Delete "Harbor at dawn" for good? This cannot be undone.')).toBeInTheDocument()
	expect(within(dialog).queryByRole('heading')).not.toBeInTheDocument()
	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	expect(dialog).toHaveClass('has-size-medium')
	const [cancel, confirm] = within(dialog).getAllByRole('button')
	expect([cancel.textContent, confirm.textContent]).toEqual(['Cancel', 'Delete permanently'])
	expect([...cancel.classList]).toEqual(buttonClasses('minimal'))
	await waitFor(() => expect(cancel).toHaveFocus())
	expect(deleted).toEqual([])
})

test('names a file with no title as (no title) in the delete question', async () => {
	stored.set(HARBOR.id, { ...HARBOR, title: '' })
	renderAt('/media')

	const dialog = await openCardDialog('(no title)', 'Permanently delete…')

	expect(within(dialog).getByText('Delete "(no title)" for good? This cannot be undone.')).toBeInTheDocument()
})

test('keeps the confirm busy with Cancel greyed while the delete runs, then closes it and names the file', async () => {
	const answer = gate()
	server.use(
		http.delete('/api/media/:id', async ({ params }) => {
			await answer.held
			deleted.push(Number(params.id))
			stored.delete(Number(params.id))
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	await waitBusy(dialog, 'Delete permanently')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(await findToast('"Harbor at dawn" permanently deleted.')).toBeInTheDocument()
	expect(deleted).toEqual([HARBOR.id])
})

/**
 * Ticks both files of the library and opens the permanent delete of the bulk bar.
 * @returns The confirm dialog.
 */
async function deleteBothTicked(): Promise<HTMLElement> {
	await userEvent.click(await screen.findByRole('checkbox', { name: 'Harbor at dawn' }))
	await userEvent.click(screen.getByRole('checkbox', { name: 'Manual' }))
	await userEvent.click(bulkBar().getByRole('button', { name: 'Permanently delete…' }))
	return screen.findByRole('dialog')
}

test('deletes every ticked file behind one confirm and counts them', async () => {
	renderAt('/media')

	const dialog = await deleteBothTicked()
	expect(dialog).toHaveTextContent('Delete 2 items for good? This cannot be undone.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	expect(await findToast('2 items permanently deleted.')).toBeInTheDocument()
	expect(deleted.sort()).toEqual([HARBOR.id, MANUAL.id])
})

test('shows the reason a refused delete gave in a toast, the confirm closed and the card kept', async () => {
	server.use(http.delete('/api/media/:id', gone))
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	expect(await findToast(NOT_FOUND)).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(await cardOf('Harbor at dawn')).toBeTruthy()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('counts only the files a partly refused bulk delete could not remove, in one toast', async () => {
	server.use(
		http.delete('/api/media/:id', ({ params }) => {
			if (Number(params.id) === HARBOR.id) {
				return gone()
			}
			deleted.push(Number(params.id))
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/media')

	const dialog = await deleteBothTicked()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	expect(await findToast('1 item could not be permanently deleted.')).toBeInTheDocument()
	expect(deleted).toEqual([MANUAL.id])
	expect(document.querySelectorAll('.godmin-toast')).toHaveLength(1)
})

/**
 * Starts the held permanent delete of a file and closes its busy confirm on the dimmed page.
 * @param title - The title the card of the file shows.
 * @param answer - Builds the answer the delete gets once the gate opens.
 * @returns The gate the delete waits on.
 */
async function deleteThenLeave(title: string, answer: () => Response) {
	const held = gate()
	server.use(
		http.delete('/api/media/:id', async () => {
			await held.held
			return answer()
		}),
	)
	renderAt('/media')
	const confirm = await openCardDialog(title, 'Permanently delete…')
	await userEvent.click(within(confirm).getByRole('button', { name: 'Delete permanently' }))
	await waitBusy(confirm, 'Delete permanently')
	await clickDimmedPage()
	return held
}

test('leaves a dialog opened on the card after a delete dismissed while busy open once that delete ends', async () => {
	const held = await deleteThenLeave('Harbor at dawn', gone)
	const edit = await openCardDialog('Harbor at dawn', 'Edit…')

	held.release()
	await findToast(NOT_FOUND)
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Edit media' })).toBe(edit)
})

test('keeps the focus in an Edit opened after a dismissed delete once that delete lands', async () => {
	stubLayout()
	const held = await deleteThenLeave('Manual', () => {
		stored.delete(MANUAL.id)
		return new HttpResponse(null, { status: 204 })
	})
	const edit = await openCardDialog('Harbor at dawn', 'Edit…')
	const title = within(edit).getByRole('textbox', { name: 'Title' })
	await waitFor(() => expect(title).toHaveFocus())

	held.release()
	await findToast('"Manual" permanently deleted.')
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Edit media' })).toBe(edit)
	expect(title).toHaveFocus()
})

/**
 * Waits until the picture of the card now in the first place holds the focus.
 */
async function waitFocusOnManual() {
	const manual = await cardHolding('Manual')
	await waitFor(() => expect(pictureTrigger(manual)).toHaveFocus())
}

test('lands the focus on the card in its place once a deleted file has left the list', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	await findToast('"Harbor at dawn" permanently deleted.')
	await waitFocusOnManual()
})

/** Lists the files the server holds by title, as a library sorted by Title shows them. */
function listByTitle() {
	server.use(
		http.get('/api/media', () => {
			const items = [...stored.values()].sort((one, other) => String(one.title).localeCompare(String(other.title)))
			return HttpResponse.json({ items, total: items.length, per_page: siteSettings.list_page_size })
		}),
	)
}

/** A file another account uploaded while an edit was in flight, its title sorting before every other. */
const ANCHOR = { ...MANUAL, id: 9, file: '2026/08/anchor-chain.pdf', title: 'Anchor chain' }

/**
 * Opens Edit from a part of the card of Harbor at dawn, as a click on it does.
 * @param trigger - Finds the part clicked inside the card.
 * @returns The dialog.
 */
async function openFromHarbor(trigger: CardTrigger): Promise<HTMLElement> {
	await userEvent.click(trigger(await cardHolding('Harbor at dawn')) as HTMLElement)
	return screen.findByRole('dialog', { name: 'Edit media' })
}

test.each([
	{ from: 'Edit…', open: () => openCardDialog('Harbor at dawn', 'Edit…'), box: 'Title', toast: 'Media file updated.' },
	{ from: 'Rename…', open: () => openCardDialog('Harbor at dawn', 'Rename…'), box: 'Name', toast: 'Name updated.' },
	{ from: 'the title', open: () => openFromHarbor(titleTrigger), box: 'Title', toast: 'Media file updated.' },
	{ from: 'the picture', open: () => openFromHarbor(pictureTrigger), box: 'Title', toast: 'Media file updated.' },
])('lands the focus on the card in its place once the file saved from $from moved away', async ({
	open,
	box,
	toast,
}) => {
	listByTitle()
	renderAt('/media')
	const dialog = await open()
	await retype(dialog, box, 'Zebra crossing')

	await save(dialog)

	await findToast(toast)
	await cardOf('Zebra crossing')
	await waitFocusOnManual()
})

test.each([
	{ part: 'actions menu', trigger: menuTrigger, open: () => openCardDialog('Harbor at dawn', 'Edit…') },
	{ part: 'title', trigger: titleTrigger, open: () => openFromHarbor(titleTrigger) },
])('gives the focus back to the $part of a file whose edit failed, on its card where it moved', async ({
	trigger,
	open,
}) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	listByTitle()
	server.use(
		http.patch('/api/media/:id', () => {
			stored.set(ANCHOR.id, { ...ANCHOR })
			return HttpResponse.error()
		}),
	)
	renderAt('/media')

	await save(await open())

	await findToast('Could not save image. The server could not be reached, so nothing was saved.')
	const moved = await cardHolding('Harbor at dawn')
	await waitFor(() => expect(trigger(moved)).toHaveFocus())
	expect(document.querySelectorAll('.dataviews-view-grid__card')[1]).toBe(moved)
})

/** Drops a focus the list holds each time the library is read. */
function dropFocusWhileReading() {
	server.use(
		http.get('/api/media', () => {
			const focused = document.activeElement as HTMLElement
			if (focused.closest('.dataviews-view-list, .dataviews-view-grid') !== null) {
				focused.blur()
			}
		}),
	)
}

test('gives the focus back to the phone row Edit opened from, where it moved, once Save stored the file', async () => {
	setViewport({ width: 639 })
	listByTitle()
	dropFocusWhileReading()
	renderAt('/media')
	await userEvent.click(await screen.findByRole('button', { name: /^Harbor at dawn/ }))
	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	await retype(dialog, 'Title', 'Zebra crossing')

	await save(dialog)

	await findToast('Media file updated.')
	const row = await screen.findByRole('button', { name: /^Zebra crossing/ })
	await waitFor(() => expect(row).toHaveFocus())
	expect(document.querySelectorAll('.dataviews-view-list__item')[1]).toBe(row)
})

test('gives the focus back to the phone row Edit opened from once the save failed, the list read again', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	setViewport({ width: 639 })
	dropFocusWhileReading()
	server.use(http.patch('/api/media/:id', () => HttpResponse.error()))
	renderAt('/media')
	const row = await screen.findByRole('button', { name: /^Harbor at dawn/ })
	await userEvent.click(row)

	await save(await screen.findByRole('dialog', { name: 'Edit media' }))

	await findToast('Could not save image. The server could not be reached, so nothing was saved.')
	await waitFor(() => expect(row).toHaveFocus())
	expect(listings).toBeGreaterThan(1)
})

test('falls back to the actions menu once the trigger Edit opened from is gone, as a phone row in a grid', async () => {
	setViewport({ width: 639 })
	renderAt('/media')
	await userEvent.click(await screen.findByRole('button', { name: /^Harbor at dawn/ }))
	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	act(() => setViewport({ width: 1280 }))
	const card = await cardOf('Harbor at dawn')

	await save(dialog)

	await findToast('Media file updated.')
	await waitFor(() => expect(card.getByRole('button', { name: 'Actions' })).toHaveFocus())
})

test('lands the focus on the list once the file a save went to left it, as a delete does', async () => {
	server.use(
		http.get('/api/media', ({ request }) => {
			const search = new URL(request.url).searchParams.get('search') ?? ''
			const items = [...stored.values()].filter((file) => String(file.title).toLowerCase().includes(search))
			return HttpResponse.json({ items, total: items.length, per_page: siteSettings.list_page_size })
		}),
	)
	renderAt('/media?search=harbor')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(dialog, 'Name', 'Zebra crossing')

	await save(dialog)

	await findToast('Name updated.')
	await waitFor(() => expect(screen.getByRole('region', { name: 'Media' })).toHaveFocus())
})

test('leaves the focus in a dialog opened while a save of another file was still in flight', async () => {
	stubLayout()
	const answer = gate()
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) => {
			const body = await request.json()
			await answer.held
			return kept(Number(params.id), body)
		}),
	)
	renderAt('/media')
	await save(await openCardDialog('Harbor at dawn', 'Edit…'))
	const rename = await openCardDialog('Manual', 'Rename…')
	const name = within(rename).getByRole('textbox', { name: 'Name' })
	await waitFor(() => expect(name).toHaveFocus())

	answer.release()
	await findToast('Media file updated.')
	await aMoment()

	expect(name).toHaveFocus()
})

test('sends a delete of a file only once an edit of it still in flight has answered', async () => {
	const edit = gate()
	const heard: string[] = []
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) => {
			heard.push('edit sent')
			const body = await request.json()
			await edit.held
			heard.push('edit answered')
			return kept(Number(params.id), body)
		}),
		http.delete('/api/media/:id', () => {
			heard.push('delete sent')
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/media')
	await save(await openCardDialog('Harbor at dawn', 'Edit…'))
	await waitFor(() => expect(heard).toEqual(['edit sent']))

	const confirm = await openCardDialog('Harbor at dawn', 'Permanently delete…')
	await userEvent.click(within(confirm).getByRole('button', { name: 'Delete permanently' }))
	await aMoment()
	edit.release()

	await waitFor(() => expect(heard).toEqual(['edit sent', 'edit answered', 'delete sent']))
})

test('sends an edit of one file while an edit of another file is still in flight', async () => {
	const first = gate()
	const sent: number[] = []
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) => {
			sent.push(Number(params.id))
			const body = await request.json()
			await (Number(params.id) === HARBOR.id ? first.held : Promise.resolve())
			return kept(Number(params.id), body)
		}),
	)
	renderAt('/media')
	await save(await openCardDialog('Harbor at dawn', 'Edit…'))
	await waitFor(() => expect(sent).toEqual([HARBOR.id]))

	await save(await openCardDialog('Manual', 'Edit…'))

	await waitFor(() => expect(sent).toEqual([HARBOR.id, MANUAL.id]))
	first.release()
})

test('reads the version for a rename only once an edit of the file still in flight has answered', async () => {
	const edit = gate()
	const heard: string[] = []
	server.use(
		http.get('/api/media/:id', ({ params }) => {
			heard.push('version read')
			return HttpResponse.json({ ...stored.get(Number(params.id)), updated_at: CURRENT_VERSION })
		}),
		http.patch('/api/media/:id', async ({ params, request }) => {
			heard.push('write sent')
			const body = await request.json()
			await (heard.length === 2 ? edit.held : Promise.resolve())
			return kept(Number(params.id), body)
		}),
	)
	renderAt('/media')
	await save(await openCardDialog('Harbor at dawn', 'Edit…'))
	await waitFor(() => expect(heard).toEqual(['version read', 'write sent']))

	await save(await openCardDialog('Harbor at dawn', 'Rename…'))
	await aMoment()
	expect(heard).toEqual(['version read', 'write sent'])
	edit.release()

	await waitFor(() => expect(heard).toEqual(['version read', 'write sent', 'version read', 'write sent']))
})
