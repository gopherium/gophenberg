// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses } from '@gopherium/godmin/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { heldWrites, holdWrite } from '../content/postWrites'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { expectUnmarked, holdReads, waitMarked } from './reloads'
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

const SECOND = {
	...PUBLISHED,
	id: '019fb000-0000-7000-8000-0000000000bb',
	slug: 'notes-on-the-next-release',
	title: 'Notes on the Next Release',
}

const TRASHED_REASON = 'This item is in the trash. Restore it first to work on it again.'

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
 * Opens the rename dialog of a listed post.
 * @param title - The title of the post, the first listed one unless a test names another.
 * @param menuItem - The words the menu item reads, the English ones unless a test names others.
 * @param heading - The words the dialog is named by, the English ones unless a test names others.
 * @returns The dialog.
 */
async function openRename(title = PUBLISHED.title, menuItem = 'Rename…', heading = 'Rename'): Promise<HTMLElement> {
	const row = within((await screen.findByRole('link', { name: title })).closest('tr') as HTMLElement)
	await userEvent.click(row.getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: menuItem }))
	return screen.findByRole('dialog', { name: heading })
}

/**
 * Types a new name in the open rename dialog and saves it.
 * @param dialog - The rename dialog.
 * @param name - The new name.
 */
async function renameTo(dialog: HTMLElement, name: string) {
	const field = within(dialog).getByRole('textbox', { name: 'Name' })
	await userEvent.clear(field)
	await userEvent.type(field, name)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
}

/**
 * Refuses every rename with the reason the trash gives.
 */
function refuseRenames() {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/content/${PUBLISHED.id}`, () =>
			HttpResponse.json({ error: 'content: trashed', code: 'content_trashed' }, { status: 422 }),
		),
	)
}

test('renames in a medium dialog headed Rename, as WordPress sizes it, its Name field holding the title', async () => {
	renderAt('/content/post')

	const dialog = await openRename()

	expect(dialog).toHaveClass('has-size-medium')
	expect(within(dialog).getByRole('heading', { name: 'Rename' })).toBeInTheDocument()
	expect(within(dialog).getByRole('textbox', { name: 'Name' })).toHaveValue('Welcome to Gophenberg')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
})

test('draws Cancel of the rename dialog as accent text, as WordPress Site Editor dialogs do', async () => {
	renderAt('/content/post')

	const dialog = await openRename()

	expect([...within(dialog).getByRole('button', { name: 'Cancel' }).classList]).toEqual(buttonClasses('minimal'))
})

test('sends the rename with Save, the word WordPress gives its rename button', async () => {
	renderAt('/content/post')

	const dialog = await openRename()

	expect(within(dialog).getByRole('button', { name: 'Save' })).toHaveAttribute('type', 'submit')
	expect(within(dialog).queryByRole('button', { name: 'Rename' })).not.toBeInTheDocument()
})

test('sends the Spanish rename with Guardar under the Renombrar heading', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content/post')

	const dialog = await openRename(PUBLISHED.title, 'Renombrar…', 'Renombrar')

	expect(within(dialog).getByRole('textbox', { name: 'Nombre' })).toHaveValue('Welcome to Gophenberg')
	expect(within(dialog).getByRole('button', { name: 'Guardar' })).toHaveAttribute('type', 'submit')
	expect(within(dialog).getByRole('button', { name: 'Cancelar' })).toBeInTheDocument()
})

test('keeps Save off only while the name is blank, as WordPress keeps its rename', async () => {
	renderAt('/content/post')
	const dialog = await openRename()
	const submit = within(dialog).getByRole('button', { name: 'Save' })
	const field = within(dialog).getByRole('textbox', { name: 'Name' })

	expect(submit).not.toHaveAttribute('aria-disabled', 'true')
	await userEvent.clear(field)
	await userEvent.type(field, '   ')

	expect(submit).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(field, 'A New Welcome')
	expect(submit).not.toHaveAttribute('aria-disabled', 'true')
})

test('sends a name left as it was, as the WordPress rename does', async () => {
	renderAt('/content/post')
	const dialog = await openRename()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(patched).toEqual([{ title: 'Welcome to Gophenberg', updated_at: CURRENT_VERSION }]))
})

test('writes the new name over the version read just before the write, so the newest name wins', async () => {
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	await waitFor(() => expect(patched).toEqual([{ title: 'A New Welcome', updated_at: CURRENT_VERSION }]))
	expect(patchedAs).toEqual(['application/json'])
})

test('waits for a write in flight to the item before reading its version, so the name lands on it', async () => {
	const settledVersion = '2026-07-21T09:30:00Z'
	let answered = false
	server.use(
		http.get(`/api/content/${PUBLISHED.id}`, () =>
			HttpResponse.json({
				...storedPost,
				title: PUBLISHED.title,
				updated_at: answered ? settledVersion : CURRENT_VERSION,
			}),
		),
	)
	const saving = gate()
	const client = renderAt('/content/post')
	const dialog = await openRename()
	holdWrite(
		client,
		PUBLISHED.id,
		saving.held.then(() => {
			answered = true
		}),
	)

	await renameTo(dialog, 'A New Welcome')
	saving.release()

	await waitFor(() => expect(patched).toEqual([{ title: 'A New Welcome', updated_at: settledVersion }]))
})

test('closes the dialog the moment Save is pressed, before the write answers, as WordPress does', async () => {
	const answer = gate()
	server.use(
		http.patch(`/api/content/${PUBLISHED.id}`, async ({ request }) => {
			await answer.held
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			return HttpResponse.json({ ...PUBLISHED, title: body.title })
		}),
	)
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(patched).toEqual([])
	answer.release()
	expect(await screen.findByText('Name updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('says Name updated, closes the dialog and reads the list again', async () => {
	renderAt('/content/post')
	const dialog = await openRename()
	const before = listings

	await renameTo(dialog, 'A New Welcome')

	expect(await screen.findByText('Name updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	expect(listings).toBeGreaterThan(before)
})

test('says Name updated as the write ends and marks the list until the rows come back', async () => {
	renderAt('/content/post')
	const dialog = await openRename()
	holdReads('/api/content')

	await renameTo(dialog, 'A New Welcome')

	await screen.findByText('Name updated.', { selector: '.godmin-toast' })
	await waitMarked()
})

test('marks nothing when the rename was refused', async () => {
	refuseRenames()
	renderAt('/content/post')

	await renameTo(await openRename(), 'A New Welcome')

	await screen.findByText(TRASHED_REASON, { selector: '.godmin-toast' })
	expectUnmarked()
})

test('puts the focus back on the row menu the rename opened from once the rows came back', async () => {
	renderAt('/content/post')

	await renameTo(await openRename(), 'A New Welcome')

	await screen.findByText('Name updated.', { selector: '.godmin-toast' })
	const row = within((await screen.findByRole('link', { name: PUBLISHED.title })).closest('tr') as HTMLElement)
	await waitFor(() => expect(row.getByRole('button', { name: 'Actions' })).toHaveFocus())
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

test('opens the editor on the new name when it opens while the rename is still in flight', async () => {
	const answer = gate()
	let title = PUBLISHED.title
	server.use(
		http.get(`/api/content/${PUBLISHED.id}`, () =>
			HttpResponse.json({ ...storedPost, title, updated_at: CURRENT_VERSION }),
		),
		http.patch(`/api/content/${PUBLISHED.id}`, async ({ request }) => {
			const body = (await request.json()) as { title: string }
			await answer.held
			title = body.title
			return HttpResponse.json({ ...PUBLISHED, title, updated_at: '2026-07-21T09:00:00Z' })
		}),
	)
	const { client, router } = renderRoutedAt('/content/post')
	await renameTo(await openRename(), 'A New Welcome')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await act(() =>
		router.navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: 'post', postId: PUBLISHED.id } }),
	)

	await waitFor(() =>
		expect(heldWrites(client, PUBLISHED.id) ?? screen.queryByRole('textbox', { name: 'Title' })).toBeTruthy(),
	)
	answer.release()

	expect(await screen.findByRole('textbox', { name: 'Title' })).toHaveValue('A New Welcome')
})

test('shows the reason a refused rename gave in a toast, the dialog already closed', async () => {
	refuseRenames()
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	expect(await screen.findByText(TRASHED_REASON, { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('offers the name a refused rename left unsaved when the dialog opens again', async () => {
	refuseRenames()
	renderAt('/content/post')
	await renameTo(await openRename(), 'A New Welcome')
	await screen.findByText(TRASHED_REASON, { selector: '.godmin-toast' })

	const again = await openRename()

	expect(within(again).getByRole('textbox', { name: 'Name' })).toHaveValue('A New Welcome')
})

test('offers the name a refused rename left unsaved only on the item it was typed for', async () => {
	refuseRenames()
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [PUBLISHED, SECOND], total: 2, per_page: 20 })),
	)
	renderAt('/content/post')
	await renameTo(await openRename(), 'A New Welcome')
	await screen.findByText(TRASHED_REASON, { selector: '.godmin-toast' })

	const other = await openRename(SECOND.title)

	expect(within(other).getByRole('textbox', { name: 'Name' })).toHaveValue(SECOND.title)
})

test('opens on the stored name again once a later rename lands', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	let refuse = true
	server.use(
		http.patch(`/api/content/${PUBLISHED.id}`, async ({ request }) => {
			if (refuse) {
				return HttpResponse.json({ error: 'content: trashed', code: 'content_trashed' }, { status: 422 })
			}
			const body = (await request.json()) as Record<string, unknown>
			return HttpResponse.json({ ...PUBLISHED, title: body.title })
		}),
	)
	renderAt('/content/post')
	await renameTo(await openRename(), 'A New Welcome')
	await screen.findByText(TRASHED_REASON, { selector: '.godmin-toast' })
	refuse = false
	await userEvent.click(within(await openRename()).getByRole('button', { name: 'Save' }))
	await screen.findByText('Name updated.', { selector: '.godmin-toast' })

	const again = await openRename()

	expect(within(again).getByRole('textbox', { name: 'Name' })).toHaveValue('Welcome to Gophenberg')
})

test('opens on the saved name when an earlier rename fails after a later one was sent', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const first = gate()
	let title = PUBLISHED.title
	let writes = 0
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [{ ...PUBLISHED, title }], total: 1, per_page: 20 })),
		http.patch(`/api/content/${PUBLISHED.id}`, async ({ request }) => {
			writes += 1
			if (writes === 1) {
				await first.held
				return HttpResponse.json({ error: 'content: trashed', code: 'content_trashed' }, { status: 422 })
			}
			title = ((await request.json()) as { title: string }).title
			return HttpResponse.json({ ...PUBLISHED, title })
		}),
	)
	renderAt('/content/post')
	await renameTo(await openRename(), 'A Failed Welcome')
	await waitFor(() => expect(writes).toBe(1))
	await renameTo(await openRename(), 'A Saved Welcome')

	first.release()

	await screen.findByText(TRASHED_REASON, { selector: '.godmin-toast' })
	await screen.findByText('Name updated.', { selector: '.godmin-toast' })
	const again = await openRename('A Saved Welcome')
	expect(within(again).getByRole('textbox', { name: 'Name' })).toHaveValue('A Saved Welcome')
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
])('says in a toast the name could not be updated after $refusal', async ({ at, status, body }) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const answer = () => HttpResponse.json(body, { status })
	const address = `/api/content/${PUBLISHED.id}`
	server.use(at === 'patch' ? http.patch(address, answer) : http.get(address, answer))
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	expect(await screen.findByText('The name could not be updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('says in a toast the name could not be updated when the network drops the write', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch(`/api/content/${PUBLISHED.id}`, () => HttpResponse.error()))
	renderAt('/content/post')
	const dialog = await openRename()

	await renameTo(dialog, 'A New Welcome')

	expect(await screen.findByText('The name could not be updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})
