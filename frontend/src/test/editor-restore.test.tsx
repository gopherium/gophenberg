// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { OTHER_POST, comeBack, openAnotherPost } from './anotherPost'
import { collectParks } from './parks'
import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const NEWER = {
	target: 'autosave',
	content_id: storedPost.id,
	title: 'Words the browser kept',
	content: '<!-- wp:paragraph -->\n<p>Kept words.</p>\n<!-- /wp:paragraph -->',
	excerpt: '',
	saved_at: '2026-07-30T09:00:00Z',
}

const OLDER = { ...NEWER, saved_at: '2026-07-20T09:00:00Z' }

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

afterEach(() => {
	vi.useRealTimers()
})

/**
 * Serves the given autosave for the post under edit.
 * @param autosave - The autosave the server holds, or nothing.
 */
function serveAutosave(autosave: object | null) {
	server.use(
		http.get(`/api/content/${storedPost.id}/autosave`, () =>
			autosave === null
				? HttpResponse.json({}, { status: 404 })
				: HttpResponse.json(autosave),
		),
	)
}

test('says nothing when the server kept no words', async () => {
	serveAutosave(null)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument()
})

test('says nothing when the kept words are older than the post', async () => {
	const asked: string[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}/autosave`, () => {
			asked.push('read')
			return HttpResponse.json(OLDER)
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await waitFor(() => expect(asked).toHaveLength(1))

	expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument()
})

test('says nothing when the post already holds the kept words', async () => {
	const asked: string[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}/autosave`, () => {
			asked.push('read')
			return HttpResponse.json({
				...NEWER,
				title: storedPost.title,
				content: storedPost.content,
				excerpt: storedPost.excerpt,
			})
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await waitFor(() => expect(asked).toHaveLength(1))

	expect(screen.queryByRole('button', { name: 'Restore' })).not.toBeInTheDocument()
})

test('says nothing when the kept words trail the post inside one second', async () => {
	const asked: string[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, updated_at: '2026-07-28T09:00:00.1045Z' }),
		),
		http.get(`/api/content/${storedPost.id}/autosave`, () => {
			asked.push('read')
			return HttpResponse.json({ ...NEWER, saved_at: '2026-07-28T09:00:00.104Z' })
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await waitFor(() => expect(asked).toHaveLength(1))

	expect(screen.queryByRole('button', { name: 'Restore' })).not.toBeInTheDocument()
})

test('offers the kept words when they are newer than the post', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)

	expect(await screen.findByText(/unsaved version/i)).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Restore' })).toBeInTheDocument()
})

test('takes the kept words into the editor when restored', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)
	await screen.findByText(/unsaved version/i)

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	await waitFor(() =>
		expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('Words the browser kept'),
	)
})

test('leaves the post unsaved after a restore so the author can keep it', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)
	await screen.findByText(/unsaved version/i)

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	await waitFor(() =>
		expect(screen.getByRole('button', { name: 'Save draft' })).toHaveAttribute(
			'aria-disabled',
			'false',
		),
	)
})

test('drops the offer once it is taken', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)
	await screen.findByText(/unsaved version/i)

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	await waitFor(() => expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument())
})

/**
 * Serves the given answer on the first read of the kept words and the newer words on every read after it.
 * @param first - The answer to the first read.
 * @returns The reads made so far.
 */
function serveKeptWordsLater(first: () => Response): string[] {
	const asked: string[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}/autosave`, () => {
			asked.push('read')
			return asked.length === 1 ? first() : HttpResponse.json(NEWER)
		}),
	)
	return asked
}

/**
 * Answers that the server keeps no words.
 * @returns The answer.
 */
function noKeptWords(): Response {
	return HttpResponse.json({}, { status: 404 })
}

/**
 * Advances the clock inside a React update.
 * @param ms - The milliseconds to advance.
 */
async function tick(ms: number) {
	await act(async () => {
		await vi.advanceTimersByTimeAsync(ms)
	})
}

/**
 * Brings the window back in front of the author and moves the clock on a second.
 */
async function windowComesBack() {
	act(() => {
		window.dispatchEvent(new Event('visibilitychange'))
	})
	await tick(1000)
}

test('makes no offer when the window comes back in the middle of an edit', async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
	const parks = collectParks()
	const asked = serveKeptWordsLater(noKeptWords)
	const client = renderAt(EDITOR_PATH)
	client.setQueryDefaults(['post-autosave'], { staleTime: 0 })
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await tick(60000)
	await waitFor(() => expect(parks).toHaveLength(1))

	await windowComesBack()

	expect(asked).toHaveLength(1)
	expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument()
})

test('makes no offer when the window comes back after the first read failed', async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
	const asked = serveKeptWordsLater(() => HttpResponse.error())
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	await waitFor(() => expect(asked).toHaveLength(1))
	await tick(1000)

	await windowComesBack()

	expect(asked).toHaveLength(1)
	expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument()
})

test('asks the server for kept words when an earlier answer is still cached', async () => {
	serveAutosave(NEWER)
	const client = renderAt(EDITOR_PATH)
	client.setQueryData(['post-autosave', storedPost.id], null)

	expect(await screen.findByText(/unsaved version/i)).toBeInTheDocument()
})

test('asks the server for kept words again when the editor opens again', async () => {
	server.use(http.get(`/api/content/${OTHER_POST.id}`, () => HttpResponse.json(OTHER_POST)))
	const asked = serveKeptWordsLater(noKeptWords)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	await waitFor(() => expect(asked).toHaveLength(1))
	await openAnotherPost(router)

	await comeBack(router)

	expect(await screen.findByText(/unsaved version/i)).toBeInTheDocument()
})
