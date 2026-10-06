// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import type { QueryClient } from '@tanstack/react-query'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import '../index.css'
import { errorMessage } from '../content/TypesScreen'
import { renderAt } from './render'

const POST_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: 'Manage the posts on this site.',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [],
}

const PAGE_TYPE = {
	...POST_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	description: 'Manage the pages on this site.',
	route_word: 'pages',
	hierarchical: true,
	default: false,
}

/**
 * Returns a gate an answer waits behind, and what opens it.
 * @returns The gate and the call opening it.
 */
function gate() {
	let release: () => void = () => {}
	const held = new Promise<void>((resolve) => {
		release = resolve
	})
	return { held, release: () => release() }
}

/**
 * Returns how many writes wait their turn behind another write.
 * @param client - The query client the writes run on.
 * @returns The count of waiting writes.
 */
function queued(client: QueryClient) {
	return client.isMutating({ predicate: (write) => write.state.isPaused })
}

/**
 * Registers a guide behind a held answer, then closes Add New Type and opens it again.
 * @returns The gate holding the answer, the query client, and the reopened dialog.
 */
async function reopenWhileRegistering() {
	const register = gate()
	server.use(
		http.post('/api/types', async () => {
			await register.held
			return HttpResponse.json(PAGE_TYPE, { status: 201 })
		}),
	)
	const client = renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })
	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const dialog = await screen.findByRole('dialog', { name: 'Register a content type' })
	await userEvent.type(within(dialog).getByLabelText('Singular name'), 'Guide')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), 'Guides')
	await userEvent.type(within(dialog).getByLabelText('Description'), 'Manage the guides on this site.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Register' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Register' })).toHaveAttribute('aria-disabled', 'true'),
	)
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const reopened = await screen.findByRole('dialog', { name: 'Register a content type' })
	return { register, client, reopened }
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
	await userEvent.type(screen.getByLabelText('Description'), 'Manage the guides on this site.')
	await userEvent.click(screen.getByRole('button', { name: 'Register' }))

	await waitFor(() =>
		expect(sent[0]).toEqual({
			key: 'guide',
			singular_label: 'Guide',
			plural_label: 'Guides',
			description: 'Manage the guides on this site.',
			route_word: 'guides',
		}),
	)
})

test('registers a type with no description', async () => {
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
			description: '',
			route_word: 'guides',
		}),
	)
})

test('empties the description field once a type is registered', async () => {
	server.use(http.post('/api/types', () => HttpResponse.json(PAGE_TYPE, { status: 201 })))
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	await userEvent.type(screen.getByLabelText('Singular name'), 'Guide')
	await userEvent.type(screen.getByLabelText('Plural name'), 'Guides')
	await userEvent.type(screen.getByLabelText('Description'), 'Manage the guides on this site.')
	await userEvent.click(screen.getByRole('button', { name: 'Register' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))

	expect(await screen.findByLabelText('Description')).toHaveValue('')
})

test('ties the address hint to the plural name of a new type', async () => {
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))

	expect(await screen.findByLabelText('Plural name')).toHaveAccessibleDescription(
		'This type will answer under /address.',
	)

	await userEvent.type(screen.getByLabelText('Plural name'), 'Guides')

	expect(screen.getByLabelText('Plural name')).toHaveAccessibleDescription('This type will answer under /guides.')
})

test('explains the description field of a new type', async () => {
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))

	expect(await screen.findByLabelText('Description')).toHaveAccessibleDescription(
		'A descriptive summary of the content type.',
	)
})

test('names the type in the title of the Describe dialog', async () => {
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const posts = within(table).getByRole('row', { name: /Posts/ })
	await userEvent.click(within(posts).getByRole('button', { name: 'Describe' }))

	expect(await screen.findByRole('dialog', { name: 'Describe Posts' })).toBeInTheDocument()
})

test('explains the description field of the Describe dialog', async () => {
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(dialog).getByLabelText('Description')).toHaveAccessibleDescription(
		'A descriptive summary of the content type.',
	)
})

test('describes a type', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...PAGE_TYPE, description: 'Every page this site keeps.' })
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	const field = within(dialog).getByLabelText('Description')

	expect(field).toHaveValue('Manage the pages on this site.')

	await userEvent.clear(field)
	await userEvent.type(field, 'Every page this site keeps.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toEqual({ description: 'Every page this site keeps.' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('changes no description when the dialog is cancelled', async () => {
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
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.type(within(dialog).getByLabelText('Description'), ' Kept by the team.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Manage the pages on this site.')
	expect(sent).toHaveLength(0)
})

test('wraps the actions of a type onto more lines and keeps every label whole', async () => {
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	expect(within(table).getByRole('columnheader', { name: 'Actions' })).not.toHaveClass('godmin-table__actions')
	for (const name of [/Posts/, /Pages/]) {
		const actions = within(within(table).getByRole('row', { name })).getByRole('button', { name: 'Describe' })
			.parentElement as HTMLElement
		expect(actions.closest('td')).not.toHaveClass('godmin-table__actions')
		expect(actions).toHaveStyle({ flexWrap: 'wrap' })
		expect(actions).toHaveClass('gophenberg-types__actions')
		for (const button of within(actions).getAllByRole('button')) {
			expect(getComputedStyle(button).whiteSpace).toBe('nowrap')
		}
	}
})

test('changes no description when the dialog is dismissed', async () => {
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
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.type(within(dialog).getByLabelText('Description'), ' Kept by the team.')
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Manage the pages on this site.')
	expect(sent).toHaveLength(0)
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
	for (const name of ['Describe', 'Change address', 'Make default', 'Delete']) {
		expect(within(declared).queryByRole('button', { name })).not.toBeInTheDocument()
		expect(within(site).getByRole('button', { name })).toBeInTheDocument()
	}
	expect(within(declared).queryByRole('button', { name: 'Stop nesting' })).not.toBeInTheDocument()
	expect(within(declared).getByRole('button', { name: 'Deactivate' })).toBeInTheDocument()
})

test('stops a type nesting', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...PAGE_TYPE, hierarchical: false })
		}),
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, hierarchical: sent.length === 0 }] }),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Stop nesting' }))

	await waitFor(() => expect(sent[0]).toEqual({ hierarchical: false }))
	expect(await within(pages).findByRole('button', { name: 'Let items nest' })).toBeInTheDocument()
	expect(within(pages).queryByText('Nests')).not.toBeInTheDocument()
})

test('lets a flat type nest', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/post', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...POST_TYPE, hierarchical: true })
		}),
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...POST_TYPE, hierarchical: sent.length > 0 }, PAGE_TYPE] }),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const posts = within(table).getByRole('row', { name: /Posts/ })
	await userEvent.click(within(posts).getByRole('button', { name: 'Let items nest' }))

	await waitFor(() => expect(sent[0]).toEqual({ hierarchical: true }))
	expect(await within(posts).findByRole('button', { name: 'Stop nesting' })).toBeInTheDocument()
	expect(within(posts).getByText('Nests')).toBeInTheDocument()
})

test('says how many items keep a type nesting', async () => {
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json(
				{
					error: 'content: items of the type still nest: 2 in page',
					code: 'type_nesting_in_use',
					meta: { type: 'page', items: 2 },
				},
				{ status: 422 },
			),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Stop nesting' }))

	expect(await screen.findByText(/2 of its items sit inside another/)).toBeInTheDocument()
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

test('opens Change address on the stored address after a kept back edit', async () => {
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog', { name: 'Change the address of Pages' })
	await userEvent.type(within(dialog).getByLabelText('Route word'), 'x')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Keep it' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const reopened = await screen.findByRole('dialog', { name: 'Change the address of Pages' })

	expect(within(reopened).getByLabelText('Route word')).toHaveValue('pages')
})

test('reopens Describe on the description a failed save carried', async () => {
	server.use(http.patch('/api/types/page', () => HttpResponse.json({ error: 'content: down' }, { status: 500 })))
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.clear(within(dialog).getByLabelText('Description'))
	await userEvent.type(within(dialog).getByLabelText('Description'), 'Every page this site keeps.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(await screen.findByRole('alert')).toBeInTheDocument()
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Every page this site keeps.')
})

test('reopens Change address on the route word a failed move carried', async () => {
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json({ error: 'content: route word taken' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog', { name: 'Change the address of Pages' })
	await userEvent.clear(within(dialog).getByLabelText('Route word'))
	await userEvent.type(within(dialog).getByLabelText('Route word'), 'sections')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(await screen.findByText(/route word taken/)).toBeInTheDocument()
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const reopened = await screen.findByRole('dialog', { name: 'Change the address of Pages' })

	expect(within(reopened).getByLabelText('Route word')).toHaveValue('sections')
})

test('starts Describe over on the stored description once a failed draft is kept back', async () => {
	server.use(http.patch('/api/types/page', () => HttpResponse.json({ error: 'content: down' }, { status: 500 })))
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.type(within(dialog).getByLabelText('Description'), ' Kept by the team.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(await screen.findByRole('alert')).toBeInTheDocument()
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const kept = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.click(within(kept).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Manage the pages on this site.')
})

test('opens Describe on the stored description after another edit of the type failed', async () => {
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json({ error: 'content: the type still holds content' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.type(within(dialog).getByLabelText('Description'), ' Kept by the team.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Deactivate' }))
	expect(await screen.findByText(/still holds content/)).toBeInTheDocument()
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Manage the pages on this site.')
})

test('opens Describe on the description the registry holds now', async () => {
	let closed = false
	server.use(
		http.patch('/api/types/page', () => {
			closed = true
			return HttpResponse.json({ ...PAGE_TYPE, active: false })
		}),
		http.get('/api/types', () =>
			HttpResponse.json({
				items: [
					POST_TYPE,
					closed ? { ...PAGE_TYPE, active: false, description: 'Every page this site keeps.' } : PAGE_TYPE,
				],
			}),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Deactivate' }))
	expect(await within(pages).findByRole('button', { name: 'Activate' })).toBeInTheDocument()
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(dialog).getByLabelText('Description')).toHaveValue('Every page this site keeps.')
})

test('names the type a saved description updated', async () => {
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json({ ...PAGE_TYPE, description: 'Every page this site keeps.' }),
		),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

	expect(await screen.findByText('Pages updated.')).toBeInTheDocument()
})

test('holds Describe open on the saved description until the list refresh answers', async () => {
	const refresh = gate()
	let refreshing = false
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json({ ...PAGE_TYPE, description: 'Every page this site keeps.' }),
		),
		http.get('/api/types', async () => {
			refreshing = true
			await refresh.held
			return HttpResponse.json({
				items: [POST_TYPE, { ...PAGE_TYPE, description: 'Every page this site keeps.' }],
			})
		}),
	)

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.clear(within(dialog).getByLabelText('Description'))
	await userEvent.type(within(dialog).getByLabelText('Description'), 'Every page this site keeps.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(refreshing).toBe(true))
	const waiting = screen.getByRole('dialog', { name: 'Describe Pages' })

	expect(within(waiting).getByLabelText('Description')).toHaveValue('Every page this site keeps.')

	refresh.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Every page this site keeps.')
})

test('holds Change address open on the saved route word until the list refresh answers', async () => {
	const refresh = gate()
	let refreshing = false
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })
	server.use(
		http.patch('/api/types/page', () => HttpResponse.json({ ...PAGE_TYPE, route_word: 'sections' })),
		http.get('/api/types', async () => {
			refreshing = true
			await refresh.held
			return HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, route_word: 'sections' }] })
		}),
	)

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog', { name: 'Change the address of Pages' })
	await userEvent.clear(within(dialog).getByLabelText('Route word'))
	await userEvent.type(within(dialog).getByLabelText('Route word'), 'sections')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))
	await waitFor(() => expect(refreshing).toBe(true))
	const waiting = screen.getByRole('dialog', { name: 'Change the address of Pages' })

	expect(within(waiting).getByLabelText('Route word')).toHaveValue('sections')

	refresh.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const reopened = await screen.findByRole('dialog', { name: 'Change the address of Pages' })

	expect(within(reopened).getByLabelText('Route word')).toHaveValue('sections')
})

test('closes Describe on Cancel while its save runs', async () => {
	const save = gate()
	server.use(
		http.patch('/api/types/page', async () => {
			await save.held
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Save' })).toHaveAttribute('aria-disabled', 'true'),
	)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	save.release()

	expect(await screen.findByText('Pages updated.')).toBeInTheDocument()
})

test('closes Change address on Keep it while its move runs', async () => {
	const move = gate()
	server.use(
		http.patch('/api/types/page', async () => {
			await move.held
			return HttpResponse.json({ error: 'content: route word taken' }, { status: 422 })
		}),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog', { name: 'Change the address of Pages' })
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Move every address' })).toHaveAttribute(
			'aria-disabled',
			'true',
		),
	)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Keep it' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	move.release()

	expect(await screen.findByText(/route word taken/)).toBeInTheDocument()
})

test('closes Add New Type on Cancel while the type is registered', async () => {
	const register = gate()
	server.use(
		http.post('/api/types', async () => {
			await register.held
			return HttpResponse.json(PAGE_TYPE, { status: 201 })
		}),
	)
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const dialog = await screen.findByRole('dialog', { name: 'Register a content type' })
	await userEvent.type(within(dialog).getByLabelText('Singular name'), 'Guide')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), 'Guides')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Register' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Register' })).toHaveAttribute('aria-disabled', 'true'),
	)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	register.release()

	expect(await screen.findByText('Guides registered.')).toBeInTheDocument()
})

test('leaves a reopened Describe on its new draft when an earlier save is turned away', async () => {
	const save = gate()
	server.use(
		http.patch('/api/types/page', async () => {
			await save.held
			return HttpResponse.json({ error: 'content: the registry is busy' }, { status: 422 })
		}),
	)
	const client = renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.clear(within(dialog).getByLabelText('Description'))
	await userEvent.type(within(dialog).getByLabelText('Description'), 'Every page this site keeps.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Save' })).toHaveAttribute('aria-disabled', 'true'),
	)
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Manage the pages on this site.')
	expect(within(reopened).getByRole('button', { name: 'Save' })).not.toHaveAttribute('aria-disabled', 'true')

	await userEvent.clear(within(reopened).getByLabelText('Description'))
	await userEvent.type(within(reopened).getByLabelText('Description'), 'Pages the team keeps.')
	save.release()
	expect(await screen.findByText(/the registry is busy/)).toBeInTheDocument()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(screen.getByRole('dialog', { name: 'Describe Pages' })).toBeInTheDocument()
	expect(within(reopened).getByLabelText('Description')).toHaveValue('Pages the team keeps.')
})

test('leaves a reopened Change address on its new draft when an earlier move lands', async () => {
	const move = gate()
	let moved = false
	server.use(
		http.patch('/api/types/page', async () => {
			await move.held
			moved = true
			return HttpResponse.json({ ...PAGE_TYPE, route_word: 'sections' })
		}),
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, route_word: moved ? 'sections' : 'pages' }] }),
		),
	)
	const client = renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog', { name: 'Change the address of Pages' })
	await userEvent.clear(within(dialog).getByLabelText('Route word'))
	await userEvent.type(within(dialog).getByLabelText('Route word'), 'sections')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Move every address' })).toHaveAttribute(
			'aria-disabled',
			'true',
		),
	)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Close' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const reopened = await screen.findByRole('dialog', { name: 'Change the address of Pages' })

	expect(within(reopened).getByRole('button', { name: 'Move every address' })).not.toHaveAttribute(
		'aria-disabled',
		'true',
	)

	await userEvent.clear(within(reopened).getByLabelText('Route word'))
	await userEvent.type(within(reopened).getByLabelText('Route word'), 'chapters')
	move.release()
	expect(await screen.findByText('Pages updated.')).toBeInTheDocument()
	expect(await within(pages).findByText('/sections')).toBeInTheDocument()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(screen.getByRole('dialog', { name: 'Change the address of Pages' })).toBeInTheDocument()
	expect(within(reopened).getByLabelText('Route word')).toHaveValue('chapters')
})

test('sends a second Describe save only once the first save to the type has answered', async () => {
	const first = gate()
	const heard: string[] = []
	let stored = PAGE_TYPE.description
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			const asked = (await request.json()) as { description: string }
			heard.push(`sent ${asked.description}`)
			if (heard.length === 1) {
				await first.held
			}
			stored = asked.description
			heard.push(`answered ${asked.description}`)
			return HttpResponse.json({ ...PAGE_TYPE, description: stored })
		}),
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, description: stored }] })),
	)
	const client = renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.clear(within(dialog).getByLabelText('Description'))
	await userEvent.type(within(dialog).getByLabelText('Description'), 'Every page this site keeps.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(heard).toEqual(['sent Every page this site keeps.']))
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const reopened = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.clear(within(reopened).getByLabelText('Description'))
	await userEvent.type(within(reopened).getByLabelText('Description'), 'Pages the team keeps.')
	await userEvent.click(within(reopened).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(heard.length > 1 || queued(client) > 0).toBe(true))
	first.release()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(heard).toEqual([
		'sent Every page this site keeps.',
		'answered Every page this site keeps.',
		'sent Pages the team keeps.',
		'answered Pages the team keeps.',
	])

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const settled = await screen.findByRole('dialog', { name: 'Describe Pages' })

	expect(within(settled).getByLabelText('Description')).toHaveValue('Pages the team keeps.')
})

test('sends a one-click edit only once a Describe save to the same type has answered', async () => {
	const first = gate()
	const heard: string[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			const asked = JSON.stringify(await request.json())
			heard.push(`sent ${asked}`)
			if (heard.length === 1) {
				await first.held
			}
			heard.push(`answered ${asked}`)
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	const client = renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Describe' }))
	const dialog = await screen.findByRole('dialog', { name: 'Describe Pages' })
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(heard).toHaveLength(1))
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(within(pages).getByRole('button', { name: 'Stop nesting' }))
	await waitFor(() => expect(heard.length > 1 || queued(client) > 0).toBe(true))
	first.release()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(heard).toEqual([
		'sent {"description":"Manage the pages on this site."}',
		'answered {"description":"Manage the pages on this site."}',
		'sent {"hierarchical":false}',
		'answered {"hierarchical":false}',
	])
})

test('sends an edit to one type while a save to another type runs', async () => {
	const save = gate()
	let nested = false
	server.use(
		http.patch('/api/types/page', async () => {
			await save.held
			return HttpResponse.json(PAGE_TYPE)
		}),
		http.patch('/api/types/post', () => {
			nested = true
			return HttpResponse.json({ ...POST_TYPE, hierarchical: true })
		}),
	)
	const client = renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	const posts = within(table).getByRole('row', { name: /Posts/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Stop nesting' }))
	await userEvent.click(within(posts).getByRole('button', { name: 'Let items nest' }))

	await waitFor(() => expect(nested).toBe(true))

	save.release()
	await waitFor(() => expect(client.isMutating()).toBe(0))
})

test('leaves a reopened Add New Type on its new fields when an earlier type is registered', async () => {
	const register = gate()
	server.use(
		http.post('/api/types', async () => {
			await register.held
			return HttpResponse.json(PAGE_TYPE, { status: 201 })
		}),
	)
	const client = renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const dialog = await screen.findByRole('dialog', { name: 'Register a content type' })
	await userEvent.type(within(dialog).getByLabelText('Singular name'), 'Guide')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), 'Guides')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Register' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Register' })).toHaveAttribute('aria-disabled', 'true'),
	)
	await userEvent.keyboard('{Escape}')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const reopened = await screen.findByRole('dialog', { name: 'Register a content type' })

	expect(within(reopened).getByRole('button', { name: 'Register' })).not.toHaveAttribute('aria-disabled', 'true')

	await userEvent.clear(within(reopened).getByLabelText('Singular name'))
	await userEvent.type(within(reopened).getByLabelText('Singular name'), 'Lesson')
	await userEvent.clear(within(reopened).getByLabelText('Plural name'))
	await userEvent.type(within(reopened).getByLabelText('Plural name'), 'Lessons')
	register.release()
	expect(await screen.findByText('Guides registered.')).toBeInTheDocument()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(screen.getByRole('dialog', { name: 'Register a content type' })).toBeInTheDocument()
	expect(within(reopened).getByLabelText('Singular name')).toHaveValue('Lesson')
	expect(within(reopened).getByLabelText('Plural name')).toHaveValue('Lessons')
})

test('opens Add New Type empty once a type it was closed on is registered', async () => {
	const register = gate()
	server.use(
		http.post('/api/types', async () => {
			await register.held
			return HttpResponse.json(PAGE_TYPE, { status: 201 })
		}),
	)
	const client = renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const dialog = await screen.findByRole('dialog', { name: 'Register a content type' })
	await userEvent.type(within(dialog).getByLabelText('Singular name'), 'Guide')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), 'Guides')
	await userEvent.type(within(dialog).getByLabelText('Description'), 'Manage the guides on this site.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Register' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Register' })).toHaveAttribute('aria-disabled', 'true'),
	)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	register.release()
	expect(await screen.findByText('Guides registered.')).toBeInTheDocument()
	await waitFor(() => expect(client.isMutating()).toBe(0))
	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const reopened = await screen.findByRole('dialog', { name: 'Register a content type' })

	expect(within(reopened).getByLabelText('Singular name')).toHaveValue('')
	expect(within(reopened).getByLabelText('Plural name')).toHaveValue('')
	expect(within(reopened).getByLabelText('Description')).toHaveValue('')
})

test('empties a reopened Add New Type once the type it still holds is registered', async () => {
	const { register, client, reopened } = await reopenWhileRegistering()

	expect(within(reopened).getByLabelText('Singular name')).toHaveValue('Guide')

	register.release()
	expect(await screen.findByText('Guides registered.')).toBeInTheDocument()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(screen.getByRole('dialog', { name: 'Register a content type' })).toBeInTheDocument()
	expect(within(reopened).getByLabelText('Singular name')).toHaveValue('')
	expect(within(reopened).getByLabelText('Plural name')).toHaveValue('')
	expect(within(reopened).getByLabelText('Description')).toHaveValue('')
})

test.each([
	{ label: 'Singular name', typed: 'book', kept: ['Guidebook', 'Guides', 'Manage the guides on this site.'] },
	{ label: 'Plural name', typed: ' and notes', kept: ['Guide', 'Guides and notes', 'Manage the guides on this site.'] },
	{
		label: 'Description',
		typed: ' Kept by the team.',
		kept: ['Guide', 'Guides', 'Manage the guides on this site. Kept by the team.'],
	},
])('keeps a reopened Add New Type on its draft once its $label changed and an earlier type is registered', async ({
	label,
	typed,
	kept,
}) => {
	const { register, client, reopened } = await reopenWhileRegistering()
	await userEvent.type(within(reopened).getByLabelText(label), typed)

	register.release()
	expect(await screen.findByText('Guides registered.')).toBeInTheDocument()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(screen.getByRole('dialog', { name: 'Register a content type' })).toBeInTheDocument()
	expect(within(reopened).getByLabelText('Singular name')).toHaveValue(kept[0])
	expect(within(reopened).getByLabelText('Plural name')).toHaveValue(kept[1])
	expect(within(reopened).getByLabelText('Description')).toHaveValue(kept[2])
})

test('closes Add New Type once the registry turns the type away', async () => {
	server.use(
		http.post('/api/types', () =>
			HttpResponse.json({ error: 'content: the route word is taken' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const dialog = await screen.findByRole('dialog', { name: 'Register a content type' })
	await userEvent.type(within(dialog).getByLabelText('Singular name'), 'Guide')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), 'Guides')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Register' }))

	expect(await screen.findByText(/route word is taken/)).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('names the type it registered when its plural name changes while it runs', async () => {
	const register = gate()
	server.use(
		http.post('/api/types', async () => {
			await register.held
			return HttpResponse.json(PAGE_TYPE, { status: 201 })
		}),
	)
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const dialog = await screen.findByRole('dialog', { name: 'Register a content type' })
	await userEvent.type(within(dialog).getByLabelText('Singular name'), 'Guide')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), 'Guides')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Register' }))
	await waitFor(() =>
		expect(within(dialog).getByRole('button', { name: 'Register' })).toHaveAttribute('aria-disabled', 'true'),
	)
	await userEvent.type(within(dialog).getByLabelText('Plural name'), ' and notes')
	register.release()

	expect(await screen.findByText('Guides registered.')).toBeInTheDocument()
})

test('closes Add New Type when it is cancelled', async () => {
	renderAt('/content-types')
	await screen.findByRole('region', { name: 'Content Types' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Type' }))
	const dialog = await screen.findByRole('dialog', { name: 'Register a content type' })
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('closes a type and reports no error once the list refreshes', async () => {
	let closed = false
	server.use(
		http.patch('/api/types/page', () => {
			closed = true
			return HttpResponse.json({ ...PAGE_TYPE, active: false })
		}),
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, active: !closed }] })),
	)
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Deactivate' }))

	expect(await within(pages).findByRole('button', { name: 'Activate' })).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('keeps remembered entries out of the route word field', async () => {
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })

	const pages = within(table).getByRole('row', { name: /Pages/ })
	await userEvent.click(within(pages).getByRole('button', { name: 'Change address' }))
	const dialog = await screen.findByRole('dialog', { name: 'Change the address of Pages' })

	expect(within(dialog).getByLabelText('Route word')).toHaveAttribute('autocomplete', 'off')
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
