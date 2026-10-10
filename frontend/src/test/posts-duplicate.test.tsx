// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { holdWrite } from '../content/postWrites'
import { gate } from './gate'
import { renderAt } from './render'
import { storedPost } from './postFixture'
import { siteSettings } from './siteSettings'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const PARENT_ID = '019fb000-0000-7000-8000-000000000010'

const PUBLISHED = {
	id: storedPost.id,
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: '019fb000-0000-7000-8000-0000000000ee',
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-20T10:00:00Z',
}

const STORED = {
	...storedPost,
	excerpt: 'A short tour of the editor.',
	parent_id: PARENT_ID,
	fields: { rating: 5 },
}

const STAMP = '2026-08-01T10:00:00Z'

const POST_TYPE = {
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
	created_at: STAMP,
	updated_at: STAMP,
	fields: [],
}

const RATING = { key: 'rating', label: 'Rating', kind: 'number', many: false, required: false, updated_at: STAMP }

const LINKED_FROM = {
	key: 'linked-from',
	label: 'Linked from',
	kind: 'backlinks',
	many: true,
	required: false,
	updated_at: STAMP,
}

const ON_SALE = { key: 'on-sale', label: 'On sale', kind: 'boolean', many: false, required: false, updated_at: STAMP }

const SALE_NOTE = {
	key: 'sale-note',
	label: 'Sale note',
	kind: 'text',
	many: false,
	required: false,
	updated_at: STAMP,
	settings: { conditions: [[{ source: 'on-sale', operator: '==', value: 'true' }]] },
}

/**
 * Serves the post type declaring the given fields and the stored item holding the given values.
 * @param fields - The fields the type declares.
 * @param held - The values the item holds.
 */
function declaring(fields: Record<string, unknown>[], held: Record<string, unknown>) {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [{ ...POST_TYPE, fields }] })),
		http.get(`/api/content/${PUBLISHED.id}`, () => HttpResponse.json({ ...STORED, fields: held })),
	)
}

const created: Record<string, unknown>[] = []
const createdAs: (string | null)[] = []
let listings = 0

beforeEach(() => {
	created.length = 0
	createdAs.length = 0
	listings = 0
	server.use(
		http.get('/api/content', ({ request }) => {
			listings += 1
			const trash = new URL(request.url).searchParams.get('status') === 'trash'
			return HttpResponse.json({ items: [{ ...PUBLISHED, status: trash ? 'trash' : 'published' }], total: 1 })
		}),
		http.get(`/api/content/${PUBLISHED.id}`, () => HttpResponse.json(STORED)),
		http.post('/api/content', async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			created.push(body)
			createdAs.push(request.headers.get('content-type'))
			const copy = { ...PUBLISHED, id: '019fb000-0000-7000-8000-0000000000aa', title: body.title }
			return HttpResponse.json(copy, { status: 201 })
		}),
	)
})

/**
 * Opens the duplicate dialog of the only listed post.
 * @returns The dialog.
 */
async function openDuplicate(): Promise<HTMLElement> {
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Duplicate…' }))
	return screen.findByRole('dialog', { name: 'Duplicate' })
}

test('duplicates in a medium dialog headed Duplicate, as WordPress sizes it, its Title field a copy', async () => {
	renderAt('/content/post')

	const dialog = await openDuplicate()

	expect(dialog).toHaveClass('has-size-medium')
	expect(within(dialog).getByRole('heading', { name: 'Duplicate' })).toBeInTheDocument()
	expect(within(dialog).getByRole('textbox', { name: 'Title' })).toHaveValue('Welcome to Gophenberg (Copy)')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
})

test('makes one draft holding the title typed and the content, excerpt, parent and fields of the item', async () => {
	renderAt('/content/post')
	const dialog = await openDuplicate()
	const field = within(dialog).getByRole('textbox', { name: 'Title' })
	await userEvent.clear(field)
	await userEvent.type(field, 'A Second Welcome')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))

	await waitFor(() =>
		expect(created).toEqual([
			{
				type: 'post',
				title: 'A Second Welcome',
				content: STORED.content,
				excerpt: 'A short tour of the editor.',
				parent_id: PARENT_ID,
				fields: { rating: 5 },
			},
		]),
	)
	expect(createdAs).toEqual(['application/json'])
})

test('leaves the empty Linked from field out of a copy, the field the server reads rather than writes', async () => {
	declaring([RATING, LINKED_FROM], { rating: 5, 'linked-from': [] })
	renderAt('/content/post')
	const dialog = await openDuplicate()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))

	await waitFor(() => expect(created).toHaveLength(1))
	expect(created[0].fields).toEqual({ rating: 5 })
})

test('leaves a value under a field its conditions hide out of a copy, never meeting field_hidden', async () => {
	declaring([RATING, ON_SALE, SALE_NOTE], { rating: 5, 'on-sale': false, 'sale-note': 'Half price' })
	renderAt('/content/post')
	const dialog = await openDuplicate()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))

	await waitFor(() => expect(created).toHaveLength(1))
	expect(created[0].fields).toEqual({ rating: 5, 'on-sale': false })
})

test('waits for a write in flight to the item before reading it, so the copy carries the settled words', async () => {
	const settled = {
		content: '<!-- wp:paragraph -->\n<p>Settled words.</p>\n<!-- /wp:paragraph -->',
		excerpt: 'Settled.',
	}
	let answered = false
	server.use(
		http.get(`/api/content/${PUBLISHED.id}`, () => HttpResponse.json(answered ? { ...STORED, ...settled } : STORED)),
	)
	const saving = gate()
	const client = renderAt('/content/post')
	const dialog = await openDuplicate()
	holdWrite(
		client,
		PUBLISHED.id,
		saving.held.then(() => {
			answered = true
		}),
	)

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))
	saving.release()

	await waitFor(() => expect(created).toHaveLength(1))
	expect(created[0]).toMatchObject(settled)
})

test('names no parent for a copy of an item at the top of its tree', async () => {
	server.use(http.get(`/api/content/${PUBLISHED.id}`, () => HttpResponse.json({ ...STORED, parent_id: null })))
	renderAt('/content/post')
	const dialog = await openDuplicate()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))

	await waitFor(() => expect(created).toHaveLength(1))
	expect(created[0]).not.toHaveProperty('parent_id')
})

test('names the copy in a toast cut at the length the site serves, and reads the list again', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, toast_name_length: 10 })))
	renderAt('/content/post')
	const dialog = await openDuplicate()
	const before = listings

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))

	const toast = '"Welcome to…" successfully created.'
	expect(await screen.findByText(toast, { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(listings).toBeGreaterThan(before)
})

test('shows the reason a refused copy gave inside the dialog, the title kept', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.post('/api/content', () =>
			HttpResponse.json({ error: 'content: trashed', code: 'content_trashed' }, { status: 422 }),
		),
	)
	renderAt('/content/post')
	const dialog = await openDuplicate()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(
		'This item is in the trash. Restore it first to work on it again.',
	)
	expect(within(dialog).getByRole('textbox', { name: 'Title' })).toHaveValue('Welcome to Gophenberg (Copy)')
})

/**
 * Answers a request with a failure that names no reason.
 * @returns The failed answer.
 */
function refused() {
	return HttpResponse.json({}, { status: 500 })
}

test.each([
	{ refusal: 'a create the server refused with no reason', answer: () => http.post('/api/content', refused) },
	{ refusal: 'a read of the item that failed', answer: () => http.get(`/api/content/${PUBLISHED.id}`, refused) },
])('says the item could not be duplicated after $refusal', async ({ answer }) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(answer())
	renderAt('/content/post')
	const dialog = await openDuplicate()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent('The item could not be duplicated.')
})

test('greys Cancel out and keeps the dialog open when Cancel is pressed while the copy is made', async () => {
	const answer = gate()
	server.use(
		http.post('/api/content', async () => {
			await answer.held
			return HttpResponse.json({ ...PUBLISHED, id: '019fb000-0000-7000-8000-0000000000aa' }, { status: 201 })
		}),
	)
	renderAt('/content/post')
	const dialog = await openDuplicate()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Duplicate' }))
	const cancel = within(dialog).getByRole('button', { name: 'Cancel' })

	await userEvent.click(cancel)

	expect(cancel).toHaveAttribute('aria-disabled', 'true')
	expect(dialog).toBeInTheDocument()
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('closes the dialog on Cancel, making nothing', async () => {
	renderAt('/content/post')
	const dialog = await openDuplicate()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(created).toEqual([])
})

test('offers no duplicate on an item in the trash', async () => {
	renderAt('/content/post?status=trash')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	await screen.findByRole('menuitem', { name: 'Restore' })
	expect(screen.queryByRole('menuitem', { name: 'Duplicate…' })).not.toBeInTheDocument()
})
