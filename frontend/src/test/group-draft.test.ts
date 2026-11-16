// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import {
	changedParts,
	draftOf,
	editedParts,
	editOf,
	linkPart,
	rebased,
	withLivePicks,
	withPick,
} from '../content/groupDraft'
import type { FieldGroup, Location } from '../content/groups'
import type { ContentField } from '../content/types'

const READ_STAMP = '2026-08-01T10:00:00Z'

const NEWER_STAMP = '2026-08-02T09:30:00Z'

const ON_POSTS: Location = [[{ source: 'content_type', operator: '==', value: 'post' }]]

const ON_RECIPES: Location = [[{ source: 'content_type', operator: '==', value: 'recipe' }]]

const ANYWHERE: Location = [[{ source: 'content_type', operator: '==', value: '*' }]]

const MAKER: ContentField = {
	key: 'maker',
	label: 'Maker',
	kind: 'relation',
	relatesTo: 'post',
	many: false,
	required: false,
	settings: {},
	fields: [],
	updatedAt: READ_STAMP,
}

const LINKED: ContentField = {
	...MAKER,
	key: 'linked-from',
	label: 'Linked from',
	kind: 'backlinks',
	relatesTo: '',
	settings: { source_group: 'cars', source_field: ['maker'] },
}

const BASE: FieldGroup = {
	id: 3,
	key: 'article-details',
	title: 'Article details',
	location: ON_POSTS,
	position: 1,
	active: true,
	fields: [LINKED],
	createdAt: READ_STAMP,
	updatedAt: READ_STAMP,
}

const CARS: FieldGroup = {
	...BASE,
	id: 6,
	key: 'cars',
	title: 'Cars',
	position: 2,
	fields: [MAKER, { ...MAKER, key: 'rival', label: 'Rival' }],
}

const VANS: FieldGroup = {
	...BASE,
	id: 7,
	key: 'vans',
	title: 'Vans',
	position: 3,
	fields: [{ ...MAKER, key: 'driver', label: 'Driver' }],
}

const GROUPS = [BASE, CARS, VANS]

/**
 * Returns the group with its Linked from field reading another source and carrying another stamp.
 * @param group - The group to change.
 * @param source - The settings naming the source the field reads.
 * @param stamp - The stamp the field carries.
 * @returns The changed group.
 */
function linkedAs(group: FieldGroup, source: Record<string, unknown>, stamp: string): FieldGroup {
	return { ...group, fields: [{ ...LINKED, settings: source, updatedAt: stamp }] }
}

test('names no part of a draft that holds the group as it was read', () => {
	expect(changedParts(BASE, draftOf(BASE))).toEqual([])
})

test('names only the parts a draft changed', () => {
	const draft = draftOf(BASE)

	expect(changedParts(BASE, { ...draft, title: 'Car details' })).toEqual(['title'])
	expect(changedParts(BASE, { ...draft, active: false })).toEqual(['active'])
	expect(changedParts(BASE, { ...draft, location: ON_RECIPES })).toEqual(['location'])
	expect(changedParts(BASE, { ...draft, title: 'Car details', location: ON_RECIPES })).toEqual(['title', 'location'])
})

test('names nothing once a change is undone', () => {
	const changed = { ...draftOf(BASE), title: 'Car details', location: ON_RECIPES }

	expect(changedParts(BASE, { ...changed, title: BASE.title, location: [[...ON_POSTS[0]]] })).toEqual([])
})

test('names a Linked from field picked to read another relation, and none picked back to its own', () => {
	const picked = withPick(draftOf(BASE), LINKED, { group: 'cars', field: 'rival' })

	expect(changedParts(BASE, picked)).toEqual([linkPart('linked-from')])
	expect(changedParts(BASE, withPick(picked, LINKED, { group: 'cars', field: 'maker' }))).toEqual([])
})

test('sends only the changed parts of a draft', () => {
	const draft = { ...draftOf(BASE), title: 'Car details' }

	expect(editOf(draft, changedParts(BASE, draft), BASE, GROUPS)).toEqual({
		title: 'Car details',
		active: undefined,
		location: undefined,
		backlinks: undefined,
	})
})

test('sends a pick with the newest stamp its field was read with while its source reads as when picked', () => {
	const draft = withPick(draftOf(BASE), LINKED, { group: 'vans', field: '' })
	const renamed = linkedAs(BASE, LINKED.settings, NEWER_STAMP)

	expect(editOf(draft, changedParts(BASE, draft), renamed, GROUPS).backlinks).toEqual({
		'linked-from': { source_group: 'vans', source_field: ['driver'], updated_at: NEWER_STAMP },
	})
})

test('sends a pick with the stamp it was made at once its field reads another source', () => {
	const draft = withPick(draftOf(BASE), LINKED, { group: 'cars', field: 'rival' })
	const repointed = linkedAs(BASE, { source_group: 'vans', source_field: ['driver'] }, NEWER_STAMP)

	expect(editOf(draft, changedParts(BASE, draft), repointed, GROUPS).backlinks).toEqual({
		'linked-from': { source_group: 'cars', source_field: ['rival'], updated_at: READ_STAMP },
	})
})

test('keeps the stamp and the source of the first pick of a field through the picks after it', () => {
	const first = withPick(draftOf(BASE), LINKED, { group: 'vans', field: '' })
	const renamed = { ...LINKED, label: 'Linked from cars', updatedAt: NEWER_STAMP }
	const second = withPick(first, renamed, { group: 'cars', field: 'rival' })

	expect(second.links['linked-from']).toEqual({
		group: 'cars',
		field: 'rival',
		was: { group: 'cars', field: 'maker' },
		updatedAt: READ_STAMP,
	})
})

test('keeps a part only the draft changed and takes a part only the saved version changed', () => {
	const draft = { ...draftOf(BASE), title: 'Car details' }
	const saved = { ...BASE, active: false, updatedAt: NEWER_STAMP }

	const moved = rebased(BASE, draft, saved)

	expect(moved.draft).toEqual({ ...draftOf(saved), title: 'Car details' })
	expect(moved.lost).toEqual([])
	expect(changedParts(saved, moved.draft)).toEqual(['title'])
})

test('takes the saved value of a part both changed and names it', () => {
	const draft = { ...draftOf(BASE), title: 'Car details', location: ANYWHERE }
	const saved = { ...BASE, location: ON_RECIPES, updatedAt: NEWER_STAMP }

	const moved = rebased(BASE, draft, saved)

	expect(moved.draft.location).toEqual(ON_RECIPES)
	expect(moved.draft.title).toBe('Car details')
	expect(moved.lost).toEqual(['location'])
})

test('names no part both changed the same way', () => {
	const draft = { ...draftOf(BASE), location: ON_RECIPES }
	const saved = { ...BASE, location: ON_RECIPES, updatedAt: NEWER_STAMP }

	expect(rebased(BASE, draft, saved).lost).toEqual([])
})

test('keeps a pick only the draft changed on the source and stamp the saved version holds', () => {
	const draft = withPick(draftOf(BASE), LINKED, { group: 'cars', field: 'rival' })
	const saved = { ...linkedAs(BASE, LINKED.settings, NEWER_STAMP), updatedAt: NEWER_STAMP }

	const moved = rebased(BASE, draft, saved)

	expect(moved.draft.links['linked-from']).toEqual({
		group: 'cars',
		field: 'rival',
		was: { group: 'cars', field: 'maker' },
		updatedAt: NEWER_STAMP,
	})
	expect(moved.lost).toEqual([])
})

test('drops a pick whose field the saved version points elsewhere too, and names it', () => {
	const draft = withPick(draftOf(BASE), LINKED, { group: 'cars', field: 'rival' })
	const saved = { ...linkedAs(BASE, { source_group: 'vans', source_field: ['driver'] }, NEWER_STAMP) }

	const moved = rebased(BASE, draft, saved)

	expect(moved.draft.links).toEqual({})
	expect(moved.lost).toEqual([linkPart('linked-from')])
})

test('keeps only the picks whose field the group still reads through Linked from', () => {
	const draft = withPick({ ...draftOf(BASE), title: 'Car details' }, LINKED, { group: 'cars', field: 'rival' })

	expect(withLivePicks(draft, BASE)).toEqual(draft)
	expect(withLivePicks(draft, { ...BASE, fields: [] })).toEqual({ ...draft, links: {} })
	expect(withLivePicks(draft, { ...BASE, fields: [{ ...LINKED, kind: 'text' }] })).toEqual({ ...draft, links: {} })
})

test('names the parts one draft holds otherwise than another', () => {
	const draft = draftOf(BASE)
	const picked = withPick(draft, LINKED, { group: 'cars', field: 'rival' })

	expect(editedParts(draft, draft)).toEqual([])
	expect(editedParts(draft, { ...draft, title: 'Car details', active: false })).toEqual(['title', 'active'])
	expect(editedParts(draft, { ...draft, location: ON_RECIPES })).toEqual(['location'])
	expect(editedParts(draft, picked)).toEqual([linkPart('linked-from')])
	expect(editedParts(picked, withPick(picked, LINKED, { group: 'cars', field: 'maker' }))).toEqual([
		linkPart('linked-from'),
	])
	expect(editedParts(draft, withPick(draft, LINKED, { group: 'cars', field: 'maker' }))).toEqual([])
})

test('drops a pick whose field the saved version no longer holds', () => {
	const draft = withPick(draftOf(BASE), LINKED, { group: 'cars', field: 'rival' })

	const moved = rebased(BASE, draft, { ...BASE, fields: [], updatedAt: NEWER_STAMP })

	expect(moved.draft.links).toEqual({})
	expect(moved.lost).toEqual([])
})

const UNKEPT = [
	{
		picked: 'a pick pointed back to the source its field read',
		draft: withPick(draftOf(BASE), LINKED, { group: 'cars', field: 'maker' }),
		saved: linkedAs(BASE, LINKED.settings, NEWER_STAMP),
	},
	{
		picked: 'a pick whose saved field reads through more than one relation',
		draft: withPick(draftOf(BASE), LINKED, { group: 'cars', field: 'rival' }),
		saved: linkedAs(BASE, { source_group: 'cars', source_field: ['maker', 'driver'] }, NEWER_STAMP),
	},
]

test.each(UNKEPT)('drops $picked once the draft moves onto the saved version, naming no part lost', (asked) => {
	const moved = rebased(BASE, asked.draft, asked.saved)

	expect(moved.draft.links).toEqual({})
	expect(moved.lost).toEqual([])
})
