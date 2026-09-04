// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { errorMessage } from '../content/TypesScreen'
import { renderAt } from './render'

const POST_TYPE = {
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
}

const PAGE_TYPE = {
	...POST_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	route_word: 'pages',
	hierarchical: true,
	default: false,
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
})

test('lists every registered type with the address it answers under', async () => {
	renderAt('/content-types')

	const table = await screen.findByRole('region', { name: 'Content Types' })

	expect(within(table).getByText('Posts')).toBeInTheDocument()
	expect(within(table).getByText('Pages')).toBeInTheDocument()
	expect(within(table).getByText('/pages')).toBeInTheDocument()
})

test('marks which type answers at the root', async () => {
	renderAt('/content-types')

	const table = await screen.findByRole('region', { name: 'Content Types' })
	const posts = within(table).getByRole('row', { name: /Posts/ })

	expect(within(posts).getByText('Default')).toBeInTheDocument()
})

test('registers a type from the labels it was given', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(PAGE_TYPE, { status: 201 })
		}),
	)
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	await userEvent.type(screen.getByLabelText('Singular name'), 'Guide')
	await userEvent.type(screen.getByLabelText('Plural name'), 'Guides')
	await userEvent.click(screen.getByRole('button', { name: 'Register' }))

	await waitFor(() =>
		expect(sent[0]).toEqual({
			key: 'guide',
			singular_label: 'Guide',
			plural_label: 'Guides',
			route_word: 'guides',
		}),
	)
})

test('carries the reason the registry refused a type', async () => {
	server.use(
		http.post('/api/types', () =>
			HttpResponse.json({ error: 'content: the route word is taken' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	await userEvent.type(screen.getByLabelText('Singular name'), 'Page')
	await userEvent.type(screen.getByLabelText('Plural name'), 'Pages')
	await userEvent.click(screen.getByRole('button', { name: 'Register' }))

	expect(await screen.findByText(/route word is taken/)).toBeInTheDocument()
})

test('states that every address moves before changing a route word', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...PAGE_TYPE, route_word: 'sections' })
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog')

	expect(within(dialog).getByText(/Every address of this type moves/i)).toBeInTheDocument()

	await userEvent.clear(within(dialog).getByLabelText('Route word'))
	await userEvent.type(within(dialog).getByLabelText('Route word'), 'sections')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))

	await waitFor(() => expect(sent[0]).toEqual({ route_word: 'sections' }))
})

test('closes a type without removing it', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...PAGE_TYPE, active: false })
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Deactivate' }))

	await waitFor(() => expect(sent[0]).toEqual({ active: false }))
})

test('removes a type the registry lets go', async () => {
	let asked = ''
	server.use(
		http.delete('/api/types/page', ({ request }) => {
			asked = new URL(request.url).pathname
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Delete' }))

	await waitFor(() => expect(asked).toBe('/api/types/page'))
})

test('names the plugin that declared a type and keeps its shape out of reach', async () => {
	const recipe = {
		...PAGE_TYPE,
		key: 'recipe',
		singular_label: 'Recipe',
		plural_label: 'Recipes',
		route_word: 'recipes',
	}
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, origin: 'events' }, recipe] }),
		),
	)
	renderAt('/content-types')

	const table = await screen.findByRole('region', { name: 'Content Types' })
	const declared = within(table).getByRole('row', { name: /Pages/ })
	const site = within(table).getByRole('row', { name: /Recipes/ })

	expect(within(declared).getByText('From events')).toBeInTheDocument()
	for (const name of ['Change address', 'Make default', 'Delete']) {
		expect(within(declared).queryByRole('button', { name })).not.toBeInTheDocument()
		expect(within(site).getByRole('button', { name })).toBeInTheDocument()
	}
	expect(within(declared).getByRole('button', { name: 'Deactivate' })).toBeInTheDocument()
})

test('keeps the default type from being deleted or closed', async () => {
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const posts = within(table).getByRole('row', { name: /Posts/ })

	expect(within(posts).queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
	expect(within(posts).queryByRole('button', { name: 'Deactivate' })).not.toBeInTheDocument()
})

test('announces a registry it could not read', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ error: 'nope' }, { status: 500 })))
	renderAt('/content-types')

	expect(await screen.findByText('The content types could not be loaded.')).toBeInTheDocument()
})

test('reopens a type that was closed', async () => {
	const sent: unknown[] = []
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, active: false }] }),
		),
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Activate' }))

	await waitFor(() => expect(sent[0]).toEqual({ active: true }))
})

test('carries the reason the registry refused an edit', async () => {
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json({ error: 'content: the type still holds content' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Deactivate' }))

	expect(await screen.findByText(/still holds content/)).toBeInTheDocument()
})

test('carries the reason a type could not be removed', async () => {
	server.use(
		http.delete('/api/types/page', () =>
			HttpResponse.json({ error: 'content: the type is in use' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Delete' }))

	expect(await screen.findByText(/the type is in use/)).toBeInTheDocument()
})

test('reports a registry that could not be reached at all', async () => {
	server.use(http.patch('/api/types/page', () => HttpResponse.error()))
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Deactivate' }))

	expect(await screen.findByRole('alert')).toBeInTheDocument()
})

test('changes no address when the confirmation is dismissed', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.type(within(dialog).getByLabelText('Route word'), 'x')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Keep it' }))

	expect(sent).toHaveLength(0)
})

test('registers nothing when the new type is cancelled', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(PAGE_TYPE, { status: 201 })
		}),
	)
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	await userEvent.type(screen.getByLabelText('Singular name'), 'Guide')
	await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

	expect(sent).toHaveLength(0)
})

test('names the error a registry write carried', () => {
	expect(errorMessage(new Error('content: the route word is taken'))).toBe(
		'content: the route word is taken',
	)
})

test('names an unreachable registry when the failure carries no message', () => {
	expect(errorMessage('nonsense')).toBe('The registry could not be reached.')
})

test('states what the root hand over moves before moving it', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...PAGE_TYPE, default: true, route_word: '' })
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Make default' }))
	const dialog = await screen.findByRole('dialog')

	expect(within(dialog).getByText(/Pages will answer at the root/i)).toBeInTheDocument()
	expect(within(dialog).getByText(/Posts moves/i)).toBeInTheDocument()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Hand over the root' }))

	await waitFor(() => expect(sent[0]).toEqual({ default: true }))
})

test('hands over nothing when the root confirmation is dismissed', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...PAGE_TYPE, default: true })
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Make default' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Keep it' }))

	expect(sent).toHaveLength(0)
})
