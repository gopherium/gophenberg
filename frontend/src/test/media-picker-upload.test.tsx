// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses } from '@gopherium/godmin/testing'
import { QueryClient } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StrictMode } from 'react'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import '../content/editor.css'
import { gate } from './gate'
import {
	card,
	choose,
	CLIFF,
	drop,
	frame,
	HARBOR,
	HOLLOW,
	listing,
	MANUAL,
	NOTES,
	openPicker,
	PHOTO,
	REASON,
	recording,
	SECOND,
	stored,
	TYPELESS,
	uploadsAnswering,
} from './mediaPicker'
import { siteSettings } from './siteSettings'

/** What tells a toast from the words of a row. */
const TOAST = { selector: '.godmin-toast' }

/** A block that takes pictures alone. */
const IMAGE_BLOCK = { allowedTypes: ['image'] }

/** The two ways a file reaches the picker. */
const WAYS = [
	{ way: 'a pick', hand: choose },
	{ way: 'a drop', hand: async (...files: File[]) => drop(...files) },
]

/**
 * Opens the picker on its first page.
 * @param props - The props the block editor would pass.
 * @returns What the picker reported to onSelect.
 */
async function openOnCards(props: Record<string, unknown> = {}) {
	const chosen = await openPicker(props)
	await card(HARBOR)
	return chosen
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
 * Presses the Select button of the open picker.
 */
async function pressSelect() {
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))
}

/**
 * Closes the open picker by its close button.
 */
async function closeFrame() {
	await userEvent.click(within(frame()).getByRole('button', { name: 'Close' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

/**
 * Gives whatever an upload set off the time to reach the picker.
 * @returns A promise settling once the time has passed.
 */
function settled() {
	return act(() => new Promise((resolve) => setTimeout(resolve, 50)))
}

beforeEach(() => {
	listing()
})

test('offers Upload media as the minimal neutral button, drawn with an icon', async () => {
	await openOnCards()

	const button = within(frame()).getByRole('button', { name: 'Upload media' })

	expect([...button.classList]).toEqual(buttonClasses('minimal', 'default', 'neutral'))
	expect(button.querySelector('svg')).not.toBeNull()
})

test.each([
	{ props: {}, accept: null, how: 'names no kinds' },
	{ props: { allowedTypes: ['*'] }, accept: null, how: 'accepts anything' },
	{ props: IMAGE_BLOCK, accept: 'image/*', how: 'names a family' },
	{ props: { allowedTypes: ['image', 'video'] }, accept: 'image/*,video/*', how: 'names two families' },
	{ props: { allowedTypes: ['image/*'] }, accept: 'image/*', how: 'writes a family with a star' },
	{ props: { allowedTypes: ['image/jpeg'] }, accept: 'image/jpeg', how: 'names a full type' },
])('offers $accept in the file dialog when the block $how', async ({ props, accept }) => {
	await openOnCards(props)

	expect(screen.getByLabelText('Upload media').getAttribute('accept')).toBe(accept)
})

test('takes several files at once even for a block that takes one', async () => {
	await openOnCards()

	expect(screen.getByLabelText('Upload media')).toHaveAttribute('multiple')
})

test.each(WAYS)('sends one upload for each file of $way', async ({ hand }) => {
	const sent = uploadsAnswering()
	await openOnCards()

	await hand(PHOTO, SECOND)

	await waitFor(() => expect(sent.toSorted()).toEqual([SECOND.name, PHOTO.name]))
})

test('keeps Upload media working during a run, a second pick running beside the first', async () => {
	const { held, sent } = heldUploads()
	await openOnCards()

	await choose(PHOTO)
	await waitFor(() => expect(sent).toEqual([PHOTO.name]))

	expect(within(frame()).getByRole('button', { name: 'Upload media' })).toHaveAttribute('aria-disabled', 'false')
	await choose(SECOND)
	await waitFor(() => expect(sent).toEqual([PHOTO.name, SECOND.name]))
	held.release()
	await screen.findByText('Uploaded 2 files.', TOAST)
})

test.each([
	{
		how: 'a file of a type the block refuses, sending nothing',
		props: IMAGE_BLOCK,
		file: NOTES,
		answer: stored,
		line: 'notes.txt: notes.txt: Sorry, this file type is not supported here.',
		asked: [],
	},
	{
		how: 'an empty file, sending nothing',
		props: {},
		file: HOLLOW,
		answer: stored,
		line: 'hollow.jpg: hollow.jpg: This file is empty.',
		asked: [],
	},
	{
		how: 'a file the server refuses, with its reason',
		props: {},
		file: NOTES,
		answer: stored,
		line: `notes.txt: ${REASON}`,
		asked: [NOTES.name],
	},
	{
		how: 'an upload that never reached the server',
		props: {},
		file: PHOTO,
		answer: () => HttpResponse.error(),
		line: 'harbor.jpg: The server could not be reached, so nothing was uploaded.',
		asked: [PHOTO.name],
	},
	{
		how: 'an answer that cannot be read',
		props: {},
		file: PHOTO,
		answer: () => new HttpResponse('stored', { status: 201 }),
		line: 'harbor.jpg: Something went wrong. Try again.',
		asked: [PHOTO.name],
	},
])('lists $how', async ({ props, file, answer, line, asked }) => {
	const sent = uploadsAnswering(answer)
	await openOnCards(props)

	await choose(file)

	expect(await screen.findByText(line)).toBeInTheDocument()
	expect(sent).toEqual(asked)
})

test.each(WAYS)('sends a file with no known type to the server from an image block, by $way', async ({ hand }) => {
	const sent = uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)

	await hand(TYPELESS)

	await waitFor(() => expect(sent).toEqual([TYPELESS.name]))
})

test('lists a dropped file of a type the block refuses', async () => {
	const sent = uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)

	drop(NOTES)

	expect(await screen.findByText(/^notes\.txt: notes\.txt: Sorry/)).toBeInTheDocument()
	expect(sent).toEqual([])
})

test.each([
	{
		how: 'hands a block that takes many its new files in the order picked',
		props: { multiple: true },
		files: [PHOTO, SECOND],
		handed: [{ id: HARBOR.id }, { id: CLIFF.id }],
	},
	{
		how: 'adds new files to a block that takes many, in the order picked and without repeats',
		props: { multiple: true, value: [CLIFF.id] },
		files: [PHOTO, SECOND],
		handed: [{ id: CLIFF.id }, { id: HARBOR.id }],
	},
	{
		how: 'replaces what a one file block holds with the first file',
		props: { value: MANUAL.id },
		files: [SECOND, PHOTO],
		handed: { id: CLIFF.id },
	},
	{
		how: 'leaves the selection alone when a file with no type lands as a kind the block refuses',
		props: { ...IMAGE_BLOCK, value: HARBOR.id },
		files: [TYPELESS],
		handed: { id: HARBOR.id },
	},
])('$how', async ({ props, files, handed }) => {
	uploadsAnswering()
	const chosen = await openOnCards(props)

	await choose(...files)
	await within(frame()).findByText(/uploaded\.$|^Uploaded/, TOAST)
	await pressSelect()

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject(handed)
})

test('keeps the tick of a one file block whose upload failed', async () => {
	uploadsAnswering()
	await openOnCards({ value: HARBOR.id })

	await choose(NOTES)
	await screen.findByText(`notes.txt: ${REASON}`)

	expect(await card(HARBOR)).toHaveAttribute('aria-selected', 'true')
})

test.each([
	{ how: 'reads the page again once a file landed', file: PHOTO, grows: true },
	{ how: 'reads nothing again when no file landed', file: NOTES, grows: false },
])('$how', async ({ file, grows }) => {
	const asked = recording()
	uploadsAnswering()
	await openOnCards()
	await settled()
	const before = asked.length

	await choose(file)
	await settled()

	expect(asked.length > before).toBe(grows)
})

test.each([
	{ how: 'names the one file it stored', files: [PHOTO], words: '"harbor.jpg" uploaded.' },
	{ how: 'counts the files it stored', files: [PHOTO, SECOND], words: 'Uploaded 2 files.' },
	{ how: 'counts no refused file', files: [PHOTO, NOTES], words: 'Uploaded 1 file.' },
])('$how in a toast inside the dialog', async ({ files, words }) => {
	uploadsAnswering()
	await openOnCards()

	await choose(...files)

	expect(await within(frame()).findByText(words, TOAST)).toBeInTheDocument()
})

test('raises one toast for two picks that overlap', async () => {
	const { held, sent } = heldUploads()
	await openOnCards()
	await choose(PHOTO)
	await choose(SECOND)
	await waitFor(() => expect(sent).toHaveLength(2))

	held.release()
	await within(frame()).findByText('Uploaded 2 files.', TOAST)
	await settled()

	expect(document.querySelectorAll(TOAST.selector)).toHaveLength(1)
})

test('raises no toast when every file was refused', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)

	await choose(NOTES)
	await screen.findByText(/^notes\.txt: notes\.txt: Sorry/)
	await settled()

	expect(document.querySelector(TOAST.selector)).toBeNull()
})

test('counts in its toast only the files of this run, not those an open panel kept', async () => {
	uploadsAnswering()
	await openOnCards(IMAGE_BLOCK)
	await choose(PHOTO, NOTES)
	await within(frame()).findByText('Uploaded 1 file.', TOAST)

	drop(SECOND)

	expect(await within(frame()).findByText('"cliff.jpg" uploaded.', TOAST)).toBeInTheDocument()
})

test('goes on after a close, asks for a reload and raises its toast once on the next open', async () => {
	const invalidated = vi.spyOn(QueryClient.prototype, 'invalidateQueries')
	onTestFinished(() => invalidated.mockRestore())
	const { held, sent } = heldUploads()
	await openPicker({}, { wrapper: StrictMode })
	await card(HARBOR)
	await choose(PHOTO)
	await waitFor(() => expect(sent).toEqual([PHOTO.name]))
	await closeFrame()

	held.release()
	await waitFor(() => expect(invalidated).toHaveBeenCalledWith({ queryKey: ['media'] }))
	await userEvent.click(await screen.findByRole('button', { name: 'Media Library' }))

	await within(await screen.findByRole('dialog', { name: 'Select Media' })).findByText('"harbor.jpg" uploaded.', TOAST)
	await settled()
	expect(document.querySelectorAll(TOAST.selector)).toHaveLength(1)
})

test('holds the toast of a run that ends while the dialog plays its exit until the next open', async () => {
	const exit = gate()
	const { held, sent } = heldUploads()
	await openOnCards()
	await choose(PHOTO)
	await waitFor(() => expect(sent).toEqual([PHOTO.name]))
	const playing = {
		effect: { getTiming: () => ({ duration: 200, iterations: 1 }) },
		finished: exit.held,
		pending: false,
		playState: 'running',
	}
	const elements = Element.prototype as unknown as { getAnimations?: () => unknown[] }
	elements.getAnimations = () => [playing]
	onTestFinished(() => {
		delete elements.getAnimations
	})
	await userEvent.click(within(frame()).getByRole('button', { name: 'Close' }))

	held.release()
	await settled()
	expect(document.querySelector(TOAST.selector)).toBeNull()

	exit.release()
	delete elements.getAnimations
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(screen.getByRole('button', { name: 'Media Library' }))
	expect(await within(frame()).findByText('"harbor.jpg" uploaded.', TOAST)).toBeInTheDocument()
})

test('keeps Select working during a run, handing over what is ticked at that moment', async () => {
	const { held } = heldUploads()
	const chosen = await openOnCards()
	await userEvent.click(await card(CLIFF))
	await choose(PHOTO)

	await pressSelect()

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject({ id: CLIFF.id })
	held.release()
})

test.each([
	{ property: 'bottom', value: '49px' },
	{ property: 'inset-inline-end', value: 'var(--gophenberg-editor-sidebar-width)' },
	{ property: 'inset-inline-start', value: 'var(--gophenberg-editor-outline-width)' },
])('moves $property of the app toasts around the editor, and not of the toasts in a dialog', ({ property, value }) => {
	render(
		<>
			<div className="gophenberg-editor">
				<div className="gophenberg-editor__sidebar" />
				<div className="gophenberg-editor__outline" />
			</div>
			<div id="root">
				<div className="godmin-toasts" data-testid="in the app" />
			</div>
			<div className="godmin-toasts" data-testid="in a dialog" />
		</>,
	)

	expect(getComputedStyle(screen.getByTestId('in the app')).getPropertyValue(property)).toBe(value)
	expect(getComputedStyle(screen.getByTestId('in a dialog')).getPropertyValue(property)).not.toBe(value)
})

test('stays on the page on show when a file lands on another, only the count of selected files telling', async () => {
	const onPage = { items: [HARBOR], total: 60, per_page: siteSettings.picker_page_size }
	server.use(http.get('/api/media', () => HttpResponse.json(onPage)))
	uploadsAnswering()
	await openOnCards()

	await choose(SECOND)
	await within(frame()).findByText('"cliff.jpg" uploaded.', TOAST)

	expect(screen.getByRole('option', { name: HARBOR.title })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: CLIFF.title })).not.toBeInTheDocument()
	expect(screen.getByText('1 Item selected')).toBeInTheDocument()
})
