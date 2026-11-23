// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { shadowings } from '../content/GroupsScreen'
import { typesQueryKey } from '../content/nav'
import { catalogFor } from '../i18n/catalog'
import { listenAt } from './announcements'
import { fieldRow } from './groupFieldsFixtures'
import { renderAt } from './render'
import type { Location } from '../content/groups'

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

test('warns of an overlap without speaking it, as WordPress leaves a static notice silent', async () => {
	const polite = listenAt('polite')
	const assertive = listenAt('assertive')
	listing([DETAILS, EXTRAS])
	renderAt('/field-groups')

	await screen.findByText('subtitle is served by Article details, so Extras does not show it on Posts.')

	expect(polite()).toEqual([])
	expect(assertive()).toEqual([])
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

/**
 * Returns the main work of the page of a group once it carries the group's title.
 * @param title - The title of the group.
 * @returns The main region.
 */
async function groupPage(title: string) {
	const main = await screen.findByRole('main')
	await within(main).findByRole('heading', { level: 2, name: title })
	return main
}

const MORE = { ...EXTRAS, id: 5, key: 'more', title: 'More', position: 3 }

test('warns on the page of a group that loses a field and marks the field it loses', async () => {
	listing([DETAILS, EXTRAS])
	renderAt('/field-groups/4')

	const main = await groupPage('Extras')

	expect(
		within(main).getByText('subtitle is served by Article details, so Extras does not show it on Posts.'),
	).toBeInTheDocument()
	const lost = await fieldRow(main, 'Subtitle')
	expect(within(lost).getByText('Shadowed')).toBeInTheDocument()
})

test('warns on the page of a group of every overlap it serves and leaves its own fields unmarked', async () => {
	listing([DETAILS, EXTRAS, MORE])
	renderAt('/field-groups/3')

	const main = await groupPage('Article details')

	expect(
		within(main).getByText('subtitle is served by Article details, so Extras does not show it on Posts.'),
	).toBeInTheDocument()
	expect(
		within(main).getByText('subtitle is served by Article details, so More does not show it on Posts.'),
	).toBeInTheDocument()
	expect(within(await fieldRow(main, 'Subtitle')).queryByText('Shadowed')).toBeNull()
})

test('keeps the page of a group quiet about an overlap between two other groups', async () => {
	listing([DETAILS, EXTRAS, { ...MORE, fields: [{ ...SUBTITLE, key: 'teaser', label: 'Teaser' }] }])
	renderAt('/field-groups/5')

	const main = await groupPage('More')

	expect(within(main).queryByText(/is served by/)).not.toBeInTheDocument()
	expect(within(main).queryByText('Shadowed')).not.toBeInTheDocument()
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
	).toEqual([{ key: 'a', winner: 'First', winnerID: 1, loser: 'Second', loserID: 2, where: ['Posts'] }])
	expect(
		shadowings([holding(1, 'First', onPost, ['a']), holding(2, 'Second', anywhere, ['a'])], types),
	).toEqual([{ key: 'a', winner: 'First', winnerID: 1, loser: 'Second', loserID: 2, where: ['Posts'] }])
	expect(
		shadowings([holding(1, 'First', anywhere, ['a']), holding(2, 'Second', anywhere, ['a'])], types),
	).toEqual([{ key: 'a', winner: 'First', winnerID: 1, loser: 'Second', loserID: 2, where: ['Posts', 'Recipes'] }])
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
		{ key: 'a', winner: 'First', winnerID: 1, loser: 'Second', loserID: 2, where: ['Posts'] },
		{ key: 'a', winner: 'First', winnerID: 1, loser: 'Third', loserID: 3, where: ['Posts'] },
	])
	expect(
		shadowings([holding(1, 'First', onPost, ['a', 'b']), holding(2, 'Second', onPost, ['b'])], types),
	).toEqual([{ key: 'b', winner: 'First', winnerID: 1, loser: 'Second', loserID: 2, where: ['Posts'] }])
})

test('names the built-in type an overlap sits on in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	const types = [
		{ key: 'post', pluralLabel: 'Posts' },
		{ key: 'recipe', pluralLabel: 'Recipes' },
	]
	const anywhere = [[{ source: 'content_type', operator: '==', value: '*' }]]
	const first = { id: 1, title: 'First', active: true, location: anywhere, fields: [{ key: 'a' }] }
	const second = { id: 2, title: 'Second', active: true, location: anywhere, fields: [{ key: 'a' }] }

	expect(shadowings([first, second], types)).toEqual([
		{ key: 'a', winner: 'First', winnerID: 1, loser: 'Second', loserID: 2, where: ['Entradas', 'Recipes'] },
	])
})
