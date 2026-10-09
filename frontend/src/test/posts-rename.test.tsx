// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

const LISTED_VERSION = '2026-07-20T10:00:00Z'

const CURRENT_VERSION = '2026-07-21T08:30:00Z'

const PUBLISHED = {
	id: storedPost.id,
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: storedPost.author_id,
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: LISTED_VERSION,
}

const patched: Record<string, unknown>[] = []
const patchedAs: (string | null)[] = []
let listings = 0

beforeEach(() => {
	patched.length = 0
	patchedAs.length = 0
	listings = 0
	server.use(
		http.get('/api/content', () => {
			listings += 1
			return HttpResponse.json({ items: [PUBLISHED], total: 1, per_page: 20 })
		}),
		http.get(`/api/content/${PUBLISHED.id}`, () =>
			HttpResponse.json({ ...storedPost, title: PUBLISHED.title, updated_at: CURRENT_VERSION }),
		),
		http.patch(`/api/content/${PUBLISHED.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			patchedAs.push(request.headers.get('content-type'))
			return HttpResponse.json({ ...PUBLISHED, title: body.title, updated_at: '2026-07-21T09:00:00Z' })
		}),
	)
})

/**
 * Opens the rename dialog of the only listed post.
 * @returns The dialog.
 */
async function openRename(): Promise<HTMLElement> {
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Rename…' }))
	return screen.findByRole('dialog', { name: 'Rename' })
}

/**
 * Types a new name in the open rename dialog and submits it.
 * @param dialog - The rename dialog.
 * @param name - The new name.
 */
async function renameTo(dialog: HTMLElement, name: string) {
	const field = within(dialog).getByRole('textbox', { name: 'Name' })
	await userEvent.clear(field)
	await userEvent.type(field, name)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename' }))
}

test('renames in a small dialog headed Rename, its Name field holding the title', async () => {
	renderAt('/content/post')

	const dialog = await openRename()

	expect(dialog).toHaveClass('has-size-small')
	expect(within(dialog).getByRole('heading', { name: 'Rename' })).toBeInTheDocument()
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('Welcome to Gophenberg')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
})

test('keeps Rename off while the name is blank or unchanged', async () => {
	renderAt('/content/post')
	const dialog = await openRename()
	const submit = within(dialog).getByRole('button', { name: 'Rename' })
	const field = within(dialog).getByRole('textbox', { name: 'Name' })

	expect(submit).toHaveAttribute('aria-disabled', 'true')
	await userEvent.clear(field)
	await userEvent.type(field, '   ')

	expect(submit).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(field, 'A New Welcome')
	expect(submit).not.toHaveAttribute('aria-disabled', 'true')
})

test('writes the new name over the version read just before the write, so the newest name wins', async () => {
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	await waitFor(() => expect(patched).toEqual([{ title: 'A New Welcome', updated_at: CURRENT_VERSION }]))
	expect(patchedAs).toEqual(['application/json'])
})

test('says Name updated, closes the dialog and reads the list again', async () => {
	renderAt('/content/post')
	const dialog = await openRename()
	const before = listings

	await renameTo(dialog, 'A New Welcome')

	expect(await screen.findByText('Name updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(listings).toBeGreaterThan(before)
})

test('opens the editor on the new name after a rename from the list, not on a copy read before it', async () => {
	let title = PUBLISHED.title
	server.use(
		http.get(`/api/content/${PUBLISHED.id}`, () =>
			HttpResponse.json({ ...storedPost, title, updated_at: CURRENT_VERSION }),
		),
		http.patch(`/api/content/${PUBLISHED.id}`, async ({ request }) => {
			title = ((await request.json()) as { title: string }).title
			return HttpResponse.json({ ...PUBLISHED, title, updated_at: '2026-07-21T09:00:00Z' })
		}),
	)
	const { router } = renderRoutedAt(`/content/post/${PUBLISHED.id}/edit`)
	expect(await screen.findByRole('textbox', { name: 'Title' })).toHaveValue(PUBLISHED.title)
	await act(() => router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } }))
	await renameTo(await openRename(), 'A New Welcome')
	await screen.findByText('Name updated.', { selector: '.godmin-toast' })

	await act(() =>
		router.navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: 'post', postId: PUBLISHED.id } }),
	)

	expect(await screen.findByRole('textbox', { name: 'Title' })).toHaveValue('A New Welcome')
})

test('keeps the dialog open when Cancel is pressed while the rename runs', async () => {
	const answer = gate()
	server.use(
		http.patch(`/api/content/${PUBLISHED.id}`, async () => {
			await answer.held
			return HttpResponse.json({ ...PUBLISHED, title: 'A New Welcome' })
		}),
	)
	renderAt('/content/post')
	const dialog = await openRename()
	await renameTo(dialog, 'A New Welcome')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	expect(dialog).toBeInTheDocument()
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('shows the reason a refused rename gave under the field, the typed name kept', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/content/${PUBLISHED.id}`, () =>
			HttpResponse.json({ error: 'content: trashed', code: 'content_trashed' }, { status: 422 }),
		),
	)
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(
		'This item is in the trash. Restore it first to work on it again.',
	)
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('A New Welcome')
})

test.each([
	{ refusal: 'a write the server refused with no reason', at: 'patch', status: 500, body: {} },
	{
		refusal: 'a write that lost the race to another save',
		at: 'patch',
		status: 409,
		body: { error: 'content: stale', code: 'content_stale_update' },
	},
	{ refusal: 'a read of the current version that failed', at: 'get', status: 500, body: {} },
])('says the name could not be updated after $refusal', async ({ at, status, body }) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const answer = () => HttpResponse.json(body, { status })
	const address = `/api/content/${PUBLISHED.id}`
	server.use(at === 'patch' ? http.patch(address, answer) : http.get(address, answer))
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	expect(await within(dialog).findByRole('alert')).toHaveTextContent('The name could not be updated.')
})

test('says the name could not be updated when the network drops the write', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch(`/api/content/${PUBLISHED.id}`, () => HttpResponse.error()))
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	expect(await within(dialog).findByRole('alert')).toHaveTextContent('The name could not be updated.')
})
