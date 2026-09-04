// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, within } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { shadowings } from '../content/GroupsScreen'
import { typesQueryKey } from '../content/nav'
import { renderAt } from './render'
import type { Location } from '../content/groups'

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

const SUBTITLE = {
	key: 'subtitle',
	label: 'Subtitle',
	kind: 'text',
	many: false,
	required: false,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const DETAILS = {
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

const EXTRAS = {
	...DETAILS,
	id: 4,
	key: 'extras',
	title: 'Extras',
	position: 2,
}

/**
 * Serves the given groups from the listing endpoint.
 * @param groups - The groups the listing answers with.
 */
function listing(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, RECIPE_TYPE] })))
})

test('warns which group serves a key another group also holds', async () => {
	listing([DETAILS, EXTRAS])
	renderAt('/field-groups')

	await screen.findByRole('region', { name: 'Field Groups' })

	expect(
		screen.getByText('subtitle is served by Article details, so Extras does not show it on Posts.'),
	).toBeInTheDocument()
})

test('marks the losing group and leaves the winning one alone', async () => {
	listing([DETAILS, EXTRAS])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	const losing = within(table).getByRole('row', { name: /Extras/ })
	expect(within(losing).getByText('Shadowed')).toBeInTheDocument()
	const winning = within(table).getByRole('row', { name: /Article details/ })
	expect(within(winning).queryByText('Shadowed')).not.toBeInTheDocument()
})

test('stays quiet while the groups reach different content', async () => {
	listing([
		DETAILS,
		{ ...EXTRAS, location: [[{ source: 'content_type', operator: '==', value: 'recipe' }]] },
	])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	expect(screen.queryByText(/is served by/)).not.toBeInTheDocument()
	expect(within(table).queryByText('Shadowed')).not.toBeInTheDocument()
})

test('stays quiet while the losing group is inactive', async () => {
	listing([DETAILS, { ...EXTRAS, active: false }])
	renderAt('/field-groups')

	await screen.findByRole('region', { name: 'Field Groups' })

	expect(screen.queryByText(/is served by/)).not.toBeInTheDocument()
})

test('says the overlaps are unknown while the content types are out of reach', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => new HttpResponse(null, { status: 500 })))
	listing([DETAILS, EXTRAS])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	expect(
		screen.getByText(
			'The content types could not be loaded, so where these groups appear and which fields they shadow are not shown.',
		),
	).toBeInTheDocument()
	expect(within(table).queryByText('Shadowed')).not.toBeInTheDocument()
})

test('says the overlaps may be out of date when the types cannot be refreshed', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	listing([DETAILS, EXTRAS])
	const client = renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	expect(within(table).getByText('Shadowed')).toBeInTheDocument()

	server.use(http.get('/api/types', () => new HttpResponse(null, { status: 500 })))
	await act(async () => {
		await client.invalidateQueries({ queryKey: typesQueryKey })
	})

	expect(
		await screen.findByText(
			'The content types could not be refreshed, so where these groups appear and what they shadow may be out of date.',
		),
	).toBeInTheDocument()
	expect(screen.getByText('Shadowed')).toBeInTheDocument()
	expect(screen.getByText(/is served by/)).toBeInTheDocument()
})

test('names the overlaps a listing carries from its rules alone', () => {
	const types = [
		{ key: 'post', pluralLabel: 'Posts' },
		{ key: 'recipe', pluralLabel: 'Recipes' },
	]
	/**
	 * Returns a group the overlap walk can read.
	 * @param id - The identifier the group carries.
	 * @param title - The name the group is listed under.
	 * @param location - The rules placing the group.
	 * @param keys - The field keys the group holds.
	 * @param active - Whether the group is active.
	 * @returns The group to walk.
	 */
	const holding = (id: number, title: string, location: Location, keys: string[], active = true) => ({
		id,
		title,
		active,
		location,
		fields: keys.map((key) => ({ key })),
	})
	const onPost = [[{ source: 'content_type', operator: '==', value: 'post' }]]
	const anywhere = [[{ source: 'content_type', operator: '==', value: '*' }]]
	const notPost = [[{ source: 'content_type', operator: '!=', value: 'post' }]]

	expect(shadowings([], types)).toEqual([])
	expect(
		shadowings([holding(1, 'First', onPost, ['a']), holding(2, 'Second', onPost, ['a'])], types),
	).toEqual([{ key: 'a', winner: 'First', loser: 'Second', loserID: 2, where: ['Posts'] }])
	expect(
		shadowings([holding(1, 'First', onPost, ['a']), holding(2, 'Second', anywhere, ['a'])], types),
	).toEqual([{ key: 'a', winner: 'First', loser: 'Second', loserID: 2, where: ['Posts'] }])
	expect(
		shadowings([holding(1, 'First', anywhere, ['a']), holding(2, 'Second', anywhere, ['a'])], types),
	).toEqual([{ key: 'a', winner: 'First', loser: 'Second', loserID: 2, where: ['Posts', 'Recipes'] }])
	expect(
		shadowings([holding(1, 'First', onPost, ['a']), holding(2, 'Second', notPost, ['a'])], types),
	).toEqual([])
	expect(
		shadowings(
			[holding(1, 'First', [[{ source: 'elsewhere', operator: '==', value: 'x' }]], ['a']),
				holding(2, 'Second', onPost, ['a'])],
			types,
		),
	).toEqual([])
	expect(
		shadowings([holding(1, 'First', [], ['a']), holding(2, 'Second', onPost, ['a'])], types),
	).toEqual([])
	expect(
		shadowings([holding(1, 'First', onPost, ['a'], false), holding(2, 'Second', onPost, ['a'])], types),
	).toEqual([])
	expect(
		shadowings(
			[holding(1, 'First', onPost, ['a']), holding(2, 'Second', onPost, ['a']),
				holding(3, 'Third', onPost, ['a'])],
			types,
		),
	).toEqual([
		{ key: 'a', winner: 'First', loser: 'Second', loserID: 2, where: ['Posts'] },
		{ key: 'a', winner: 'First', loser: 'Third', loserID: 3, where: ['Posts'] },
	])
	expect(
		shadowings([holding(1, 'First', onPost, ['a', 'b']), holding(2, 'Second', onPost, ['b'])], types),
	).toEqual([{ key: 'b', winner: 'First', loser: 'Second', loserID: 2, where: ['Posts'] }])
})
