// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { trashQuestion } from '../content/actionWords'
import type { Post } from '../content/api'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { storedPostWithId } from './postFixture'
import { siteSettings } from './siteSettings'

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

const WELCOME: Post = {
	id: PUBLISHED.id,
	type: 'post',
	parentId: null,
	parentTitle: '',
	path: 'welcome',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	status: 'published',
	excerpt: '',
	authorId: PUBLISHED.author_id,
	authorName: 'Maria Perez',
	publishedAt: PUBLISHED.published_at,
	createdAt: PUBLISHED.created_at,
	updatedAt: PUBLISHED.updated_at,
	date: PUBLISHED.published_at,
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

test('offers View first, then Duplicate…, Rename… and Trash… last, and leaves editing to the title', async () => {
	renderAt('/content/post')

	await openRowActions()

	await screen.findByRole('menuitem', { name: 'Trash…' })
	const offered = screen.getAllByRole('menuitem').map((item) => item.textContent)
	expect(offered).toEqual(['View', 'Duplicate…', 'Rename…', 'Trash…'])
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

test('asks to confirm before trashing', async () => {
	renderAt('/content/post')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent('Move "Welcome to Gophenberg" to the trash?')
	expect(trashed).toEqual([])
})

test('cuts a long title in the trash confirm at the length the site serves', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, toast_name_length: 10 })))
	renderAt('/content/post')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent('Move "Welcome to…" to the trash?')
})

test('quotes the name in the Spanish trash confirm as the Spanish toasts do', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))

	expect(trashQuestion([WELCOME], (title) => title)).toBe('¿Mover «Welcome to Gophenberg» a la papelera?')
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

	await screen.findByRole('menuitem', { name: 'Trash…' })

	expect(screen.queryByRole('menuitem', { name: 'View' })).not.toBeInTheDocument()
})

test('offers no view of an item of a type the site does not serve', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [{ ...BUILT_IN_TYPE, active: false }] })))
	renderAt('/content/post')
	await openRowActions()

	await screen.findByRole('menuitem', { name: 'Trash…' })

	expect(screen.queryByRole('menuitem', { name: 'View' })).not.toBeInTheDocument()
})

test('offers no trash on a post already in the trash', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [{ ...PUBLISHED, status: 'trash' }], total: 1 })))
	renderAt('/content/post')

	await screen.findByText('Welcome to Gophenberg')

	expect(screen.queryByRole('button', { name: 'Actions' })).not.toBeInTheDocument()
})

test('asks the trash confirm under no header, as WordPress does, and confirms with the action', async () => {
	renderAt('/content/post')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))

	const dialog = await screen.findByRole('dialog')
	expect(within(dialog).queryByRole('heading')).not.toBeInTheDocument()
	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Trash' })).toBeInTheDocument()
})

test('opens the trash confirm at the medium width WordPress gives it', async () => {
	renderAt('/content/post')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))

	expect(await screen.findByRole('dialog')).toHaveClass('has-size-medium')
})

test('names a post that has no title yet in the confirm', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...PUBLISHED, title: '' }], total: 1 }),
		),
	)
	renderAt('/content/post')
	await screen.findByRole('link', { name: '(no title)' })
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent('Move "(no title)" to the trash?')
})

/**
 * Trashes the only listed post through its row actions and its confirm.
 */
async function trashTheListedPost() {
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Trash' }))
}

test('trashes the post once confirmed', async () => {
	renderAt('/content/post')

	await trashTheListedPost()

	await waitFor(() => expect(trashed).toEqual([PUBLISHED.id]))
})

test('leaves the post alone when the confirm is dismissed', async () => {
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(trashed).toEqual([])
})

test('refreshes the listing after trashing', async () => {
	renderAt('/content/post')
	const listedBefore = listed.length

	await trashTheListedPost()

	await waitFor(() => expect(listed.length).toBeGreaterThan(listedBefore))
})

test('names the post it trashed in a toast with no undo', async () => {
	renderAt('/content/post')

	await trashTheListedPost()

	expect(await screen.findByText('"Welcome to Gophenberg" moved to the trash.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument()
})

test('moves the focus to the list once the trash finishes', async () => {
	renderAt('/content/post')

	await trashTheListedPost()

	await screen.findByText('"Welcome to Gophenberg" moved to the trash.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('region', { name: 'Posts' })).toHaveFocus())
})

test('cuts a long title in the toast at the length the site serves', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, toast_name_length: 10 })))
	renderAt('/content/post')

	await trashTheListedPost()

	expect(await screen.findByText('"Welcome to…" moved to the trash.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
})

test('shows the reason a refused trash gave above the list', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete('/api/content/:id', () =>
			HttpResponse.json({ error: 'content: item holds children', code: 'content_holds_children' }, { status: 422 }),
		),
	)
	renderAt('/content/post')

	await trashTheListedPost()

	expect(await screen.findByText('This item still holds items nested inside it. Move or delete those first.'))
		.toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('returns the focus to the row a refused trash left in place', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')

	await trashTheListedPost()

	await screen.findByText('The item could not be moved to the trash.')
	await waitFor(() => expect(screen.getByRole('button', { name: 'Actions' })).toHaveFocus())
})

test('reports a trash the server refused in its own words', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')

	await trashTheListedPost()

	expect(await screen.findByText('The item could not be moved to the trash.')).toBeInTheDocument()
})

test('clears the last failure once the next trash starts', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const second = gate()
	let refuse = true
	server.use(
		http.delete('/api/content/:id', async () => {
			if (refuse) {
				return HttpResponse.json({}, { status: 500 })
			}
			await second.held
			return HttpResponse.json({ ...PUBLISHED, status: 'trash' })
		}),
	)
	renderAt('/content/post')
	await trashTheListedPost()
	await screen.findByText('The item could not be moved to the trash.')
	refuse = false

	await trashTheListedPost()

	await waitFor(() =>
		expect(screen.queryByText('The item could not be moved to the trash.')).not.toBeInTheDocument(),
	)
	expect(screen.getByRole('dialog')).toBeInTheDocument()
	second.release()
	await screen.findByText('"Welcome to Gophenberg" moved to the trash.', { selector: '.godmin-toast' })
})

test('keeps the confirm open when Cancel is pressed while the trash runs', async () => {
	const answer = gate()
	server.use(
		http.delete('/api/content/:id', async () => {
			await answer.held
			return HttpResponse.json({ ...PUBLISHED, status: 'trash' })
		}),
	)
	renderAt('/content/post')
	await trashTheListedPost()
	const dialog = screen.getByRole('dialog')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	expect(dialog).toBeInTheDocument()
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('forgets the failure of one type on the list of another', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const pageType = { ...BUILT_IN_TYPE, key: 'page', singular_label: 'Page', plural_label: 'Pages', route_word: 'pages' }
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [BUILT_IN_TYPE, { ...pageType, default: false }] })),
		http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })),
	)
	const { router } = renderRoutedAt('/content/post')
	await trashTheListedPost()
	await screen.findByText('The item could not be moved to the trash.')

	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'page' } })
	})

	expect(await screen.findByRole('heading', { name: 'Pages', level: 1 })).toBeInTheDocument()
	expect(screen.queryByText('The item could not be moved to the trash.')).not.toBeInTheDocument()
})

test('forgets the last failure once another status is shown', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await trashTheListedPost()
	await screen.findByText('The item could not be moved to the trash.')

	await userEvent.click(
		within(screen.getByRole('navigation', { name: 'Filter by status' })).getByRole('link', { name: 'Published' }),
	)

	await waitFor(() =>
		expect(screen.queryByText('The item could not be moved to the trash.')).not.toBeInTheDocument(),
	)
})
