// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { operatorLabel } from '../content/RulesDialog'
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

const RECIPE_TYPE = {
	...POST_TYPE,
	key: 'recipe',
	singular_label: 'Recipe',
	plural_label: 'Recipes',
	route_word: 'recipes',
	default: false,
}

const ON_POSTS = {
	id: 3,
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [],
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
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

/** Serves the given group from the listing endpoint. */
function listing(group: unknown) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [group] })))
}

/** Opens the rules of the only listed group and answers with its dialog. */
async function openRules() {
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })
	await userEvent.click(within(row).getByRole('button', { name: 'Rules' }))
	return screen.findByRole('dialog')
}

/** Answers the group edit and records what the dialog sent. */
function recording(sent: { body?: unknown }) {
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			sent.body = await request.json()
			return HttpResponse.json(ON_POSTS)
		}),
	)
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, RECIPE_TYPE] })))
	server.use(http.get('/api/groups/params', () => HttpResponse.json(SOURCES)))
	listing(ON_POSTS)
})

test('shows a stored rule under the source, condition and value it carries', async () => {
	renderAt('/field-groups')

	const dialog = await openRules()
	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })

	expect(within(rule).getByLabelText('Source')).toHaveTextContent('Content type')
	expect(within(rule).getByLabelText('Condition')).toHaveTextContent('is')
	expect(within(rule).getByLabelText('Value')).toHaveTextContent('Posts')
})

test('shows each alternative as its own rule set', async () => {
	listing({
		...ON_POSTS,
		location: [
			[{ source: 'content_type', operator: '==', value: 'post' }],
			[{ source: 'content_type', operator: '==', value: 'recipe' }],
		],
	})
	renderAt('/field-groups')

	const dialog = await openRules()

	expect(within(dialog).getByRole('group', { name: 'Rule set 1' })).toBeInTheDocument()
	expect(within(dialog).getByRole('group', { name: 'Rule set 2' })).toBeInTheDocument()
})

test('narrows a set by adding a condition every item has to meet', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	renderAt('/field-groups')
	const dialog = await openRules()

	const set = within(dialog).getByRole('group', { name: 'Rule set 1' })
	await userEvent.click(within(set).getByRole('button', { name: 'Add condition' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [
				[
					{ source: 'content_type', operator: '==', value: 'post' },
					{ source: 'content_type', operator: '==', value: '*' },
				],
			],
		}),
	)
})

test('widens the placement by adding an alternative set', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	renderAt('/field-groups')
	const dialog = await openRules()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[{ source: 'content_type', operator: '==', value: '*' }],
			],
		}),
	)
})

test('stores the value the operator picked', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	renderAt('/field-groups')
	const dialog = await openRules()

	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '==', value: 'recipe' }]],
		}),
	)
})

test('turns a rule around when the condition is negated', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	renderAt('/field-groups')
	const dialog = await openRules()

	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Condition'))
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '!=', value: 'post' }]],
		}),
	)
})

test('drops the whole set when its last condition goes', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	renderAt('/field-groups')
	const dialog = await openRules()

	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByRole('button', { name: 'Remove' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() => expect(sent.body).toEqual({ location: [] }))
})

test('warns that a group with no rules appears nowhere', async () => {
	listing({ ...ON_POSTS, location: [] })
	renderAt('/field-groups')

	const dialog = await openRules()

	expect(within(dialog).getByText(/nowhere/i)).toBeInTheDocument()
})

test('gives a group with no rules a first one to fill in', async () => {
	const sent: { body?: unknown } = {}
	listing({ ...ON_POSTS, location: [] })
	recording(sent)
	renderAt('/field-groups')
	const dialog = await openRules()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '==', value: '*' }]],
		}),
	)
})

test('offers only the conditions the source declares', async () => {
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({
				items: [{ ...SOURCES.items[0], operators: ['=='] }],
			}),
		),
	)
	renderAt('/field-groups')
	const dialog = await openRules()

	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Condition'))

	expect(await screen.findByRole('option', { name: 'is' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'is not' })).not.toBeInTheDocument()
})

test('reports why the rules were turned away and keeps them on screen', async () => {
	server.use(
		http.patch('/api/groups/3', () =>
			HttpResponse.json(
				{ error: 'content: another group already claims those items', code: 'group_collides' },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openRules()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/claims/i)
	expect(screen.getByRole('dialog')).toBeInTheDocument()
})

test('abandons the edits when the dialog is dismissed', async () => {
	let hit = false
	server.use(
		http.patch('/api/groups/3', () => {
			hit = true
			return HttpResponse.json(ON_POSTS)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openRules()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(hit).toBe(false)
})

test('reports rules it could not offer sources for', async () => {
	server.use(http.get('/api/groups/params', () => new HttpResponse(null, { status: 500 })))
	renderAt('/field-groups')

	const dialog = await openRules()

	expect(within(dialog).getByText('The rules could not be loaded.')).toBeInTheDocument()
})

const ROLE_SOURCE = {
	source: 'user_role',
	operators: ['=='],
	values: [{ value: 'editor', label: 'Editors' }],
}

test('names a source the admin has no word for by the key the server gave it', async () => {
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({ items: [SOURCES.items[0], ROLE_SOURCE] }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openRules()

	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Source'))

	expect(await screen.findByRole('option', { name: 'user_role' })).toBeInTheDocument()
})

test('starts the condition and value afresh when the source changes', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({ items: [SOURCES.items[0], ROLE_SOURCE] }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openRules()

	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Source'))
	await userEvent.click(await screen.findByRole('option', { name: 'user_role' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'user_role', operator: '==', value: 'editor' }]],
		}),
	)
})

test('shows a value the registry no longer offers as the rule stored it', async () => {
	listing({
		...ON_POSTS,
		location: [[{ source: 'content_type', operator: '==', value: 'vanished' }]],
	})
	renderAt('/field-groups')

	const dialog = await openRules()
	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })

	expect(within(rule).getByLabelText('Value')).toHaveTextContent('vanished')
})

test('leaves a rule whose source is gone with nothing to choose from', async () => {
	listing({
		...ON_POSTS,
		location: [[{ source: 'retired', operator: '==', value: 'anything' }]],
	})
	renderAt('/field-groups')

	const dialog = await openRules()
	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Condition'))

	expect(screen.queryByRole('option', { name: 'is' })).not.toBeInTheDocument()
})

test('adds nothing while the server offers no source to build a rule from', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	server.use(http.get('/api/groups/params', () => HttpResponse.json({ items: [] })))
	renderAt('/field-groups')
	const dialog = await openRules()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
		}),
	)
})

test('adds nothing from a source that offers no value to match', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({ items: [{ source: 'content_type', operators: [], values: [] }] }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openRules()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
		}),
	)
})

test('edits only the set the rule belongs to', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	listing({
		...ON_POSTS,
		location: [
			[{ source: 'content_type', operator: '==', value: 'post' }],
			[{ source: 'content_type', operator: '==', value: 'recipe' }],
		],
	})
	renderAt('/field-groups')
	const dialog = await openRules()

	const second = within(dialog).getByRole('group', { name: 'Rule 1 of set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Remove' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
		}),
	)
})

test('adds a condition to the set it was asked from', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	listing({
		...ON_POSTS,
		location: [
			[{ source: 'content_type', operator: '==', value: 'post' }],
			[{ source: 'content_type', operator: '==', value: 'recipe' }],
		],
	})
	renderAt('/field-groups')
	const dialog = await openRules()

	const second = within(dialog).getByRole('group', { name: 'Rule set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Add condition' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[
					{ source: 'content_type', operator: '==', value: 'recipe' },
					{ source: 'content_type', operator: '==', value: '*' },
				],
			],
		}),
	)
})

test('changes the rule asked for and leaves the other set alone', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	listing({
		...ON_POSTS,
		location: [
			[{ source: 'content_type', operator: '==', value: 'post' }],
			[{ source: 'content_type', operator: '==', value: 'post' }],
		],
	})
	renderAt('/field-groups')
	const dialog = await openRules()

	const second = within(dialog).getByRole('group', { name: 'Rule 1 of set 2' })
	await userEvent.click(within(second).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[{ source: 'content_type', operator: '==', value: 'recipe' }],
			],
		}),
	)
})

test('changes the condition asked for and leaves its neighbour alone', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	listing({
		...ON_POSTS,
		location: [
			[
				{ source: 'content_type', operator: '==', value: 'post' },
				{ source: 'content_type', operator: '==', value: 'post' },
			],
		],
	})
	renderAt('/field-groups')
	const dialog = await openRules()

	const second = within(dialog).getByRole('group', { name: 'Rule 2 of set 1' })
	await userEvent.click(within(second).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [
				[
					{ source: 'content_type', operator: '==', value: 'post' },
					{ source: 'content_type', operator: '==', value: 'recipe' },
				],
			],
		}),
	)
})

test('leaves the rule untouched when the source picked offers no value', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({ items: [{ source: 'content_type', operators: [], values: [] }] }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openRules()

	const rule = within(dialog).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Source'))
	await userEvent.click(await screen.findByRole('option', { name: 'Content type' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
		}),
	)
})

test('names every condition an operator stands for', () => {
	expect(operatorLabel('==')).toBe('is')
	expect(operatorLabel('!=')).toBe('is not')
	expect(operatorLabel('~=')).toBe('~=')
})
