// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { groupsQueryKey } from '../content/groups'
import { operatorLabel } from '../content/RuleSets'
import type { FieldGroup } from '../content/groups'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { renderAt } from './render'
import { expectRuleSets } from './ruleSets'
import { expectReadOnly, findToast, waitBusy } from './typeFixtures'

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

const ON_POSTS = {
	id: 3,
	key: 'article-details',
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

/** Opens the page of the listed group by its title and answers with the details column holding its rules. */
async function openRules() {
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	await userEvent.click(within(table).getByRole('link', { name: 'Article details' }))
	return screen.findByRole('region', { name: 'Field group settings' })
}

/** Returns the Save button in the header of the group's page. */
function saveButton() {
	return within(screen.getByRole('main')).getByRole('button', { name: 'Save' })
}

/** Presses the Save button in the header of the group's page. */
async function save() {
	await userEvent.click(saveButton())
}

/** Leaves the group's page by its breadcrumb and drops the draft when asked. */
async function leaveDiscarding() {
	const trail = within(screen.getByRole('main')).getByRole('navigation', { name: 'Breadcrumbs' })
	await userEvent.click(within(trail).getByRole('link', { name: 'Field Groups' }))
	const asking = await screen.findByRole('alertdialog', { name: 'Discard changes?' })
	await userEvent.click(within(asking).getByRole('button', { name: 'Discard' }))
}

/** A type a plugin declared under the built-in page key, with the page's shipped names. */
const DECLARED_PAGE = { ...RECIPE_TYPE, key: 'page', singular_label: 'Page', plural_label: 'Pages', origin: 'shop' }

/** Serves the content type source with the page key among its values. */
function offeringPages() {
	const [offered] = SOURCES.items
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({ items: [{ ...offered, values: [...offered.values, { value: 'page', label: 'Pages' }] }] }),
		),
	)
}

/**
 * Opens the value select of the first rule in a Spanish admin.
 * @returns The rule the select belongs to.
 */
async function openSpanishValues() {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Grupos de campos' })
	await userEvent.click(within(table).getByRole('link', { name: 'Article details' }))
	const rule = await screen.findByRole('group', { name: 'Regla 1 del conjunto 1' })
	await userEvent.click(within(rule).getByLabelText('Valor'))
	return rule
}

/** Answers the group edit and records what the page sent, the stamp it held apart from the parts it edits. */
function recording(sent: { body?: unknown; stamp?: unknown }) {
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			const { updated_at: stamp, ...parts } = (await request.json()) as Record<string, unknown>
			sent.body = parts
			sent.stamp = stamp
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

	const details = await openRules()
	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })

	await waitFor(() => expect(within(rule).getByLabelText('Source')).toHaveTextContent('Content type'))
	expect(within(rule).getByLabelText('Condition')).toHaveTextContent('is')
	expect(within(rule).getByLabelText('Value')).toHaveTextContent('Posts')
})

test('offers the built-in types as rule values in the words of WordPress es_ES for a Spanish reader', async () => {
	const rule = await openSpanishValues()

	expect(await screen.findByRole('option', { name: 'Entradas' })).toBeInTheDocument()
	expect(within(rule).getByLabelText('Valor')).toHaveTextContent('Entradas')
	expect(screen.getByRole('option', { name: 'Recipes' })).toBeInTheDocument()
})

test('offers a type a plugin declared under a built-in key as a rule value by the name it declared', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, RECIPE_TYPE, DECLARED_PAGE] })))
	offeringPages()

	await openSpanishValues()

	expect(await screen.findByRole('option', { name: 'Pages' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Entradas' })).toBeInTheDocument()
})

test('names a rule value as the server sent it until the type list says who declared it', async () => {
	const types = gate()
	server.use(
		http.get('/api/types', async () => {
			await types.held
			return HttpResponse.json({ items: [POST_TYPE, RECIPE_TYPE, DECLARED_PAGE] })
		}),
	)
	offeringPages()

	await openSpanishValues()

	expect(await screen.findByRole('option', { name: 'Pages' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Páginas' })).not.toBeInTheDocument()
	types.release()
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

	const details = await openRules()

	expect(within(details).getByRole('group', { name: 'Rule set 1' })).toBeInTheDocument()
	expect(within(details).getByRole('group', { name: 'Rule set 2' })).toBeInTheDocument()
})

test('narrows a set by adding a condition every item has to meet', async () => {
	const sent: { body?: unknown } = {}
	recording(sent)
	renderAt('/field-groups')
	const details = await openRules()

	const set = within(details).getByRole('group', { name: 'Rule set 1' })
	await userEvent.click(within(set).getByRole('button', { name: 'Add condition' }))
	await save()

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
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	await save()

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
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	await save()

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
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Condition'))
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))
	await save()

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
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByRole('button', { name: 'Remove rule 1 of set 1' }))
	await save()

	await waitFor(() => expect(sent.body).toEqual({ location: [] }))
})

test('moves the focus to the rule taking a removed rule\'s place, then to the set and to Add rule set', async () => {
	listing({
		...ON_POSTS,
		location: [
			[
				{ source: 'content_type', operator: '==', value: 'post' },
				{ source: 'content_type', operator: '==', value: 'recipe' },
			],
			[{ source: 'content_type', operator: '==', value: 'recipe' }],
		],
	})
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Remove rule 1 of set 1' }))
	const taking = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	expect(taking).toHaveFocus()
	await waitFor(() => expect(within(taking).getByRole('combobox', { name: 'Value' })).toHaveTextContent('Recipes'))
	expect(within(details).queryByRole('group', { name: 'Rule 2 of set 1' })).not.toBeInTheDocument()
	await userEvent.click(within(details).getByRole('button', { name: 'Remove rule 1 of set 1' }))
	expect(within(details).getByRole('group', { name: 'Rule set 1' })).toHaveFocus()
	expect(within(details).queryByRole('group', { name: 'Rule set 2' })).not.toBeInTheDocument()
	await userEvent.click(within(details).getByRole('button', { name: 'Remove rule 1 of set 1' }))

	expect(within(details).getByRole('button', { name: 'Add rule set' })).toHaveFocus()
})

test.each([
	{
		place: 'rule',
		trashEffect: 'takes away the last rule of a set',
		location: [
			[
				{ source: 'content_type', operator: '==', value: 'post' },
				{ source: 'content_type', operator: '==', value: 'recipe' },
			],
		],
		trashButton: 'Remove rule 2 of set 1',
		focusedGroup: 'Rule 1 of set 1',
		removedGroup: 'Rule 2 of set 1',
	},
	{
		place: 'set',
		trashEffect: 'empties the last set',
		location: [
			[{ source: 'content_type', operator: '==', value: 'post' }],
			[{ source: 'content_type', operator: '==', value: 'recipe' }],
		],
		trashButton: 'Remove rule 1 of set 2',
		focusedGroup: 'Rule set 1',
		removedGroup: 'Rule set 2',
	},
])('moves the focus to the $place before once the trash $trashEffect', async (trashCase) => {
	listing({ ...ON_POSTS, location: trashCase.location })
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: trashCase.trashButton }))

	expect(within(details).getByRole('group', { name: trashCase.focusedGroup })).toHaveFocus()
	expect(within(details).queryByRole('group', { name: trashCase.removedGroup })).not.toBeInTheDocument()
})

test('leaves the focus where it sits as an add button or a trash that never took it is pressed', async () => {
	renderAt('/field-groups')
	const details = await openRules()
	const title = within(details).getByRole('textbox', { name: 'Title' })
	act(() => title.focus())

	fireEvent.click(within(details).getByRole('button', { name: 'Add condition' }))
	expect(within(details).getByRole('group', { name: 'Rule 2 of set 1' })).toBeInTheDocument()
	expect(title).toHaveFocus()
	fireEvent.click(within(details).getByRole('button', { name: 'Remove rule 1 of set 1' }))

	expect(within(details).queryByRole('group', { name: 'Rule 2 of set 1' })).not.toBeInTheDocument()
	expect(title).toHaveFocus()
})

test('sends the stamp the group held when the draft began, even after the groups are read again', async () => {
	const sent: { body?: unknown; stamp?: unknown } = {}
	recording(sent)
	const client = renderAt('/field-groups')
	const details = await openRules()
	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	listing({ ...ON_POSTS, updated_at: '2026-08-02T09:30:00Z' })
	await client.refetchQueries({ queryKey: groupsQueryKey })
	expect(client.getQueryData<FieldGroup[]>(groupsQueryKey)?.[0].updatedAt).toBe('2026-08-02T09:30:00Z')

	await save()

	await waitFor(() => expect(sent.stamp).toBe(ON_POSTS.updated_at))
})

test('warns that a group with no rules appears nowhere', async () => {
	listing({ ...ON_POSTS, location: [] })
	renderAt('/field-groups')

	const details = await openRules()

	expect(within(details).getByText(/nowhere/i)).toBeInTheDocument()
})

test('gives a group with no rules a first one to fill in', async () => {
	const sent: { body?: unknown } = {}
	listing({ ...ON_POSTS, location: [] })
	recording(sent)
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	await save()

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
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
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
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	await save()

	const said = 'content: another group already claims those items'
	expect(await within(screen.getByRole('main')).findByRole('alert')).toHaveTextContent(said)
	expect(within(details).getByRole('group', { name: 'Rule set 2' })).toBeInTheDocument()
	expect(await findToast(`Could not save the field group. ${said}`)).toBeInTheDocument()
})

test('abandons the edits when the page is left and they are discarded', async () => {
	let hit = false
	server.use(
		http.patch('/api/groups/3', () => {
			hit = true
			return HttpResponse.json(ON_POSTS)
		}),
	)
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	await leaveDiscarding()
	const reopened = await openRules()

	expect(within(reopened).queryByRole('group', { name: 'Rule set 2' })).not.toBeInTheDocument()
	expect(hit).toBe(false)
})

test('reports rules it could not offer sources for', async () => {
	server.use(http.get('/api/groups/params', () => new HttpResponse(null, { status: 500 })))
	renderAt('/field-groups')

	const details = await openRules()

	expect(within(details).getByText('The rules could not be loaded.')).toBeInTheDocument()
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
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
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
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Source'))
	await userEvent.click(await screen.findByRole('option', { name: 'user_role' }))
	await save()

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

	const details = await openRules()
	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })

	expect(within(rule).getByLabelText('Value')).toHaveTextContent('vanished')
})

test('leaves a rule whose source is gone with nothing to choose from', async () => {
	listing({
		...ON_POSTS,
		location: [[{ source: 'retired', operator: '==', value: 'anything' }]],
	})
	renderAt('/field-groups')

	const details = await openRules()
	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Condition'))

	expect(screen.queryByRole('option', { name: 'is' })).not.toBeInTheDocument()
})

test('adds nothing while the server offers no source to build a rule from', async () => {
	server.use(http.get('/api/groups/params', () => HttpResponse.json({ items: [] })))
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))

	expect(within(details).getByRole('button', { name: 'Add rule set' })).toHaveAttribute('aria-disabled', 'true')
	expect(within(details).queryByRole('group', { name: 'Rule set 2' })).not.toBeInTheDocument()
	expect(saveButton()).toHaveAttribute('aria-disabled', 'true')
})

test('adds nothing from a source that offers no value to match', async () => {
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({ items: [{ source: 'content_type', operators: [], values: [] }] }),
		),
	)
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))

	expect(within(details).queryByRole('group', { name: 'Rule set 2' })).not.toBeInTheDocument()
	expect(saveButton()).toHaveAttribute('aria-disabled', 'true')
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
	const details = await openRules()

	const second = within(details).getByRole('group', { name: 'Rule 1 of set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Remove rule 1 of set 2' }))
	await save()

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
	const details = await openRules()

	const second = within(details).getByRole('group', { name: 'Rule set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Add condition' }))
	await save()

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
	const details = await openRules()

	const second = within(details).getByRole('group', { name: 'Rule 1 of set 2' })
	await userEvent.click(within(second).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	await save()

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
	const details = await openRules()

	const second = within(details).getByRole('group', { name: 'Rule 2 of set 1' })
	await userEvent.click(within(second).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	await save()

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
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({ items: [{ source: 'content_type', operators: [], values: [] }] }),
		),
	)
	renderAt('/field-groups')
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Source'))
	await userEvent.click(await screen.findByRole('option', { name: 'Content type' }))

	expect(within(rule).getByLabelText('Value')).toHaveTextContent('post')
	expect(saveButton()).toHaveAttribute('aria-disabled', 'true')
})

test('names every condition an operator stands for', () => {
	expect(operatorLabel('==')).toBe('is')
	expect(operatorLabel('!=')).toBe('is not')
	expect(operatorLabel('<')).toBe('is below')
	expect(operatorLabel('>')).toBe('is above')
	expect(operatorLabel('contains')).toBe('holds')
	expect(operatorLabel('empty')).toBe('is empty')
	expect(operatorLabel('not_empty')).toBe('holds anything')
	expect(operatorLabel('~=')).toBe('~=')
	expect(operatorLabel('constructor')).toBe('constructor')
})

test('names each rule set above its rules and each rule by its place, each add button at its own width', async () => {
	renderAt('/field-groups')
	const details = await openRules()
	const add = within(details).getByRole('button', { name: 'Add rule set' })
	await waitFor(() => expect(add).not.toHaveAttribute('aria-disabled', 'true'))

	await userEvent.click(add)
	const second = within(details).getByRole('group', { name: 'Rule set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Add condition' }))

	expectRuleSets(details, [1, 2])
})

const MAKER = {
	key: 'maker',
	label: 'Maker',
	kind: 'relation',
	relates_to: 'post',
	many: false,
	required: false,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const CARS = {
	...ON_POSTS,
	id: 6,
	key: 'cars',
	title: 'Cars',
	position: 2,
	fields: [MAKER, { ...MAKER, key: 'rival', label: 'Rival', relates_to: 'recipe' }],
}

const VANS = {
	...ON_POSTS,
	id: 7,
	key: 'vans',
	title: 'Vans',
	position: 3,
	fields: [{ ...MAKER, key: 'driver', label: 'Driver', relates_to: 'recipe' }],
}

const READING = {
	...ON_POSTS,
	fields: [
		{
			...MAKER,
			key: 'linked-from',
			label: 'Linked from',
			kind: 'backlinks',
			relates_to: '',
			settings: { source_group: 'cars', source_field: ['maker'] },
		},
	],
}

/** Serves every given group from the listing endpoint. */
function listingAll(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}

test('offers no source to pick for a group without a Linked from field', async () => {
	listingAll([ON_POSTS, CARS])
	renderAt('/field-groups')

	const details = await openRules()

	expect(within(details).queryByRole('combobox', { name: 'Reads from' })).not.toBeInTheDocument()
})

test('shows the relation each Linked from field reads', async () => {
	listingAll([READING, CARS])
	renderAt('/field-groups')

	const details = await openRules()
	const reading = within(details).getByRole('group', { name: 'Linked from' })

	expect(within(reading).getByRole('combobox', { name: 'Reads from' })).toHaveTextContent('Cars')
	expect(within(reading).getByRole('combobox', { name: 'Through' })).toHaveTextContent('Maker')
})

test('keeps every rule and Linked from picker read only while the rules are saved', async () => {
	const answer = gate()
	listingAll([READING, CARS])
	server.use(
		http.patch('/api/groups/3', async () => {
			await answer.held
			return HttpResponse.json(READING)
		}),
	)
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	await save()
	await waitBusy(screen.getByRole('main'), 'Save')

	expectReadOnly(details)
	expect(within(details).getByRole('combobox', { name: 'Through' })).toHaveAttribute('aria-readonly', 'true')
	expect(within(details).getByRole('checkbox', { name: 'Active' })).toHaveAttribute('aria-readonly', 'true')
	for (const name of ['Add condition', /^Remove rule \d+ of set \d+$/, 'Add rule set']) {
		for (const button of within(details).getAllByRole('button', { name })) {
			expect(button).toHaveAttribute('aria-disabled', 'true')
		}
	}
	answer.release()
	await waitFor(() => expect(saveButton()).toHaveAttribute('aria-disabled', 'true'))
	expect(within(details).getByRole('textbox', { name: 'Title' })).not.toHaveAttribute('readonly')
})

test('points a Linked from field at the relation picked as the group moves', async () => {
	const sent: { body?: unknown } = {}
	listingAll([READING, CARS])
	recording(sent)
	renderAt('/field-groups')
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	const reading = within(details).getByRole('group', { name: 'Linked from' })
	await userEvent.click(within(reading).getByRole('combobox', { name: 'Through' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Rival' }))
	await save()

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [[{ source: 'content_type', operator: '==', value: 'recipe' }]],
			backlinks: {
				'linked-from': { source_group: 'cars', source_field: ['rival'], updated_at: '2026-08-01T10:00:00Z' },
			},
		}),
	)
})

test('points a Linked from field at the first relation of the group picked', async () => {
	const sent: { body?: unknown } = {}
	listingAll([READING, CARS, VANS])
	recording(sent)
	renderAt('/field-groups')
	const details = await openRules()

	const reading = within(details).getByRole('group', { name: 'Linked from' })
	await userEvent.click(within(reading).getByRole('combobox', { name: 'Reads from' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Vans' }))
	await save()

	await waitFor(() =>
		expect(sent.body).toEqual({
			backlinks: {
				'linked-from': { source_group: 'vans', source_field: ['driver'], updated_at: '2026-08-01T10:00:00Z' },
			},
		}),
	)
})

test('leaves a Linked from field nobody touched out of the save', async () => {
	const sent: { body?: unknown } = {}
	listingAll([READING, CARS])
	recording(sent)
	renderAt('/field-groups')
	const details = await openRules()

	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	await save()

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[{ source: 'content_type', operator: '==', value: '*' }],
			],
		}),
	)
})

test('shows a Linked from reading inside a container without pickers and leaves it out of the save', async () => {
	const sent: { body?: unknown } = {}
	listingAll([
		{
			...READING,
			fields: [{ ...READING.fields[0], settings: { source_group: 'cars', source_field: ['specs', 'engine'] } }],
		},
		CARS,
	])
	recording(sent)
	renderAt('/field-groups')
	const details = await openRules()

	const reading = within(details).getByRole('group', { name: 'Linked from' })
	expect(within(reading).getByText(/inside a container/i)).toBeInTheDocument()
	expect(within(reading).queryByRole('combobox', { name: 'Reads from' })).not.toBeInTheDocument()
	await userEvent.click(within(details).getByRole('button', { name: 'Add rule set' }))
	await save()

	await waitFor(() =>
		expect(sent.body).toEqual({
			location: [
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[{ source: 'content_type', operator: '==', value: '*' }],
			],
		}),
	)
})

test('keeps the draft on a save someone else beat, then takes the rules and sources they stored', async () => {
	let listed = [READING, CARS, VANS]
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: listed })))
	server.use(
		http.patch('/api/groups/3', () => {
			const moved = { ...READING.fields[0], settings: { source_group: 'vans', source_field: ['driver'] } }
			const recipes = [[{ source: 'content_type', operator: '==', value: 'recipe' }]]
			listed = [{ ...READING, location: recipes, fields: [moved], updated_at: '2026-08-02T09:30:00Z' }, CARS, VANS]
			return HttpResponse.json(
				{ error: 'content: conflicting update', code: 'content_stale_update' },
				{ status: 409 },
			)
		}),
	)
	renderAt('/field-groups')
	const details = await openRules()

	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Any content type' }))
	const reading = within(details).getByRole('group', { name: 'Linked from' })
	await userEvent.click(within(reading).getByRole('combobox', { name: 'Through' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Rival' }))
	await save()

	const main = screen.getByRole('main')
	expect(await within(main).findByText('Someone saved Article details after you opened it.')).toBeInTheDocument()
	expect(within(rule).getByLabelText('Value')).toHaveTextContent('Any content type')
	await userEvent.click(within(main).getByRole('button', { name: 'Load saved version' }))

	await waitFor(() => expect(within(rule).getByLabelText('Value')).toHaveTextContent('Recipes'))
	expect(within(reading).getByRole('combobox', { name: 'Reads from' })).toHaveTextContent('Vans')
	expect(within(details).getAllByText('Changed elsewhere too. Your edit was not kept.')).toHaveLength(2)
	expect(saveButton()).toHaveAttribute('aria-disabled', 'true')
})

test('forgets a Linked from pick when the page is left and the pick is discarded', async () => {
	listingAll([READING, CARS])
	renderAt('/field-groups')
	const details = await openRules()

	const reading = within(details).getByRole('group', { name: 'Linked from' })
	await userEvent.click(within(reading).getByRole('combobox', { name: 'Through' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Rival' }))
	await leaveDiscarding()
	const reopened = await openRules()

	const again = within(reopened).getByRole('group', { name: 'Linked from' })
	expect(within(again).getByRole('combobox', { name: 'Through' })).toHaveTextContent('Maker')
})
