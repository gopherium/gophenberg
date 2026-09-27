// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest'

import type { Park } from './parks'
import { adminUser, renderRoutedAt } from './render'
import { storedPost, storedPostWithId } from './postFixture'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const OWN_DRAFT = { ...storedPost, author_id: adminUser.id }

const OTHER_POST = { ...storedPostWithId('019fb000-0000-7000-8000-000000000002'), title: 'Another post' }

const TYPE_WITH_COLOR = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
	fields: [
		{ key: 'color', label: 'Color', kind: 'text', many: false, required: false, updated_at: '2026-08-01T10:00:00Z' },
	],
}

/** A post as the fake server holds it, and what the editor sent it. */
interface Served {
	post: Record<string, unknown>
	row: Record<string, unknown> | null
	parks: Park[]
	patches: Record<string, unknown>[]
	getGate: Promise<void> | null
	parkGate: Promise<void> | null
	patchGate: Promise<void> | null
	failParks: boolean
}

type Router = ReturnType<typeof renderRoutedAt>['router']

let stamp = 0

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
	server.use(
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 1, pending: 0, private: 0, published: 0, trash: 0 }),
		),
	)
})

afterEach(() => {
	vi.useRealTimers()
})

/**
 * Returns a moment later than every one the fake stamped before.
 * @returns The moment, as the server writes it.
 */
function nextStamp(): string {
	stamp += 1
	return new Date(Date.UTC(2026, 7, 1, 12, 0, stamp)).toISOString()
}

/**
 * Answers an autosave the way the server does, writing only into its author's current draft.
 * @param served - The post the fake holds.
 * @param body - The autosave the editor sent.
 * @returns The answer.
 */
function parked(served: Served, body: Record<string, unknown>) {
	if (served.failParks) {
		return HttpResponse.json({ error: 'internal error' }, { status: 500 })
	}
	const words = { title: body.title, content: body.content, excerpt: body.excerpt }
	const post = served.post
	if (post.status === 'draft' && post.author_id === adminUser.id && body.updated_at === post.updated_at) {
		const fields = { ...(post.fields as Record<string, unknown>), ...(body.fields as Record<string, unknown>) }
		served.post = { ...post, ...words, fields, updated_at: nextStamp() }
		return HttpResponse.json({
			target: 'post',
			content_id: post.id,
			...words,
			fields: body.fields,
			saved_at: served.post.updated_at,
		})
	}
	served.row = { target: 'autosave', content_id: post.id, ...words, fields: body.fields, saved_at: nextStamp() }
	return HttpResponse.json(served.row)
}

/**
 * Answers a save the way the server does, refusing one prepared against an older version.
 * @param served - The post the fake holds.
 * @param body - The changes the editor sent.
 * @returns The answer.
 */
function patched(served: Served, body: Record<string, unknown>) {
	if (body.updated_at !== served.post.updated_at) {
		return HttpResponse.json({ error: 'the post changed elsewhere', code: 'content_conflict' }, { status: 409 })
	}
	served.post = { ...served.post, ...body, updated_at: nextStamp() }
	return HttpResponse.json(served.post)
}

/**
 * Serves a post, its autosave and its saves the way the server does.
 * @param post - The post as it starts.
 * @returns The state the fake holds, which a test may change.
 */
function serve(post: Record<string, unknown>): Served {
	const served: Served = {
		post,
		row: null,
		parks: [],
		patches: [],
		getGate: null,
		parkGate: null,
		patchGate: null,
		failParks: false,
	}
	const id = String(post.id)
	server.use(
		http.get(`/api/content/${id}`, async () => {
			const answer = served.post
			await served.getGate
			return HttpResponse.json(answer)
		}),
		http.get(`/api/content/${id}/autosave`, () =>
			served.row === null ? HttpResponse.json({}, { status: 404 }) : HttpResponse.json(served.row),
		),
		http.post(`/api/content/${id}/autosave`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			served.parks.push({ body, keepalive: request.keepalive })
			await served.parkGate
			return parked(served, body)
		}),
		http.patch(`/api/content/${id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			served.patches.push(body)
			await served.patchGate
			return patched(served, body)
		}),
	)
	return served
}

/**
 * Serves the posts list over the given posts.
 * @param posts - The posts the list holds.
 */
function listing(...posts: Served[]) {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: posts.map((held) => held.post), total: posts.length })),
	)
}

/**
 * Returns a gate a handler waits on, and the function opening it.
 * @returns The gate and its opener.
 */
function gate(): { held: Promise<void>, release: () => void } {
	let release = () => {}
	const held = new Promise<void>((resolve) => {
		release = resolve
	})
	return { held, release }
}

/**
 * Returns the Title field the editor shows now.
 * @returns The Title textbox.
 */
function titleField(): HTMLElement {
	return screen.getByRole('textbox', { name: 'Title' })
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
 * Leaves the editor for another post and waits until that post shows.
 * @param router - The router the admin runs on.
 */
async function openAnotherPost(router: Router) {
	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: OTHER_POST.id },
		})
	})
	await waitFor(() => expect(titleField()).toHaveValue(OTHER_POST.title))
}

/**
 * Goes back to the page the author came from.
 * @param router - The router the admin runs on.
 */
async function comeBack(router: Router) {
	await act(async () => {
		router.history.back()
	})
}

test('parks the words when the author goes back to the posts list', async () => {
	const held = serve(storedPost)
	listing(held)
	renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))

	await waitFor(() => expect(held.parks).toHaveLength(1))
	expect(held.parks[0].body).toMatchObject({ title: 'Welcome to Gophenberg!' })
	expect(held.parks[0].keepalive).toBe(false)
})

test('parks the words of the post left behind when the author opens another post', async () => {
	const held = serve(storedPost)
	const other = serve(OTHER_POST)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await openAnotherPost(router)

	await waitFor(() => expect(held.parks).toHaveLength(1))
	expect(held.parks[0].body).toMatchObject({ title: 'Welcome to Gophenberg!' })
	expect(other.parks).toEqual([])
})

test('parks nothing when the author leaves an untouched post', async () => {
	const held = serve(storedPost)
	listing(held)
	renderRoutedAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))

	await screen.findByRole('table')
	expect(held.parks).toEqual([])
})

test('shows the parked words when the author comes back to their draft', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await openAnotherPost(router)
	await waitFor(() => expect(held.post.title).toBe('Welcome to Gophenberg!'))
	const parkedAt = held.post.updated_at

	await comeBack(router)

	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!'))
	await userEvent.type(titleField(), '?')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await waitFor(() => expect(held.patches).toHaveLength(1))
	expect(held.patches[0]).toMatchObject({ updated_at: parkedAt })
	expect(await screen.findByText('Draft saved.')).toBeInTheDocument()
})

test('offers the kept words when the author comes back to a post the park could not write', async () => {
	const held = serve({ ...storedPost, status: 'published' })
	serve(OTHER_POST)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await openAnotherPost(router)
	await waitFor(() => expect(held.row).not.toBeNull())

	await comeBack(router)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))
	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!'))
})

test('waits for the park before showing a post the author came back to', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	const parkGate = gate()
	held.parkGate = parkGate.held
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await openAnotherPost(router)

	await comeBack(router)

	expect(await screen.findByText('Loading the post.')).toBeInTheDocument()
	parkGate.release()
	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!'))
})

test('brings back the words typed after pressing Save', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	const patchGate = gate()
	held.patchGate = patchGate.held
	const parkGate = gate()
	held.parkGate = parkGate.held
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await userEvent.type(titleField(), '?')
	await openAnotherPost(router)

	patchGate.release()

	await waitFor(() => expect(held.parks).toHaveLength(1))
	expect(held.parks[0].body).toMatchObject({ title: 'Welcome to Gophenberg!?', updated_at: OWN_DRAFT.updated_at })
	await comeBack(router)
	expect(await screen.findByText('Loading the post.')).toBeInTheDocument()
	parkGate.release()
	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))
	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!?'))
})

test('waits for a timed park still in flight when the author left', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	const parkGate = gate()
	held.parkGate = parkGate.held
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await tick(60000)
	await waitFor(() => expect(held.parks).toHaveLength(1))
	await userEvent.type(titleField(), '?')
	await openAnotherPost(router)
	await waitFor(() => expect(held.parks).toHaveLength(2))

	await comeBack(router)

	expect(await screen.findByText('Loading the post.')).toBeInTheDocument()
	parkGate.release()
	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!'))
	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))
	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!?'))
})

test('drops a read that was in flight when the author left', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	const { router, client } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	const getGate = gate()
	held.getGate = getGate.held
	void client.invalidateQueries({ queryKey: ['post', storedPost.id] })
	await openAnotherPost(router)
	await waitFor(() => expect(held.post.title).toBe('Welcome to Gophenberg!'))

	await comeBack(router)
	getGate.release()

	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!'))
	await tick(50)
	expect(client.getQueryData(['post', storedPost.id])).toMatchObject({ title: 'Welcome to Gophenberg!' })
})

test('shows the post as stored when the leave park fails', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	held.failParks = true
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await openAnotherPost(router)
	await waitFor(() => expect(held.parks).toHaveLength(1))

	await comeBack(router)

	await waitFor(() => expect(titleField()).toHaveValue(OWN_DRAFT.title))
})

test('opens a post at the version a timed park wrote when the author left nothing new', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await tick(60000)
	await waitFor(() => expect(held.post.title).toBe('Welcome to Gophenberg!'))
	const writtenAt = held.post.updated_at
	await openAnotherPost(router)

	await comeBack(router)

	await waitFor(() => expect(titleField()).toHaveValue('Welcome to Gophenberg!'))
	expect(held.parks).toHaveLength(1)
	await userEvent.type(titleField(), '?')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await waitFor(() => expect(held.patches).toHaveLength(1))
	expect(held.patches[0]).toMatchObject({ updated_at: writtenAt })
	expect(await screen.findByText('Draft saved.')).toBeInTheDocument()
})

test('parks the words typed back out after a timed park wrote the draft', async () => {
	const held = serve(OWN_DRAFT)
	serve(OTHER_POST)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await tick(60000)
	await waitFor(() => expect(held.post.title).toBe('Welcome to Gophenberg!'))
	await userEvent.type(titleField(), '{Backspace}')

	await openAnotherPost(router)

	await waitFor(() => expect(held.post.title).toBe(OWN_DRAFT.title))
	const writtenAt = held.post.updated_at
	await comeBack(router)
	await waitFor(() => expect(titleField()).toHaveValue(OWN_DRAFT.title))
	await userEvent.type(titleField(), '?')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await waitFor(() => expect(held.patches).toHaveLength(1))
	expect(held.patches[0]).toMatchObject({ updated_at: writtenAt })
	expect(await screen.findByText('Draft saved.')).toBeInTheDocument()
})

test('keeps the values it did not park when a timed park writes a field into the draft', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [TYPE_WITH_COLOR] })))
	const held = serve({ ...OWN_DRAFT, fields: { color: 'red', sources: ['kept'] } })
	const { client } = renderRoutedAt(EDITOR_PATH)
	const control = await screen.findByLabelText('Color')
	await userEvent.clear(control)
	await userEvent.type(control, 'blue')

	await tick(60000)

	await waitFor(() => expect(held.parks).toHaveLength(1))
	await waitFor(() =>
		expect(client.getQueryData(['post', storedPost.id])).toMatchObject({
			fields: { color: 'blue', sources: ['kept'] },
		}),
	)
})

test('lists the title a park wrote into the post', async () => {
	const held = serve(OWN_DRAFT)
	listing(held)
	const parkGate = gate()
	held.parkGate = parkGate.held
	renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))
	const table = await screen.findByRole('table')
	expect(table).not.toHaveTextContent('Welcome to Gophenberg!')

	parkGate.release()

	await waitFor(() => expect(table).toHaveTextContent('Welcome to Gophenberg!'))
})
