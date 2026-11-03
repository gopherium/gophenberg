// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { defaultUser } from '@gopherium/react-auth/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { listenAt } from './announcements'
import { gate } from './gate'
import { adminUser, renderAt, renderRoutedAt } from './render'
import { siteSettings } from './siteSettings'
import { findToast } from './typeFixtures'

const PATH = '/settings'

/** The label of the box for the page size. */
const SIZE_BOX = 'Posts per page'

/** The label of the box for the picture quality. */
const QUALITY_BOX = 'Picture quality'

/** The words under a box that holds no whole number. */
const NOT_WHOLE = 'Type a whole number.'

/** The words the server's answer about a page size of 500 reads as. */
const TOO_MANY =
	'A listing carries between 1 and 100 items, so 500 is not a size Gophenberg takes. Pick a number inside that range.'

/**
 * Returns the answer to a save that worked.
 * @returns The answer.
 */
function saved() {
	return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
}

/**
 * Returns the server's answer about a page size of 500.
 * @returns The answer.
 */
function sizeRefused() {
	return HttpResponse.json(
		{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
		{ status: 422 },
	)
}

/**
 * Answers every save with the server's words about a page size of 500.
 * @param held - The promise the answer waits on.
 */
function refuseSize(held: Promise<void> = Promise.resolve()) {
	server.use(
		http.patch('/api/settings', async () => {
			await held
			return sizeRefused()
		}),
	)
}

/**
 * Keeps what each save sends and answers it.
 * @param answer - The answer each save gets.
 * @returns What the saves have sent so far.
 */
function recordSaves(answer: () => Response = saved): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(await request.json())
			return answer()
		}),
	)
	return sent
}

/**
 * Sends the form from the Save button.
 * @returns The press.
 */
function pressSave() {
	return userEvent.click(screen.getByRole('button', { name: 'Save' }))
}

/**
 * Sends the form from the box holding the focus.
 * @returns The press.
 */
function pressEnter() {
	return userEvent.keyboard('{Enter}')
}

/** The ways a reader sends the form. */
const SEND = { Save: pressSave, Enter: pressEnter }

/**
 * Types 500, a size the server refuses, into the page size box.
 * @returns The box.
 */
async function typeTooMany() {
	const field = await screen.findByLabelText(SIZE_BOX)
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	return field
}

beforeAll(async () => {
	await import('../settings/SettingsScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, locale_default: '', content_per_page: 20, jpeg_quality: 82 }),
		),
	)
})

test('keeps the Settings title and shows a spinner under it while the settings are on their way', async () => {
	let reads = 0
	server.use(
		http.get('/api/settings', () => {
			reads += 1
			return reads === 1 ? HttpResponse.json(siteSettings) : new Promise<never>(() => {})
		}),
	)
	renderAt(PATH, adminUser)

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('heading', { level: 2 })).toHaveTextContent('Settings')
	const status = await within(main).findByRole('status')
	expect(status).toHaveTextContent('Loading the settings.')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
})

test('shows what the site chose for itself', async () => {
	renderAt(PATH, adminUser)

	expect(await screen.findByLabelText(SIZE_BOX)).toHaveValue(20)
	expect(await screen.findByLabelText(QUALITY_BOX)).toHaveValue(82)
})

test.each([
	{ box: SIZE_BOX, typed: '5', stored: { content_per_page: 5 }, send: 'Save' },
	{ box: QUALITY_BOX, typed: '30', stored: { jpeg_quality: 30 }, send: 'Save' },
	{ box: SIZE_BOX, typed: '5', stored: { content_per_page: 5 }, send: 'Enter' },
] as const)('stores what $box holds when $send is pressed', async ({ box, typed, stored, send }) => {
	const sent = recordSaves()
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText(box)
	await userEvent.clear(field)
	expect(field).toHaveValue(null)
	await userEvent.type(field, typed)
	await SEND[send]()

	await waitFor(() => expect(sent[0]).toMatchObject(stored))
})

test.each([
	{ box: SIZE_BOX, typed: '', holding: 'nothing' },
	{ box: QUALITY_BOX, typed: '', holding: 'nothing' },
	{ box: SIZE_BOX, typed: '2.5', holding: 'part of a number' },
])('stores nothing while $box holds $holding', async ({ box, typed }) => {
	const sent = recordSaves()
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText(box)
	await userEvent.clear(field)
	await userEvent.type(field, `${typed}{Enter}`)

	await waitFor(() => expect(field).toHaveAccessibleDescription(NOT_WHOLE))
	expect(sent).toHaveLength(0)
})

test('shows the words under each box that holds no whole number', async () => {
	renderAt(PATH, adminUser)

	const size = await screen.findByLabelText(SIZE_BOX)
	const quality = screen.getByLabelText(QUALITY_BOX)
	await userEvent.clear(size)
	await userEvent.clear(quality)
	await pressSave()

	await waitFor(() => expect(size).toHaveAccessibleDescription(NOT_WHOLE))
	expect(quality).toHaveAccessibleDescription(NOT_WHOLE)
})

test('leaves the box that holds a whole number without words', async () => {
	renderAt(PATH, adminUser)

	const quality = await screen.findByLabelText(QUALITY_BOX)
	await userEvent.clear(quality)
	await pressSave()

	await waitFor(() => expect(quality).toHaveAccessibleDescription(NOT_WHOLE))
	expect(screen.getByLabelText(SIZE_BOX)).toHaveAccessibleDescription('')
})

test.each([
	{ failure: 'no whole number', typed: '2.5', words: NOT_WHOLE, send: 'Enter', spoken: [NOT_WHOLE] },
	{ failure: 'a refused value', typed: '500', words: TOO_MANY, send: 'Enter', spoken: [TOO_MANY] },
	{ failure: 'no whole number', typed: '2.5', words: NOT_WHOLE, send: 'Save', spoken: [] },
	{ failure: 'a refused value', typed: '500', words: TOO_MANY, send: 'Save', spoken: [] },
] as const)(
	'says why a box holds $failure after $send, and speaks it only when the focus stayed in the box',
	async ({ typed, words, send, spoken }) => {
		const assertive = listenAt('assertive')
		refuseSize()
		renderAt(PATH, adminUser)

		const field = await screen.findByLabelText(SIZE_BOX)
		await userEvent.clear(field)
		await userEvent.type(field, typed)
		await SEND[send]()

		await waitFor(() => expect(field).toHaveAccessibleDescription(words))
		expect(field).toHaveFocus()
		expect(assertive()).toEqual(spoken)
	},
)

test.each([
	{ from: SIZE_BOX, failing: [QUALITY_BOX], lands: QUALITY_BOX },
	{ from: QUALITY_BOX, failing: [SIZE_BOX, QUALITY_BOX], lands: SIZE_BOX },
])('leaves the words to the focus when Enter in $from moves it to $lands', async ({ from, failing, lands }) => {
	const assertive = listenAt('assertive')
	renderAt(PATH, adminUser)

	await screen.findByLabelText(SIZE_BOX)
	for (const box of failing) {
		await userEvent.clear(screen.getByLabelText(box))
	}
	await userEvent.click(screen.getByLabelText(from))
	await pressEnter()

	await waitFor(() => expect(screen.getByLabelText(lands)).toHaveFocus())
	expect(assertive()).toEqual([])
})

test('sends a number outside the range on, so the server names the range', async () => {
	const sent = recordSaves(sizeRefused)
	renderAt(PATH, adminUser)

	await typeTooMany()
	await pressSave()

	await waitFor(() => expect(sent[0]).toMatchObject({ content_per_page: 500 }))
})

test('offers each box the bounds the server keeps', async () => {
	renderAt(PATH, adminUser)

	const size = await screen.findByLabelText(SIZE_BOX)
	expect(size).toHaveAttribute('min', '1')
	expect(size).toHaveAttribute('max', '100')
	expect(size).toHaveAttribute('step', '1')
	expect(await screen.findByLabelText(QUALITY_BOX)).toHaveAttribute('max', '100')
})

test.each([
	{ box: SIZE_BOX, other: QUALITY_BOX, code: 'per_page_invalid' },
	{ box: QUALITY_BOX, other: SIZE_BOX, code: 'jpeg_quality_invalid' },
])('says under $box why a refused value was not stored, and nothing under $other', async ({ box, other, code }) => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json({ error: 'out of range', code, meta: { value: '50', max: 100 } }, { status: 422 }),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText(box)
	const beside = screen.getByLabelText(other)
	for (const holding of [beside, field]) {
		await userEvent.clear(holding)
		await userEvent.type(holding, '50')
	}
	await pressSave()

	await waitFor(() => expect(field).toHaveAccessibleDescription(expect.stringContaining('100')))
	expect(beside).toHaveAccessibleDescription('')
})

test('stops saying why once the box holds another value', async () => {
	refuseSize()
	renderAt(PATH, adminUser)

	const field = await typeTooMany()
	await pressSave()
	await waitFor(() => expect(field).toHaveAccessibleDescription(TOO_MANY))

	await userEvent.type(field, '{Backspace}')

	expect(field).toHaveValue(50)
	await waitFor(() => expect(field).toHaveAccessibleDescription(''))
})

test('forgets why a value was refused once a save works', async () => {
	refuseSize()
	renderAt(PATH, adminUser)

	const field = await typeTooMany()
	await pressSave()
	await waitFor(() => expect(field).toHaveAccessibleDescription(TOO_MANY))

	const sent = recordSaves()
	await userEvent.type(field, '{Backspace}')
	await pressSave()
	await waitFor(() => expect(sent[0]).toMatchObject({ content_per_page: 50 }))
	await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).not.toHaveAttribute('aria-disabled', 'true'))
	await userEvent.type(field, '0')

	expect(field).toHaveValue(500)
	expect(field).not.toHaveAccessibleDescription(TOO_MANY)
})

test('says in a toast why a value was not stored when the box changed before the answer', async () => {
	const answer = gate()
	refuseSize(answer.held)
	renderAt(PATH, adminUser)

	const field = await typeTooMany()
	await pressSave()
	await userEvent.type(field, '{Backspace}')
	answer.release()

	expect(await findToast(`Could not save the settings. ${TOO_MANY}`)).toBeInTheDocument()
	expect(field).toHaveAccessibleDescription('')
	expect(field).toHaveValue(50)
})

test('says in a toast why a value was not stored when the reader left before the answer', async () => {
	const answer = gate()
	refuseSize(answer.held)
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
	const { router } = renderRoutedAt(PATH, adminUser)

	await typeTooMany()
	await pressSave()
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
	await screen.findByRole('heading', { name: 'Posts', level: 2 })
	answer.release()

	expect(await findToast(`Could not save the settings. ${TOO_MANY}`)).toBeInTheDocument()
})

test('says in a toast why a save failed when the words are about no box', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json({ error: 'The disk is full.', code: 'disk_full' }, { status: 500 }),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText(SIZE_BOX)
	await userEvent.clear(field)
	await userEvent.type(field, '5')
	await pressSave()

	expect(await findToast('Could not save the settings. The disk is full.')).toBeInTheDocument()
	expect(field).toHaveAccessibleDescription('')
	expect(field).toHaveValue(5)
})

test.each([
	{ cause: 'the server gave no reason', answer: () => new HttpResponse('not json at all', { status: 500 }) },
	{ cause: 'the connection dropped', answer: () => HttpResponse.error() },
	{ cause: 'the answer could not be read', answer: () => HttpResponse.json({ stored: true }) },
])('says in a toast that a save failed when $cause', async ({ answer }) => {
	server.use(http.patch('/api/settings', answer))
	renderAt(PATH, adminUser)

	await screen.findByLabelText(SIZE_BOX)
	await pressSave()

	expect(await findToast('Could not save the settings. Something went wrong. Try again.')).toBeInTheDocument()
})

test('offers no boxes to save when the settings could not be read', async () => {
	const sent = recordSaves()
	server.use(http.get('/api/settings', () => HttpResponse.json({ error: 'boom' }, { status: 500 })))
	renderAt(PATH, adminUser)

	expect(await screen.findByRole('alert')).toBeInTheDocument()
	expect(screen.queryByLabelText(SIZE_BOX)).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
	expect(sent).toHaveLength(0)
})

test('keeps an author out of the settings', async () => {
	renderAt(PATH, { ...defaultUser, role: 'author' })

	await waitFor(() => expect(screen.queryByLabelText(SIZE_BOX)).not.toBeInTheDocument())
})
