// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses, setViewport, stubLayout } from '@gopherium/godmin/testing'
import { seedSession } from '@gopherium/react-auth/testing'
import type { QueryClient } from '@tanstack/react-query'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { holdWrite } from '../content/postWrites'
import { bulkBar } from './bulkBar'
import { gate } from './gate'
import { expectUnmarked, holdReads, waitMarked } from './reloads'
import { adminUser, renderAt } from './render'
import { siteSettings } from './siteSettings'
import { findToast, menuItems, waitBusy } from './typeFixtures'

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
 * Refuses every edit, as a server that cannot find the file.
 */
function refuseEdits() {
	server.use(http.patch('/api/media/:id', gone))
}

/**
 * Holds a write to a file from outside every dialog until the returned gate opens, as a write still in flight.
 * @param client - The query client the library writes through.
 * @param id - The file the write goes to.
 * @returns The gate the write waits on.
 */
function holdFileWrite(client: QueryClient, id: number) {
	const write = gate()
	void holdWrite(client, `media ${id}`, write.held)
	return write
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
 * @param role - The role the dialog has, an alertdialog for a confirm.
 * @returns The dialog.
 */
async function openCardDialog(
	title: string,
	item: string,
	role: 'dialog' | 'alertdialog' = 'dialog',
): Promise<HTMLElement> {
	await openCardMenu(title)
	await userEvent.click(await screen.findByRole('menuitem', { name: item }))
	return screen.findByRole(role)
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
 * Presses Save in an open dialog.
 * @param dialog - The dialog.
 */
async function pressSave(dialog: HTMLElement) {
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
}

/**
 * Waits until no dialog shows.
 */
async function waitClosed() {
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

/**
 * Presses Save in an open dialog and waits for the dialog to close, as a write that works closes it.
 * @param dialog - The dialog.
 */
async function save(dialog: HTMLElement) {
	await pressSave(dialog)
	await waitClosed()
}

/**
 * Presses Cancel in an open dialog and waits for the dialog to close.
 * @param dialog - The dialog.
 */
async function closeByCancel(dialog: HTMLElement) {
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	await waitClosed()
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

test('edits in a dialog headed Edit media, the Title box focused', async () => {
	stubLayout()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')

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

test('opens the same Edit from a click on the title of a card', async () => {
	stubLayout()
	renderAt('/media')
	const card = await cardOf('Harbor at dawn')

	await userEvent.click(card.getByText('Harbor at dawn'))

	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	expect(boxes(dialog)).toMatchObject({ Title: 'Harbor at dawn', Caption: 'Before the market opens' })
	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Title' })).toHaveFocus())
})

test('keeps the Edit opened from a title on show while the save is sent, then says Media file updated.', async () => {
	const shownAtWrite: boolean[] = []
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) => {
			shownAtWrite.push(screen.queryByRole('dialog', { name: 'Edit media' }) !== null)
			return kept(Number(params.id), await request.json())
		}),
	)
	renderAt('/media')
	await userEvent.click((await cardOf('Harbor at dawn')).getByText('Harbor at dawn'))
	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	await retype(dialog, 'Title', 'Harbor at noon')

	await save(dialog)

	expect(await findToast('Media file updated.')).toBeInTheDocument()
	expect(shownAtWrite).toEqual([true])
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

test('keeps Edit busy with Cancel greyed until the write answers, then says Media file updated.', async () => {
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

	await pressSave(dialog)

	await waitBusy(dialog, 'Save')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
	expect(screen.getByRole('dialog', { name: 'Edit media' })).toBe(dialog)
	expect(screen.queryByText('Media file updated.')).not.toBeInTheDocument()
	answer.release()
	expect(await findToast('Media file updated.')).toBeInTheDocument()
	await waitClosed()
	expect(await cardOf('Harbor at noon')).toBeTruthy()
})

test('keeps the boxes of Edit in place and read only while the write runs, the focus staying in Title', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/media/:id', async () => {
			await answer.held
			return gone()
		}),
	)
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	await retype(dialog, 'Caption', 'After the market closes')
	const held = ['Title', 'Alt text', 'Caption', 'Description'].map((label) =>
		within(dialog).getByRole('textbox', { name: label }),
	)
	await userEvent.click(held[0])

	await userEvent.keyboard('{Enter}')

	await waitBusy(dialog, 'Save')
	expect(held.map((box) => box.hasAttribute('readonly'))).toEqual([true, true, true, true])
	expect(held.map((box) => box.isConnected)).toEqual([true, true, true, true])
	expect(held[0]).toHaveFocus()
	answer.release()
	expect(await within(dialog).findByText(`Could not save image. ${NOT_FOUND}`)).toBeInTheDocument()
	expect(held.map((box) => box.hasAttribute('readonly'))).toEqual([false, false, false, false])
	expect(held[0]).toHaveFocus()
	expect(boxes(dialog)).toMatchObject({ Title: 'Harbor at dawn', Caption: 'After the market closes' })
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
])('says $words with the reason inside Edit when the edit of $kind fails, the text typed kept', async ({
	title,
	words,
}) => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog(title, 'Edit…')
	await retype(dialog, 'Caption', 'After the market closes')

	await pressSave(dialog)

	expect(await within(dialog).findByText(`${words} ${NOT_FOUND}`)).toBeInTheDocument()
	expect(screen.getByRole('dialog', { name: 'Edit media' })).toBe(dialog)
	expect(boxes(dialog)).toMatchObject({ Caption: 'After the market closes' })
	expect(screen.queryByText(`${words} ${NOT_FOUND}`, { selector: '.godmin-toast' })).not.toBeInTheDocument()
})

test('says inside Edit the server could not be reached when the network drops an edit', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch('/api/media/:id', () => HttpResponse.error()))
	renderAt('/media')
	const dialog = await openCardDialog('Manual', 'Edit…')

	await pressSave(dialog)

	const words = 'Could not save media. The server could not be reached, so nothing was saved.'
	expect(await within(dialog).findByText(words)).toBeInTheDocument()
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

	await pressSave(dialog)

	await within(dialog).findByText(`Could not save image. ${NOT_FOUND}`)
	await waitFor(() => expect(listings).toBeGreaterThan(before))
})

test('marks nothing while the list reads again after a failed edit', async () => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Edit…')
	holdReads('/api/media')

	await pressSave(dialog)

	await within(dialog).findByText(`Could not save image. ${NOT_FOUND}`)
	expectUnmarked()
})

test('renames in a dialog headed Rename, its Name box holding the stored title and focused', async () => {
	stubLayout()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')

	expect(within(dialog).getByRole('heading', { name: 'Rename' })).toBeInTheDocument()
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('Harbor at dawn')
	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveFocus())
})

test('keeps Save of Rename on and sends nothing from it while the Name is blank or spaces', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	const submit = within(dialog).getByRole('button', { name: 'Save' })
	const field = within(dialog).getByRole('textbox', { name: 'Name' })
	expect([...within(dialog).getByRole('button', { name: 'Cancel' }).classList]).toEqual(
		buttonClasses('outline', undefined, 'neutral'),
	)

	await userEvent.clear(field)
	expect(submit).not.toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(submit)
	await userEvent.type(field, '   ')
	expect(submit).not.toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(submit)

	await aMoment()
	expect(patched).toEqual([])
	expect(screen.getByRole('dialog', { name: 'Rename' })).toBe(dialog)
})

test('holds an empty Name for a file with no title', async () => {
	stored.set(HARBOR.id, { ...HARBOR, title: '' })
	renderAt('/media')

	const dialog = await openCardDialog('(no title)', 'Rename…')

	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('')
})

test('keeps Rename busy with the Name read only until the write answers, then says Name updated.', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/media/:id', async ({ params, request }) => {
			const body = await request.json()
			await answer.held
			return kept(Number(params.id), body)
		}),
	)
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(dialog, 'Name', 'Harbor renamed')

	await pressSave(dialog)

	await waitBusy(dialog, 'Save')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveAttribute('readonly')
	expect(screen.getByRole('dialog', { name: 'Rename' })).toBe(dialog)
	answer.release()
	expect(await findToast('Name updated.')).toBeInTheDocument()
	await waitClosed()
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

test('shows the reason a refused rename gave inside Rename, the name typed kept and no toast raised', async () => {
	refuseEdits()
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(dialog, 'Name', 'Harbor renamed')

	await pressSave(dialog)

	expect(await within(dialog).findByText(NOT_FOUND)).toBeInTheDocument()
	expect(screen.getByRole('dialog', { name: 'Rename' })).toBe(dialog)
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('Harbor renamed')
	expect(screen.queryByText(NOT_FOUND, { selector: '.godmin-toast' })).not.toBeInTheDocument()
})

test('says inside Rename the name could not be updated when the network drops a rename, the name kept', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch('/api/media/:id', () => HttpResponse.error()))
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')
	await retype(dialog, 'Name', 'Harbor renamed')

	await pressSave(dialog)

	expect(await within(dialog).findByText('The name could not be updated.')).toBeInTheDocument()
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('Harbor renamed')
})

test('asks before deleting a file for good in a red confirm titled by the question, Cancel focused', async () => {
	stubLayout()
	renderAt('/media')

	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…', 'alertdialog')

	expect(within(dialog).getByRole('heading', { name: 'Delete "Harbor at dawn"?' })).toBeInTheDocument()
	expect(dialog).toHaveAccessibleDescription('This action cannot be undone.')
	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	const [cancel, confirm] = within(dialog).getAllByRole('button')
	expect([cancel.textContent, confirm.textContent]).toEqual(['Cancel', 'Delete permanently'])
	expect(confirm.className).toMatch(/irreversible/)
	await waitFor(() => expect(cancel).toHaveFocus())
	expect(deleted).toEqual([])
})

test('names a file with no title as (no title) in the delete question', async () => {
	stored.set(HARBOR.id, { ...HARBOR, title: '' })
	renderAt('/media')

	const dialog = await openCardDialog('(no title)', 'Permanently delete…', 'alertdialog')

	expect(within(dialog).getByRole('heading', { name: 'Delete "(no title)"?' })).toBeInTheDocument()
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
	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…', 'alertdialog')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	await waitBusy(dialog, 'Delete permanently')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
	answer.release()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
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
	return screen.findByRole('alertdialog')
}

test('deletes every ticked file behind one confirm and counts them', async () => {
	renderAt('/media')

	const dialog = await deleteBothTicked()
	expect(dialog).toHaveAccessibleName('Delete 2 items?')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	expect(await findToast('2 items permanently deleted.')).toBeInTheDocument()
	expect(deleted.sort()).toEqual([HARBOR.id, MANUAL.id])
})

test('shows the reason a refused delete gave in a toast, the confirm closed and the card kept', async () => {
	server.use(http.delete('/api/media/:id', gone))
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…', 'alertdialog')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete permanently' }))

	expect(await findToast(NOT_FOUND)).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
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
 * Waits until the picture of the card now in the first place holds the focus.
 */
async function waitFocusOnManual() {
	const manual = await cardHolding('Manual')
	await waitFor(() => expect(pictureTrigger(manual)).toHaveFocus())
}

test('lands the focus on the card in its place once a deleted file has left the list', async () => {
	renderAt('/media')
	const dialog = await openCardDialog('Harbor at dawn', 'Permanently delete…', 'alertdialog')

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
	{
		from: 'Edit…',
		trigger: menuTrigger,
		open: () => openCardDialog('Harbor at dawn', 'Edit…'),
		box: 'Title',
		toast: 'Media file updated.',
	},
	{
		from: 'Rename…',
		trigger: menuTrigger,
		open: () => openCardDialog('Harbor at dawn', 'Rename…'),
		box: 'Name',
		toast: 'Name updated.',
	},
	{
		from: 'the title',
		trigger: titleTrigger,
		open: () => openFromHarbor(titleTrigger),
		box: 'Title',
		toast: 'Media file updated.',
	},
	{
		from: 'the picture',
		trigger: pictureTrigger,
		open: () => openFromHarbor(pictureTrigger),
		box: 'Title',
		toast: 'Media file updated.',
	},
])('gives the focus back to the file saved from $from, on its card where it moved', async ({
	trigger,
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
	const moved = await cardHolding('Zebra crossing')
	await waitFor(() => expect(trigger(moved)).toHaveFocus())
	expect(document.querySelectorAll('.dataviews-view-grid__card')[1]).toBe(moved)
})

test.each([
	{ part: 'actions menu', trigger: menuTrigger, open: () => openCardDialog('Harbor at dawn', 'Edit…') },
	{ part: 'title', trigger: titleTrigger, open: () => openFromHarbor(titleTrigger) },
])('gives the focus back to the $part once a failed edit is cancelled, on its card where it moved', async ({
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
	const dialog = await open()
	await pressSave(dialog)
	await within(dialog).findByText('Could not save image. The server could not be reached, so nothing was saved.')

	await closeByCancel(dialog)

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

test('gives the focus back to the phone row of a failed Edit once it is cancelled, the list read again', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	setViewport({ width: 639 })
	dropFocusWhileReading()
	server.use(http.patch('/api/media/:id', () => HttpResponse.error()))
	renderAt('/media')
	const row = await screen.findByRole('button', { name: /^Harbor at dawn/ })
	await userEvent.click(row)
	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	await pressSave(dialog)
	await within(dialog).findByText('Could not save image. The server could not be reached, so nothing was saved.')

	await closeByCancel(dialog)

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

test('sends a delete of a file only once a write to it still in flight has answered', async () => {
	const heard: string[] = []
	server.use(
		http.delete('/api/media/:id', () => {
			heard.push('delete sent')
			return new HttpResponse(null, { status: 204 })
		}),
	)
	const client = renderAt('/media')
	const write = holdFileWrite(client, HARBOR.id)
	const confirm = await openCardDialog('Harbor at dawn', 'Permanently delete…', 'alertdialog')

	await userEvent.click(within(confirm).getByRole('button', { name: 'Delete permanently' }))
	await aMoment()
	expect(heard).toEqual([])
	write.release()

	await waitFor(() => expect(heard).toEqual(['delete sent']))
})

test('sends an edit of one file while a write to another file is still in flight', async () => {
	const client = renderAt('/media')
	const write = holdFileWrite(client, HARBOR.id)

	await save(await openCardDialog('Manual', 'Edit…'))

	expect(await findToast('Media file updated.')).toBeInTheDocument()
	expect(versionReads).toEqual([MANUAL.id])
	write.release()
})

test('reads the version for a rename only once a write to the file still in flight has answered', async () => {
	const client = renderAt('/media')
	const write = holdFileWrite(client, HARBOR.id)
	const dialog = await openCardDialog('Harbor at dawn', 'Rename…')

	await pressSave(dialog)
	await aMoment()
	expect(versionReads).toEqual([])
	write.release()

	await waitFor(() => expect(versionReads).toEqual([HARBOR.id]))
	await findToast('Name updated.')
	await waitClosed()
})
