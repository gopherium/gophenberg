// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses, stubLayout } from '@gopherium/godmin/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { gate } from './gate'
import { expectUnmarked, holdReads, waitMarked } from './reloads'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'

const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor.jpg',
	title: 'Harbor at dawn',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {},
	created_at: '2026-08-10T10:00:00Z',
	updated_at: '2026-08-10T10:00:00Z',
}

/** The reason the server refuses a file of a type it does not take. */
const REFUSAL = 'the file type is not allowed'

/**
 * Returns a file of the given name, a text file when the name says so and a picture otherwise.
 * @param name - The file name.
 * @returns The file.
 */
function file(name: string): File {
	return new File(['bytes'], name, { type: name.endsWith('.txt') ? 'text/plain' : 'image/jpeg' })
}

/**
 * Counts the reads of the library and serves it with one picture.
 * @returns The count of reads so far.
 */
function listing(): { reads: number } {
	const seen = { reads: 0 }
	server.use(
		http.get('/api/media', () => {
			seen.reads += 1
			return HttpResponse.json({ items: [HARBOR], total: 1, per_page: siteSettings.list_page_size })
		}),
	)
	return seen
}

/**
 * Stores every upload but a text file, which it refuses, recording the type of each file sent.
 * @returns The types of the files sent, filled as requests arrive.
 */
function storing(): string[] {
	const sent: string[] = []
	server.use(
		http.post('/api/media', async ({ request }) => {
			const type = /Content-Type: (\S+)/.exec(await request.text())?.[1] ?? ''
			sent.push(type)
			if (type === 'text/plain') {
				return HttpResponse.json({ error: REFUSAL }, { status: 422 })
			}
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)
	return sent
}

/**
 * Hands the files to the hidden input behind Add Media File once the library shows.
 * @param files - The files to choose.
 */
async function choose(...files: File[]) {
	await screen.findByText('Harbor at dawn')
	await userEvent.upload(screen.getByLabelText('Add Media File'), files)
}

/**
 * Drops the files on the zone laid over the library, as a drag from the desktop would.
 * @param files - The files the drag carries.
 */
function drop(...files: File[]) {
	const zone = document.querySelector('.godmin-list-overlay') as HTMLElement
	const transfer = { files, items: [], types: ['Files'], getData: () => '' }
	document.dispatchEvent(Object.assign(new Event('dragenter', { bubbles: true }), { dataTransfer: transfer }))
	zone.dispatchEvent(Object.assign(new Event('drop', { bubbles: true, cancelable: true }), { dataTransfer: transfer }))
}

beforeEach(() => {
	listing()
})

test('offers Add Media File as the compact solid header button, drawn with the upload icon', async () => {
	renderAt('/media')

	const button = await within(await screen.findByRole('main')).findByRole('button', { name: 'Add Media File' })

	expect([...button.classList]).toEqual(buttonClasses('solid', 'compact'))
	expect(button.querySelector('svg')).not.toBeNull()
	expect(screen.queryByText('Add media')).not.toBeInTheDocument()
})

test('takes several files at once from the file dialog', async () => {
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	expect(screen.getByLabelText('Add Media File')).toHaveAttribute('multiple')
	expect(screen.getByLabelText('Add Media File')).not.toBeVisible()
})

test('sends one upload for each file chosen', async () => {
	const sent = storing()
	renderAt('/media')

	await choose(file('harbor.jpg'), file('notes.txt'))

	await waitFor(() => expect(sent.sort()).toEqual(['image/jpeg', 'text/plain']))
})

test('names the one file it stored in a toast', async () => {
	storing()
	renderAt('/media')

	await choose(file('harbor.jpg'))

	expect(await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('counts the files it stored in a toast when several were chosen', async () => {
	storing()
	renderAt('/media')

	await choose(file('harbor.jpg'), file('market.jpg'))

	expect(await screen.findByText('Uploaded 2 files.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('counts no refused file as uploaded, and names each refused file with its reason above the list', async () => {
	storing()
	renderAt('/media')

	await choose(file('harbor.jpg'), file('notes.txt'), file('market.jpg'))

	expect(await screen.findByText('Uploaded 2 files.', { selector: '.godmin-toast' })).toBeInTheDocument()
	const notice = await within(screen.getByRole('region', { name: 'Media' })).findByRole('alert')
	expect(notice).toHaveTextContent(`notes.txt: ${REFUSAL}`)
	expect(notice).not.toHaveTextContent('harbor.jpg')
})

test('reports the reason the one file chosen was refused, and raises no toast', async () => {
	storing()
	renderAt('/media')

	await choose(file('notes.txt'))

	const notice = await within(screen.getByRole('region', { name: 'Media' })).findByRole('alert')
	expect(notice).toHaveTextContent(REFUSAL)
	expect(notice).not.toHaveTextContent('notes.txt')
	expect(document.querySelector('.godmin-toast')).toBeNull()
})

test('leaves the focus off the library when no file chosen landed', async () => {
	storing()
	renderAt('/media')

	await choose(file('notes.txt'))

	await within(screen.getByRole('region', { name: 'Media' })).findByRole('alert')
	expect(screen.getByRole('region', { name: 'Media' })).not.toHaveFocus()
})

test('reports an upload that never reached the server', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/media', () => HttpResponse.error()))
	renderAt('/media')

	await choose(file('harbor.jpg'))

	expect(await screen.findByRole('alert')).toHaveTextContent('The server could not be reached, so nothing was uploaded.')
})

test('clears the last failure when the next upload starts', async () => {
	storing()
	renderAt('/media')
	await choose(file('notes.txt'))
	await screen.findByRole('alert')

	await choose(file('harbor.jpg'))

	await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('holds Add Media File busy while the files upload', async () => {
	const held = gate()
	server.use(
		http.post('/api/media', async () => {
			await held.held
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)
	renderAt('/media')

	await choose(file('harbor.jpg'))

	const button = screen.getByRole('button', { name: 'Add Media File' })
	await waitFor(() => expect(button).toHaveAttribute('aria-disabled', 'true'))
	held.release()
	await waitFor(() => expect(button).not.toHaveAttribute('aria-disabled', 'true'))
})

test('says a file landed as the upload ends and marks the list until the rows come back', async () => {
	storing()
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	holdReads('/api/media')

	await choose(file('harbor.jpg'))

	await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })
	await waitMarked()
})

test('ends the busy button before the list reloads', async () => {
	storing()
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	holdReads('/api/media')

	await choose(file('harbor.jpg'))

	await waitMarked()
	const button = screen.getByRole('button', { name: 'Add Media File' })
	await waitFor(() => expect(button).not.toHaveAttribute('aria-disabled', 'true'))
})

test('marks nothing when no file landed', async () => {
	storing()
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	const seen = listing()
	const before = seen.reads

	await choose(file('notes.txt'))

	expect(await screen.findByText(REFUSAL)).toBeInTheDocument()
	expectUnmarked()
	expect(seen.reads).toBe(before)
})

test('reloads the library and moves the focus to it once a file landed', async () => {
	const seen = listing()
	storing()
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	const before = seen.reads

	await choose(file('harbor.jpg'))

	await waitFor(() => expect(seen.reads).toBeGreaterThan(before))
	await waitFor(() => expect(screen.getByRole('region', { name: 'Media' })).toHaveFocus())
})

test('leaves the focus in a dialog opened while the upload ran once a file landed', async () => {
	stubLayout()
	const held = gate()
	server.use(
		http.post('/api/media', async () => {
			await held.held
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)
	renderAt('/media')
	await choose(file('harbor.jpg'))
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Edit…' }))
	const title = within(await screen.findByRole('dialog')).getByRole('textbox', { name: 'Title' })
	await waitFor(() => expect(title).toHaveFocus())

	held.release()
	await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(title).toHaveFocus()
})

const MARKET = { ...HARBOR, id: 9, file: '2026/08/market.jpg', title: 'Market at noon' }

/**
 * Shows the library while an upload holds back a file that lands before its one card.
 * @returns The gate the upload waits on.
 */
async function uploadHeldBack() {
	stubLayout()
	const held = gate()
	const listed = [HARBOR]
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({ items: listed, total: listed.length, per_page: siteSettings.list_page_size }),
		),
		http.post('/api/media', async () => {
			await held.held
			listed.unshift(MARKET)
			return HttpResponse.json(MARKET, { status: 201 })
		}),
	)
	renderAt('/media')
	await choose(file('market.jpg'))
	return held
}

/**
 * Opens a dialog from the menu of the one card while an upload holds back a file that lands before that card.
 * @param item - The words of the menu item.
 * @returns The dialog and the gate the upload waits on.
 */
async function openWhileUploading(item: string) {
	const held = await uploadHeldBack()
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: item }))
	return { dialog: await screen.findByRole('dialog'), held }
}

test.each([
	{ item: 'Edit…', box: 'Title' },
	{ item: 'Rename…', box: 'Name' },
])('keeps the $item a card menu opened, its typed text and its focus, once an upload lands a file before it', async ({
	item,
	box,
}) => {
	const { dialog, held } = await openWhileUploading(item)
	const typed = within(dialog).getByRole('textbox', { name: box })
	await waitFor(() => expect(typed).toHaveFocus())
	await userEvent.type(typed, ' in the fog')

	held.release()
	await screen.findByText('Market at noon')
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog')).toBe(dialog)
	expect(typed).toHaveValue('Harbor at dawn in the fog')
	expect(typed).toHaveFocus()
})

test('focuses the card menu an upload moved while its dialog was open, once Cancel closes the dialog', async () => {
	const { dialog, held } = await openWhileUploading('Rename…')
	held.release()
	await screen.findByText('Market at noon')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	const harbor = (await screen.findByText('Harbor at dawn')).closest('.dataviews-view-grid__card') as HTMLElement
	await waitFor(() => expect(within(harbor).getByRole('button', { name: 'Actions' })).toHaveFocus())
	expect(document.querySelectorAll('.dataviews-view-grid__card')[1]).toBe(harbor)
})

test('focuses the card title an upload moved while its Edit was open, once Cancel closes the dialog', async () => {
	const held = await uploadHeldBack()
	await userEvent.click(screen.getByText('Harbor at dawn'))
	const dialog = await screen.findByRole('dialog', { name: 'Edit media' })
	held.release()
	await screen.findByText('Market at noon')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	const harbor = (await screen.findByText('Harbor at dawn')).closest('.dataviews-view-grid__card') as HTMLElement
	await waitFor(() => expect(harbor.querySelector('.dataviews-title-field')).toHaveFocus())
	expect(document.querySelectorAll('.dataviews-view-grid__card')[1]).toBe(harbor)
})

test('uploads every file dropped on the library', async () => {
	const sent = storing()
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	drop(file('harbor.jpg'), file('market.jpg'))

	await waitFor(() => expect(sent).toEqual(['image/jpeg', 'image/jpeg']))
	expect(await screen.findByText('Uploaded 2 files.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('keeps the refusal of a drop made while an earlier upload still runs, naming the file once both end', async () => {
	const held = gate()
	let refused = 0
	server.use(
		http.post('/api/media', async ({ request }) => {
			if ((await request.text()).includes('Content-Type: text/plain')) {
				refused += 1
				return HttpResponse.json({ error: REFUSAL }, { status: 422 })
			}
			await held.held
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)
	renderAt('/media')
	await choose(file('harbor.jpg'))

	drop(file('notes.txt'))
	await waitFor(() => expect(refused).toBe(1))
	held.release()

	await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })
	const notice = await within(screen.getByRole('region', { name: 'Media' })).findByRole('alert')
	expect(notice).toHaveTextContent(`notes.txt: ${REFUSAL}`)
})

test('holds Add Media File busy until the last of two overlapping uploads ends', async () => {
	const first = gate()
	const second = gate()
	const holds = [first.held, second.held]
	server.use(
		http.post('/api/media', async () => {
			await holds.shift()
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)
	renderAt('/media')
	await choose(file('harbor.jpg'))
	const button = screen.getByRole('button', { name: 'Add Media File' })
	await waitFor(() => expect(button).toHaveAttribute('aria-disabled', 'true'))

	drop(file('market.jpg'))
	first.release()
	await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })

	expect(button).toHaveAttribute('aria-disabled', 'true')
	second.release()
	await waitFor(() => expect(button).not.toHaveAttribute('aria-disabled', 'true'))
})
