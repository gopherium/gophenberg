// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
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
