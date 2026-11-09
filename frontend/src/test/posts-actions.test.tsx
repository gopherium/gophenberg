// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { stubLayout } from '@gopherium/godmin/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { gate } from './gate'
import { holdReads } from './reloads'
import { renderAt } from './render'
import { storedPostWithId } from './postFixture'
import { findActionToast } from './typeFixtures'

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

const PUBLISHED = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	path: 'welcome',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-20T10:00:00Z',
}

const BUILT_IN_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: '',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
	fields: [],
}

const listed: string[] = []
const trashed: string[] = []

beforeEach(() => {
	listed.length = 0
	trashed.length = 0
	server.use(
		http.get('/api/content', () => {
			listed.push('asked')
			return HttpResponse.json({ items: [PUBLISHED], total: 1 })
		}),
		http.get('/api/content/:id', ({ params }) =>
			HttpResponse.json(storedPostWithId(String(params.id))),
		),
		http.delete('/api/content/:id', ({ params }) => {
			trashed.push(String(params.id))
			return HttpResponse.json({ ...PUBLISHED, status: 'trash' })
		}),
	)
})

/**
 * Opens the row actions menu of the only listed post.
 */
async function openRowActions() {
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))
}

test('offers View first, then Duplicate…, Rename… and Trash last, and leaves editing to the title', async () => {
	renderAt('/content/post')

	await openRowActions()

	await screen.findByRole('menuitem', { name: 'Trash' })
	const offered = screen.getAllByRole('menuitem').map((item) => item.textContent)
	expect(offered).toEqual(['View', 'Duplicate…', 'Rename…', 'Trash'])
	expect(screen.queryByRole('menuitem', { name: 'Edit' })).not.toBeInTheDocument()
})

/**
 * Returns the names of the buttons a row draws, read once the row of the title shows.
 * @param title - The title of the row.
 * @returns The text or the label of each button on the row, in order.
 */
async function rowButtons(title: string): Promise<(string | null)[]> {
	const row = within((await screen.findByRole('link', { name: title })).closest('tr') as HTMLElement)
	return row.getAllByRole('button').map((button) => button.textContent || button.getAttribute('aria-label'))
}

test('draws only the primary View as a button on a published row, as WordPress shows it on hover', async () => {
	renderAt('/content/post')

	expect(await rowButtons('Welcome to Gophenberg')).toEqual(['View', 'Actions'])
})

test('draws no action button on a row that is not published, leaving every action to the menu', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [{ ...PUBLISHED, status: 'draft' }], total: 1 })))
	renderAt('/content/post')

	expect(await rowButtons('Welcome to Gophenberg')).toEqual(['Actions'])
})

test('opens the public address of a published item in a new tab', async () => {
	const opened = vi.spyOn(window, 'open').mockImplementation(() => null)
	renderAt('/content/post')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'View' }))

	expect(opened).toHaveBeenCalledWith('/welcome', '_blank')
})

test('offers no view of an item that is not published, as it has no public address yet', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [{ ...PUBLISHED, status: 'draft' }], total: 1 })))
	renderAt('/content/post')
	await openRowActions()

	await screen.findByRole('menuitem', { name: 'Trash' })

	expect(screen.queryByRole('menuitem', { name: 'View' })).not.toBeInTheDocument()
})

test('offers no view of an item of a type the site does not serve', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [{ ...BUILT_IN_TYPE, active: false }] })))
	renderAt('/content/post')
	await openRowActions()

	await screen.findByRole('menuitem', { name: 'Trash' })

	expect(screen.queryByRole('menuitem', { name: 'View' })).not.toBeInTheDocument()
})

test('offers no trash on a post already in the trash', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [{ ...PUBLISHED, status: 'trash' }], total: 1 })))
	renderAt('/content/post')

	await screen.findByText('Welcome to Gophenberg')

	expect(screen.queryByRole('button', { name: 'Actions' })).not.toBeInTheDocument()
})

/**
 * Trashes the only listed post through its row actions.
 */
async function trashTheListedPost() {
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash' }))
}

test('trashes the post at once, asking nothing first', async () => {
	renderAt('/content/post')

	await trashTheListedPost()

	await waitFor(() => expect(trashed).toEqual([PUBLISHED.id]))
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
})

test('refreshes the listing after trashing', async () => {
	renderAt('/content/post')
	const listedBefore = listed.length

	await trashTheListedPost()

	await waitFor(() => expect(listed.length).toBeGreaterThan(listedBefore))
})

test('names the post it trashed in a toast that offers Undo', async () => {
	renderAt('/content/post')

	await trashTheListedPost()

	const toast = await findActionToast('"Welcome to Gophenberg" moved to the trash.')
	expect(within(toast).getByRole('button', { name: 'Undo' })).toBeInTheDocument()
})

test('restores the trashed post on a press on Undo and names it in a toast', async () => {
	const restored: string[] = []
	server.use(
		http.post('/api/content/:id/restore', ({ params }) => {
			restored.push(String(params.id))
			return HttpResponse.json({ ...PUBLISHED, status: 'draft' })
		}),
	)
	renderAt('/content/post')
	await trashTheListedPost()
	const toast = await findActionToast('"Welcome to Gophenberg" moved to the trash.')

	await userEvent.click(within(toast).getByRole('button', { name: 'Undo' }))

	expect(await screen.findByText('"Welcome to Gophenberg" has been restored.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
	expect(restored).toEqual([PUBLISHED.id])
})

test('moves the focus to the list once the trash finishes, and to the next row once the rows land', async () => {
	const next = { ...PUBLISHED, id: '019fb000-0000-7000-8000-000000000002', slug: 'next', title: 'Notes for Next Week' }
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [PUBLISHED, next], total: 2 })))
	renderAt('/content/post')
	const row = (await screen.findByRole('link', { name: PUBLISHED.title })).closest('tr') as HTMLElement
	const release = holdReads('/api/content', { items: [next], total: 1 })

	await userEvent.click(within(row).getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash' }))

	await findActionToast('"Welcome to Gophenberg" moved to the trash.')
	await waitFor(() => expect(screen.getByRole('region', { name: 'Posts' })).toHaveFocus())
	release()
	await waitFor(() => expect(screen.getByRole('checkbox', { name: next.title })).toHaveFocus())
})

test('shows the reason a refused trash gave in a toast, never above the list', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete('/api/content/:id', () =>
			HttpResponse.json({ error: 'content: item holds children', code: 'content_holds_children' }, { status: 422 }),
		),
	)
	renderAt('/content/post')

	await trashTheListedPost()

	expect(
		await screen.findByText('This item still holds items nested inside it. Move or delete those first.', {
			selector: '.godmin-toast',
		}),
	).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('returns the focus to the row a refused trash left in place', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')

	await trashTheListedPost()

	await screen.findByText('The item could not be moved to the trash.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('button', { name: 'Actions' })).toHaveFocus())
})

test('reports a trash the server refused in its own words', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')

	await trashTheListedPost()

	expect(await screen.findByText('The item could not be moved to the trash.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
})

test('leaves a dialog opened on the row while a trash runs open once the trash ends', async () => {
	const answer = gate()
	server.use(
		http.delete('/api/content/:id', async () => {
			await answer.held
			return HttpResponse.json({}, { status: 500 })
		}),
	)
	renderAt('/content/post')
	await trashTheListedPost()
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Rename…' }))
	const rename = await screen.findByRole('dialog', { name: 'Rename' })

	answer.release()
	await screen.findByText('The item could not be moved to the trash.', { selector: '.godmin-toast' })
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Rename' })).toBe(rename)
})

test('keeps the focus in a Rename opened while a trash runs once that trash lands', async () => {
	stubLayout()
	const answer = gate()
	server.use(
		http.delete('/api/content/:id', async () => {
			await answer.held
			return HttpResponse.json({ ...PUBLISHED, status: 'trash' })
		}),
	)
	renderAt('/content/post')
	await trashTheListedPost()
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Rename…' }))
	const rename = await screen.findByRole('dialog', { name: 'Rename' })
	const name = within(rename).getByRole('textbox', { name: 'Name' })
	await waitFor(() => expect(name).toHaveFocus())

	answer.release()
	await findActionToast('"Welcome to Gophenberg" moved to the trash.')
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Rename' })).toBe(rename)
	expect(name).toHaveFocus()
})
