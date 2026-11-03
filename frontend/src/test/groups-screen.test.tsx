// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { badgeClasses, buttonClasses } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { placementOf } from '../content/GroupsScreen'
import { groupErrorMessage } from '../content/groups'
import { catalogFor } from '../i18n/catalog'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'
import { findToast } from './typeFixtures'

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
	fields: [],
}

const RECIPE_TYPE = {
	...POST_TYPE,
	key: 'recipe',
	singular_label: 'Recipe',
	plural_label: 'Recipes',
	route_word: 'recipes',
	default: false,
}

const SUBTITLE = {
	key: 'subtitle',
	label: 'Subtitle',
	kind: 'text',
	many: false,
	required: false,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const DETAILS_GROUP = {
	id: 3,
	key: 'article-details',
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [SUBTITLE],
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const EXTRAS_GROUP = {
	...DETAILS_GROUP,
	id: 4,
	key: 'extras',
	title: 'Extras',
	position: 2,
	fields: [],
}

const SOURCES = {
	items: [
		{
			source: 'content_type',
			operators: ['==', '!='],
			values: [
				{ value: '*', label: 'Any content type' },
				{ value: 'post', label: 'Posts' },
				{ value: 'recipe', label: 'Recipes' },
			],
		},
	],
}

/** Serves the given groups from the listing endpoint. */
function listing(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, RECIPE_TYPE] })))
	server.use(http.get('/api/groups/params', () => HttpResponse.json(SOURCES)))
	listing([DETAILS_GROUP, EXTRAS_GROUP])
})

test('lists every group with where it appears and how many fields it holds', async () => {
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	const details = within(table).getByRole('row', { name: /Article details/ })
	expect(within(details).getByText('Posts')).toBeInTheDocument()
	expect(within(details).getByText('1')).toBeInTheDocument()
	expect(within(table).getByText('Extras')).toBeInTheDocument()
})

test('writes how many fields a group holds in the site format', async () => {
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, format_locale: 'es-ES-u-nu-deva' })),
	)
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	const details = within(table).getByRole('row', { name: /Article details/ })
	expect(within(details).getByText('१')).toBeInTheDocument()
})

test('draws one compact solid header button beside compact outline ones, as WordPress does', async () => {
	renderAt('/field-groups')

	const add = await screen.findByRole('button', { name: 'Add Field Group' })

	expect([...add.classList]).toEqual(buttonClasses('solid', 'compact'))
	expect([...screen.getByRole('link', { name: 'Export definitions' }).classList]).toEqual(
		buttonClasses('outline', 'compact'),
	)
	expect([...screen.getByRole('button', { name: 'Import definitions' }).classList]).toEqual(
		buttonClasses('outline', 'compact'),
	)
})

test('offers the definitions as a file download', async () => {
	renderAt('/field-groups')

	const link = await screen.findByRole('link', { name: 'Export definitions' })

	expect(link).toHaveAttribute('href', '/api/definitions')
	expect(link).toHaveAttribute('download')
})

test('says a group with no rules appears nowhere', async () => {
	listing([{ ...DETAILS_GROUP, location: [] }])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	expect(within(table).getByText('Nowhere')).toBeInTheDocument()
})

test('marks a group that is inactive', async () => {
	listing([{ ...DETAILS_GROUP, active: false }])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	expect(within(row).getByText('Inactive')).toBeInTheDocument()
	expect([...within(row).getByText('Inactive').classList]).toEqual(badgeClasses('none'))
})

test('names the plugin that declared a group and keeps its shape out of reach', async () => {
	listing([{ ...DETAILS_GROUP, origin: 'events' }, EXTRAS_GROUP])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const declared = within(table).getByRole('row', { name: /Article details/ })
	const site = within(table).getByRole('row', { name: /Extras/ })

	expect(within(declared).getByText('From events')).toBeInTheDocument()
	expect(within(declared).queryByRole('button', { name: 'Rules' })).not.toBeInTheDocument()
	expect(within(declared).queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
	expect(within(declared).getByRole('button', { name: 'Deactivate' })).toBeInTheDocument()
	expect(within(site).getByRole('button', { name: 'Rules' })).toBeInTheDocument()
	expect(within(site).getByRole('button', { name: 'Delete' })).toBeInTheDocument()
})

test('shows the fields a plugin declared without offering to change them', async () => {
	const venue = { ...SUBTITLE, key: 'venue', label: 'Venue' }
	const details = { ...SUBTITLE, key: 'details', label: 'Details', kind: 'section', fields: [venue] }
	listing([{ ...DETAILS_GROUP, origin: 'events', fields: [SUBTITLE, details] }])
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const declared = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(declared).getByRole('button', { name: 'Fields' }))

	const dialog = await screen.findByRole('dialog')
	expect(within(dialog).getByRole('listitem', { name: 'Subtitle' })).toBeInTheDocument()
	expect(within(dialog).getByRole('listitem', { name: 'Venue' })).toBeInTheDocument()
	const buttons = within(dialog).getAllByRole('button')
	expect(buttons).toHaveLength(1)
	expect(buttons[0]).toHaveAccessibleName('Close')
})

test('keeps the Field Groups title and shows a spinner under it while the groups are on their way', async () => {
	server.use(http.get('/api/groups', () => new Promise(() => {})))
	renderAt('/field-groups')

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('heading', { level: 2 })).toHaveTextContent('Field Groups')
	const status = await within(main).findByRole('status')
	expect(status).toHaveTextContent('Loading field groups.')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
})

test('reports groups it could not load', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/groups', () => new HttpResponse(null, { status: 500 })))
	renderAt('/field-groups')

	expect(await screen.findByText('The field groups could not be loaded.')).toBeInTheDocument()
})

test('creates a group named after what the operator typed', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(EXTRAS_GROUP, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add Field Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Create' }))

	await waitFor(() => expect(sent).toMatchObject({ title: 'Extras' }))
})

test('seeds a new group with a rule so it is never born matching nowhere', async () => {
	let sent: { location?: unknown } = {}
	server.use(
		http.post('/api/groups', async ({ request }) => {
			sent = (await request.json()) as { location?: unknown }
			return HttpResponse.json(EXTRAS_GROUP, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add Field Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Create' }))

	await waitFor(() => expect(sent.location).toEqual([[{ source: 'content_type', operator: '==', value: 'post' }]]))
})

test('deactivates a group so it stops appearing anywhere', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...DETAILS_GROUP, active: false })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Deactivate' }))

	await waitFor(() => expect(sent).toEqual([{ active: false }]))
})

test('deletes a group once the warning is accepted', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete the group' }))

	await waitFor(() => expect(hit).toBe(true))
})

test('warns that deleting a group takes the values its fields hold', async () => {
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))

	const dialog = await screen.findByRole('dialog')
	expect(within(dialog).getByText(/values/i)).toBeInTheDocument()
})

test('keeps a group when the warning is dismissed', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Keep it' }))

	expect(hit).toBe(false)
})

test('moves a group up the order', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [EXTRAS_GROUP, DETAILS_GROUP] })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Extras/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Move Extras up' }))

	await waitFor(() => expect(sent).toEqual({ order: [4, 3] }))
})

test('moves a group down the order', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [EXTRAS_GROUP, DETAILS_GROUP] })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Move Article details down' }))

	await waitFor(() => expect(sent).toEqual({ order: [4, 3] }))
})

test('leaves the first group with no way up and the last with no way down', async () => {
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const first = within(table).getByRole('row', { name: /Article details/ })
	const last = within(table).getByRole('row', { name: /Extras/ })

	expect(within(first).getByRole('button', { name: 'Move Article details up' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(within(last).getByRole('button', { name: 'Move Extras down' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
})

test('reports why a group write was turned away', async () => {
	server.use(
		http.patch('/api/groups/3', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Deactivate' }))

	expect(await findToast(/^Could not save the change\. A field with that name already exists/)).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('reports why the groups could not be reordered', async () => {
	server.use(
		http.put('/api/groups/order', () =>
			HttpResponse.json({ error: 'content: the order misses a group' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Extras/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Move Extras up' }))

	expect(await findToast('Could not save the change. content: the order misses a group')).toBeInTheDocument()
})

test('closes the warning and reports why a group could not be deleted', async () => {
	server.use(
		http.delete('/api/groups/3', () =>
			HttpResponse.json({ error: 'content: no such group', code: 'group_missing' }, { status: 404 }),
		),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete the group' }))

	expect(await findToast('Could not save the change. content: no such group')).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('closes the form and reports why a group could not be created', async () => {
	server.use(
		http.post('/api/groups', () =>
			HttpResponse.json({ error: 'content: a group already carries that title', code: 'group_taken' }, {
				status: 422,
			}),
		),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add Field Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Create' }))

	expect(
		await findToast('Could not save the change. content: a group already carries that title'),
	).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('stores nothing when the create form is abandoned', async () => {
	let hit = false
	server.use(
		http.post('/api/groups', () => {
			hit = true
			return HttpResponse.json(EXTRAS_GROUP, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add Field Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(hit).toBe(false)
})

test('names the fallback when a group write fails without an error', () => {
	expect(groupErrorMessage(new Error('content: a group already carries that title'))).toBe(
		'content: a group already carries that title',
	)
	expect(groupErrorMessage('nonsense')).toBe('The field groups could not be reached.')
})

test('phrases where a group appears from its rules alone', () => {
	const types = [
		{ key: 'post', pluralLabel: 'Posts' },
		{ key: 'recipe', pluralLabel: 'Recipes' },
	]

	expect(placementOf([], types)).toBe('Nowhere')
	expect(placementOf([[{ source: 'content_type', operator: '==', value: '*' }]], types)).toBe(
		'Every content type',
	)
	expect(placementOf([[{ source: 'content_type', operator: '==', value: 'post' }]], types)).toBe('Posts')
	expect(
		placementOf(
			[
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[{ source: 'content_type', operator: '==', value: 'recipe' }],
			],
			types,
		),
	).toBe('Posts or Recipes')
	expect(placementOf([[{ source: 'content_type', operator: '!=', value: 'post' }]], types)).toBe('Not Posts')
	expect(placementOf([[{ source: 'content_type', operator: '==', value: 'vanished' }]], types)).toBe(
		'vanished',
	)
	expect(placementOf([[{ source: 'elsewhere', operator: '==', value: 'x' }]], types)).toBe('elsewhere is x')
})

test('phrases where a group appears with the built-in types in the words of WordPress es_ES', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	const types = [
		{ key: 'post', pluralLabel: 'Posts' },
		{ key: 'page', pluralLabel: 'Leaflets' },
		{ key: 'recipe', pluralLabel: 'Recipes' },
	]

	expect(
		placementOf(
			[
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[{ source: 'content_type', operator: '!=', value: 'page' }],
				[{ source: 'content_type', operator: '==', value: 'recipe' }],
			],
			types,
		),
	).toBe('Entradas o Excepto Leaflets o Recipes')
})

test('names the built-in type a new group starts on for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/field-groups')
	await screen.findByText('Article details')

	await userEvent.click(screen.getByRole('button', { name: 'Añadir grupo de campos' }))

	expect(
		await screen.findByText('Empieza en Entradas, y las reglas se pueden editar después.'),
	).toBeInTheDocument()
})
