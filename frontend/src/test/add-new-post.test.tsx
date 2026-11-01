// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { storedPostWithId } from './postFixture'

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/content/:id', ({ params }) =>
			HttpResponse.json(storedPostWithId(String(params.id))),
		),
	)
})

const NEW_POST_ID = '019fb000-0000-7000-8000-0000000000aa'

/**
 * Serves a created draft and records the request body.
 * @returns The recorded bodies, filled as requests arrive.
 */
function captureCreate(): { bodies: unknown[] } {
	const bodies: unknown[] = []
	server.use(
		http.post('/api/content', async ({ request }) => {
			bodies.push(await request.json())
			return HttpResponse.json(
				{ id: NEW_POST_ID, type: 'post', slug: 'untitled', title: '', status: 'draft' },
				{ status: 201 },
			)
		}),
	)
	return { bodies }
}

/**
 * Returns the add button the list header holds, apart from the sidebar's.
 * @param name - The words the button shows, Add Post unless a test names others.
 * @returns The header's add button.
 */
async function headerAddNew(name = 'Add Post'): Promise<HTMLElement> {
	const [header] = await within(await screen.findByRole('main')).findAllByRole('button', { name })
	return header
}

test('the add button of an empty list creates a draft and opens it in the editor', async () => {
	const recorded = captureCreate()
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
	const { router } = renderRoutedAt('/content/post')
	const empty = (await screen.findByText('No items yet')).closest('.godmin-empty') as HTMLElement

	await userEvent.click(within(empty).getByRole('button', { name: 'Add Post' }))

	await vi.waitFor(() => expect(router.state.location.pathname).toBe(`/content/post/${NEW_POST_ID}/edit`))
	expect(recorded.bodies).toHaveLength(1)
})

test('holds both add buttons of an empty list while one creates the draft, so only one draft is made', async () => {
	const answer = gate()
	const created: unknown[] = []
	const draft = { id: NEW_POST_ID, type: 'post', slug: 'untitled', title: '', status: 'draft' }
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.post('/api/content', async () => {
			created.push(draft)
			await answer.held
			return HttpResponse.json(draft, { status: 201 })
		}),
	)
	const { router } = renderRoutedAt('/content/post')
	const empty = (await screen.findByText('No items yet')).closest('.godmin-empty') as HTMLElement
	await userEvent.click(within(empty).getByRole('button', { name: 'Add Post' }))
	const header = await headerAddNew()
	await vi.waitFor(() => expect(header).toHaveAttribute('aria-disabled', 'true'))

	await userEvent.click(header)
	answer.release()

	await vi.waitFor(() => expect(router.state.location.pathname).toBe(`/content/post/${NEW_POST_ID}/edit`))
	expect(created).toHaveLength(1)
})

test('the add button of the list header creates a draft and opens it in the editor', async () => {
	const recorded = captureCreate()
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
	const { router } = renderRoutedAt('/content/post')

	await userEvent.click(await headerAddNew())

	expect(await screen.findByTitle('Editor canvas')).toBeInTheDocument()
	expect(router.state.location.pathname).toBe(`/content/post/${NEW_POST_ID}/edit`)
	expect(recorded.bodies).toEqual([{ type: 'post', title: '', fields: {} }])
})

test('the add button of the list header reports a failure in a toast without navigating away', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.post('/api/content', () => HttpResponse.json({ error: 'nope' }, { status: 500 })),
	)
	renderAt('/content/post')

	await userEvent.click(await headerAddNew())

	expect(await screen.findByText('The draft could not be created.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument()
	expect(await headerAddNew()).toHaveFocus()
})

test('the add button of the list header reports the reason the server gave in a toast', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.post('/api/content', () =>
			HttpResponse.json({ error: 'content: type inactive', code: 'type_inactive' }, { status: 422 }),
		),
	)
	renderAt('/content/post')

	await userEvent.click(await headerAddNew())

	expect(
		await screen.findByText(
			'This content type is turned off, so its content is not served or edited. Activate it first.',
			{ selector: '.godmin-toast' },
		),
	).toBeInTheDocument()
})

/** The post type whose rating field names a default of 5. */
const RATED_TYPE = {
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
	fields: [
		{
			key: 'rating',
			label: 'Rating',
			kind: 'number',
			many: false,
			required: false,
			settings: { default: 5 },
			updated_at: '2026-08-01T10:00:00Z',
		},
		{
			key: 'subtitle',
			label: 'Subtitle',
			kind: 'text',
			many: false,
			required: false,
			updated_at: '2026-08-01T10:00:00Z',
		},
	],
}

test('add new starts a draft on the defaults its fields name', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [RATED_TYPE] })),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
	const recorded = captureCreate()
	renderAt('/content/post')

	await userEvent.click(await headerAddNew())
	await screen.findByTitle('Editor canvas')

	expect(recorded.bodies).toEqual([{ type: 'post', title: '', fields: { rating: 5 } }])
})

test('draws no add button until the type registry answers, then drafts on its defaults', async () => {
	const registry = gate()
	server.use(
		http.get('/api/types', async () => {
			await registry.held
			return HttpResponse.json({ items: [RATED_TYPE] })
		}),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
	const recorded = captureCreate()
	renderAt('/content/post')
	const main = await screen.findByRole('main')
	await within(main).findByRole('heading', { level: 2, name: 'Content' })

	expect(within(main).queryByRole('button', { name: 'Add Content' })).not.toBeInTheDocument()
	expect(within(main).queryByRole('button', { name: 'Add Post' })).not.toBeInTheDocument()

	registry.release()
	await userEvent.click(await headerAddNew())
	await screen.findByTitle('Editor canvas')
	expect(recorded.bodies).toEqual([{ type: 'post', title: '', fields: { rating: 5 } }])
})

test.each([
	{
		when: 'the type registry fails to load',
		types: () => HttpResponse.json({ error: 'unavailable' }, { status: 500 }),
		path: '/content/post',
	},
	{
		when: 'the registry does not hold the listed type',
		types: () => HttpResponse.json({ items: [RATED_TYPE] }),
		path: '/content/missing',
	},
])('draws no add button when $when', async ({ types, path }) => {
	server.use(
		http.get('/api/types', types),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
	renderAt(path)
	const main = await screen.findByRole('main')
	await within(main).findByText('No items yet')

	expect(within(main).queryByRole('button', { name: 'Add Content' })).not.toBeInTheDocument()
})

/**
 * Returns a type a person registered, with no field.
 * @param key - The type key.
 * @param singular - The singular name the person typed.
 * @param plural - The plural name the person typed.
 * @returns The type as the registry serves it.
 */
function registeredType(key: string, singular: string, plural: string) {
	return {
		...RATED_TYPE,
		key,
		singular_label: singular,
		plural_label: plural,
		route_word: key,
		default: false,
		fields: [],
	}
}

/**
 * Serves the built-in post type beside one type a person registered, and an empty list.
 * @param registered - The type the person registered.
 */
function serveTypes(registered: ReturnType<typeof registeredType>) {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [RATED_TYPE, registered] })),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
}

test('names the header button of a type a person registered after its singular name', async () => {
	serveTypes(registeredType('book', 'Book', 'Books'))
	renderAt('/content/book')

	expect(await headerAddNew('Add Book')).toBeInTheDocument()
})

test('names the header button of the posts list as WordPress es_ES does for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveTypes(registeredType('book', 'Libro', 'Libros'))
	renderAt('/content/post')

	expect(await headerAddNew('Añadir entrada')).toBeInTheDocument()
})

test('names the header button of a registered type with its name as typed for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveTypes(registeredType('book', 'Libro', 'Libros'))
	renderAt('/content/book')

	expect(await headerAddNew('Añadir Libro')).toBeInTheDocument()
})
