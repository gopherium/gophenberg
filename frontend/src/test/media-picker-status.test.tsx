// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { listenAt } from './announcements'
import { gate } from './gate'
import {
	card,
	choose,
	drop,
	frame,
	HARBOR,
	HOLLOW,
	listing,
	NOTES,
	openPicker,
	PHOTO,
	REASON,
	SECOND,
	stored,
	uploadsAnswering,
} from './mediaPicker'

/** A block that takes pictures alone. */
const IMAGE_BLOCK = { allowedTypes: ['image'] }

/** The line a text file refused by an image block reads as. */
const NOTES_LINE = 'notes.txt: notes.txt: Sorry, this file type is not supported here.'

/** The line an empty file reads as. */
const HOLLOW_LINE = 'hollow.jpg: hollow.jpg: This file is empty.'

/** The name of the panel listing what went wrong. */
const ERRORS = { name: 'Upload errors' }

/** The name of the panel with nothing left running or wrong. */
const COMPLETE = { name: 'Upload complete' }

/**
 * Opens the picker on its first page.
 * @param props - The props the block editor would pass.
 */
async function openOnCards(props: Record<string, unknown> = {}) {
	await openPicker(props)
	await card(HARBOR)
}

/**
 * Holds every upload until the gate opens, then answers it as the server does.
 * @returns The gate and the names of the files sent so far.
 */
function heldUploads() {
	const held = gate()
	const sent = uploadsAnswering(async (file) => {
		await held.held
		return stored(file)
	})
	return { held, sent }
}

/**
 * Returns the wrapper around the foot of the list.
 * @returns The wrapper.
 */
function footer(): HTMLElement {
	return document.querySelector('.gophenberg-media-picker__footer') as HTMLElement
}

/**
 * Returns the spinner beside the trigger, null while none shows.
 * @returns The spinner.
 */
function spinner(): Element | null {
	return document.querySelector('.gophenberg-media-picker__status > svg')
}

/**
 * Returns the row of a panel showing a file.
 * @param file - The file.
 * @returns The list item.
 */
function rowOf(file: File): HTMLElement {
	return screen.getByText(file.name).closest('li') as HTMLElement
}

/**
 * Lets whatever is running settle.
 * @returns A promise settling once the time has passed.
 */
function settled() {
	return act(() => new Promise((resolve) => setTimeout(resolve, 50)))
}

/**
 * Picks a text file an image block refuses, and waits for its panel.
 * @returns The panel.
 */
async function refusedPick() {
	await choose(NOTES)
	return screen.findByRole('dialog', ERRORS)
}

/**
 * Closes the panel by Escape.
 */
async function escapePanel() {
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog', ERRORS)).not.toBeInTheDocument())
}

/**
 * Closes the frame by its own close button.
 */
async function closeFrame() {
	await userEvent.click(within(frame()).getByRole('button', { name: 'Close' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

/**
 * Presses the trigger that opens the picker again.
 */
async function reopen() {
	await userEvent.click(await screen.findByRole('button', { name: 'Media Library' }))
}

/**
 * Picks a picture held on its way to the server, and opens the panel by the trigger.
 * @returns The gate holding the upload, and the panel.
 */
async function panelOverHeldPick() {
	const { held } = heldUploads()
	await openOnCards()
	await choose(PHOTO)
	await userEvent.click(await screen.findByRole('button', { name: 'Uploading 1 file' }))
	return { held, panel: await screen.findByRole('dialog', { name: 'Uploading' }) }
}

beforeEach(() => {
	listing()
})

test('counts the files uploading', async () => {
	heldUploads()
	await openOnCards()

	await choose(PHOTO, SECOND)

	expect(await screen.findByRole('button', { name: 'Uploading 2 files' })).toBeInTheDocument()
})

test('keeps counting a stored file until its batch ends, and counts one fewer as soon as a file fails', async () => {
	const refused = gate()
	const last = gate()
	const sent = uploadsAnswering(async (file) => {
		await { [NOTES.name]: refused.held, [SECOND.name]: last.held }[file.name]
		return stored(file)
	})
	await openOnCards()

	await choose(PHOTO, SECOND, NOTES)
	await waitFor(() => expect(sent).toHaveLength(3))
	await settled()

	expect(screen.getByRole('button', { name: 'Uploading 3 files' })).toBeInTheDocument()
	refused.release()
	expect(await screen.findByRole('button', { name: 'Uploading 2 files' })).toBeInTheDocument()
	last.release()
	expect(await screen.findByRole('button', { name: '1 upload error' })).toBeInTheDocument()
})

test('reads Upload complete with its panel left open once the run ends', async () => {
	const { held, panel } = await panelOverHeldPick()

	held.release()

	expect(await screen.findByRole('button', COMPLETE)).toBeInTheDocument()
	expect(panel).toHaveAccessibleName(COMPLETE.name)
})

test('hides the item count while a row is listed, a stuck error too, and shows it once none is', async () => {
	const { held } = heldUploads()
	await openOnCards()
	const count = footer().querySelector('.dataviews-picker-footer__bulk-selection') as HTMLElement
	expect(footer()).not.toHaveClass('is-uploading')
	expect(count).toBeVisible()

	await choose(PHOTO)
	await screen.findByRole('button', { name: 'Uploading 1 file' })
	expect(footer()).toHaveClass('is-uploading')
	expect(count).not.toBeVisible()

	held.release()
	await waitFor(() => expect(footer()).not.toHaveClass('is-uploading'))
	expect(count).toBeVisible()

	drop(HOLLOW)
	await screen.findByRole('button', { name: '1 upload error' })
	expect(footer()).toHaveClass('is-uploading')
	expect(count).not.toBeVisible()
})

test('shows the spinner only while a file uploads', async () => {
	const { held } = heldUploads()
	await openOnCards()
	expect(spinner()).toBeNull()

	await choose(PHOTO, HOLLOW)
	await screen.findByRole('button', { name: 'Uploading 1 file' })
	expect(spinner()).not.toBeNull()

	held.release()
	await screen.findByRole('button', { name: '1 upload error' })
	expect(spinner()).toBeNull()
})

test('opens its panel by itself on the first error, the focus on the panel, whose title names it', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)

	const panel = await refusedPick()

	await waitFor(() => expect(panel).toHaveFocus())
	expect(within(panel).getByRole('heading', { level: 3, name: ERRORS.name })).toBeInTheDocument()
})

test('reopens no closed panel on a later error', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)
	await refusedPick()
	await escapePanel()

	await choose(HOLLOW)

	expect(await screen.findByRole('button', { name: '2 upload errors' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog', ERRORS)).not.toBeInTheDocument()
})

test('lists a spinner while uploading and a check once stored, with the name, its title and a heading', async () => {
	const { held, panel } = await panelOverHeldPick()
	expect(within(panel).getByRole('heading', { level: 3, name: 'Uploading' })).toBeInTheDocument()
	expect(rowOf(PHOTO).querySelector('svg')).toHaveAttribute('viewBox', '0 0 100 100')
	expect(screen.getByText(PHOTO.name)).toHaveAttribute('title', PHOTO.name)

	held.release()

	expect(await within(panel).findByRole('heading', { level: 3, name: COMPLETE.name })).toBeInTheDocument()
	expect(rowOf(PHOTO).querySelector('svg')).toHaveAttribute('viewBox', '0 0 24 24')
	expect(screen.getByText(PHOTO.name)).toHaveAttribute('title', PHOTO.name)
})

test('takes a row away by its close button, the dialog staying and the focus moving to the panel', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)
	const panel = await refusedPick()
	drop(HOLLOW)
	await screen.findByRole('button', { name: '2 upload errors' })

	const row = screen.getByText(NOTES_LINE).closest('li') as HTMLElement
	await userEvent.click(within(row).getByRole('button', { name: 'Close' }))

	await waitFor(() => expect(screen.queryByText(NOTES_LINE)).not.toBeInTheDocument())
	expect(screen.getByText(HOLLOW_LINE)).toBeInTheDocument()
	expect(frame()).toBeInTheDocument()
	expect(panel).toHaveFocus()
})

test('takes the status away with the last row, the focus on the frame and the next run opening no panel', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)
	const panel = await refusedPick()

	await userEvent.click(within(panel).getByRole('button', { name: 'Close' }))

	await waitFor(() => expect(footer()).not.toHaveClass('is-uploading'))
	expect(screen.queryByRole('dialog', ERRORS)).not.toBeInTheDocument()
	expect(frame()).toHaveFocus()
	await choose(PHOTO)
	await waitFor(() => expect(document.querySelector('.godmin-toast')).not.toBeNull())
	expect(screen.getAllByRole('dialog')).toEqual([frame()])
})

test('closes the panel first on Escape, the focus back where it was, and the dialog on a second Escape', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)
	await refusedPick()

	await escapePanel()

	expect(frame()).toBeInTheDocument()
	await waitFor(() => expect(within(frame()).getByRole('button', { name: 'Upload media' })).toHaveFocus())
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('puts the focus back on the search box when Escape closes a panel a drop opened from there', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)
	const search = screen.getByRole('searchbox', { name: 'Search media…' })
	await userEvent.click(search)

	drop(NOTES)
	await screen.findByRole('dialog', ERRORS)
	await escapePanel()

	await waitFor(() => expect(search).toHaveFocus())
})

test('puts the focus back on the trigger when Escape closes a panel the trigger opened', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)
	await refusedPick()
	await escapePanel()
	const trigger = screen.getByRole('button', { name: '1 upload error' })
	await userEvent.click(trigger)
	await screen.findByRole('dialog', ERRORS)

	await escapePanel()

	await waitFor(() => expect(trigger).toHaveFocus())
	expect(trigger).toHaveAttribute('aria-expanded', 'false')
})

test('keeps finished rows while the panel is open, and clears them on its close, the focus on the frame', async () => {
	const { held } = await panelOverHeldPick()
	held.release()
	await screen.findByRole('button', COMPLETE)
	await settled()
	expect(screen.getByText(PHOTO.name)).toBeInTheDocument()

	await userEvent.keyboard('{Escape}')

	await waitFor(() => expect(footer()).not.toHaveClass('is-uploading'))
	expect(screen.queryByText(PHOTO.name)).not.toBeInTheDocument()
	await settled()
	expect(frame()).toHaveFocus()
})

test('clears finished rows when the dialog closes with the panel open', async () => {
	const { held } = await panelOverHeldPick()
	held.release()
	await screen.findByRole('button', COMPLETE)

	await closeFrame()
	await reopen()
	await card(HARBOR)

	expect(footer()).not.toHaveClass('is-uploading')
})

test('speaks an error once at the assertive level, a later one once, and neither again on an open', async () => {
	const assertive = listenAt('assertive')
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)

	await refusedPick()
	await waitFor(() => expect(assertive()).toEqual([NOTES_LINE]))

	await escapePanel()
	drop(HOLLOW)
	await screen.findByRole('button', { name: '2 upload errors' })
	await waitFor(() => expect(assertive()).toEqual([NOTES_LINE, HOLLOW_LINE]))

	await userEvent.click(screen.getByRole('button', { name: '2 upload errors' }))
	await screen.findByRole('dialog', ERRORS)
	await settled()
	expect(assertive()).toEqual([NOTES_LINE, HOLLOW_LINE])
})

test('draws the status under a list it could not read, a refused drop listed and spoken', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))
	const assertive = listenAt('assertive')
	await openPicker(IMAGE_BLOCK)
	const notice = await screen.findByRole('alert')

	drop(NOTES)

	expect(await screen.findByRole('button', { name: '1 upload error' })).toBeInTheDocument()
	expect(footer()).toHaveClass('is-alone')
	expect(notice).toBeInTheDocument()
	const panel = await screen.findByRole('dialog', ERRORS)
	expect(within(panel).getByText(NOTES_LINE)).toBeInTheDocument()
	expect(assertive()).toEqual([NOTES_LINE])
})

test('draws no status under a list it could not read while no row is listed', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))

	await openPicker()
	await screen.findByRole('alert')

	expect(footer()).toBeNull()
})

test('opens on an error raised while closed, the panel focused and unspoken, Escape staying in the frame', async () => {
	const assertive = listenAt('assertive')
	const held = gate()
	uploadsAnswering(async () => {
		await held.held
		return HttpResponse.json({ error: REASON }, { status: 422 })
	})
	await openOnCards()
	await choose(PHOTO)
	await screen.findByRole('button', { name: 'Uploading 1 file' })
	await closeFrame()

	held.release()
	await waitFor(() => expect(assertive()).toEqual([`harbor.jpg: ${REASON}`]))
	await reopen()

	expect(await screen.findByRole('button', { name: '1 upload error' })).toBeInTheDocument()
	const panel = await screen.findByRole('dialog', ERRORS)
	await waitFor(() => expect(panel).toHaveFocus())
	await settled()
	expect(panel).toHaveFocus()
	expect(footer()).toHaveClass('is-uploading')
	expect(assertive()).toEqual([`harbor.jpg: ${REASON}`])

	await escapePanel()
	await settled()
	expect(within(frame()).getByRole('button', { name: 'Upload media' })).toHaveFocus()
	expect(screen.getByRole('button', { name: '1 upload error' })).toBeInTheDocument()
})

test('speaks the files refused in one drop as one line, so the region keeps each of them', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)

	drop(NOTES, HOLLOW)
	await screen.findByRole('button', { name: '2 upload errors' })

	expect(document.getElementById('a11y-speak-assertive')?.textContent).toBe(`${NOTES_LINE} ${HOLLOW_LINE}`)
})
