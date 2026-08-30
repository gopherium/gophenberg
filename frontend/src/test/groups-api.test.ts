// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import {
	createFieldInGroup,
	createGroup,
	deleteFieldInGroup,
	deleteGroup,
	listGroups,
	listRuleSources,
	moveField,
	renameFieldInGroup,
	reorderFieldsInGroup,
	reorderGroups,
	setFieldRequiredInGroup,
	setFieldSettingsInGroup,
	updateGroup,
} from '../content/groups'

const SUBTITLE_ROW = {
	key: 'subtitle',
	label: 'Subtitle',
	kind: 'text',
	many: false,
	required: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const GROUP_ROW = {
	id: 3,
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [SUBTITLE_ROW],
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

test('reads every stored group with its rules and fields', async () => {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [GROUP_ROW] })))

	const groups = await listGroups()

	expect(groups).toHaveLength(1)
	expect(groups[0].id).toBe(3)
	expect(groups[0].title).toBe('Article details')
	expect(groups[0].active).toBe(true)
	expect(groups[0].location).toEqual([[{ source: 'content_type', operator: '==', value: 'post' }]])
	expect(groups[0].fields[0]).toMatchObject({ key: 'subtitle', label: 'Subtitle', required: true })
})

test('reports groups it could not read', async () => {
	server.use(http.get('/api/groups', () => new HttpResponse(null, { status: 500 })))

	await expect(listGroups()).rejects.toThrow(/500/)
})

test('creates a group carrying its title and rules', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(GROUP_ROW, { status: 201 })
		}),
	)

	const created = await createGroup('Article details', [
		[{ source: 'content_type', operator: '==', value: 'post' }],
	])

	expect(sent).toEqual({
		title: 'Article details',
		location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	})
	expect(created.id).toBe(3)
})

test('carries the reason a group was refused', async () => {
	server.use(
		http.post('/api/groups', () =>
			HttpResponse.json(
				{ error: 'content: a field group needs a title', code: 'group_title_required' },
				{ status: 422 },
			),
		),
	)

	await expect(createGroup(' ', [])).rejects.toThrow(/title/i)
})

test('edits only what the caller names', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...GROUP_ROW, active: false })
		}),
	)

	const updated = await updateGroup(3, { active: false })

	expect(sent).toEqual({ active: false })
	expect(updated.active).toBe(false)
})

test('carries an edited title and rules together', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(GROUP_ROW)
		}),
	)

	await updateGroup(3, { title: 'Renamed', location: [] })

	expect(sent).toEqual({ title: 'Renamed', location: [] })
})

test('removes a group', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)

	await deleteGroup(3)

	expect(hit).toBe(true)
})

test('stores the order the groups are read in', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [GROUP_ROW] })
		}),
	)

	const ordered = await reorderGroups([3, 1])

	expect(sent).toEqual({ order: [3, 1] })
	expect(ordered[0].id).toBe(3)
})

test('reads the sources a rule may name with their choices', async () => {
	server.use(
		http.get('/api/groups/params', () =>
			HttpResponse.json({
				items: [
					{
						source: 'content_type',
						operators: ['==', '!='],
						values: [{ value: 'post', label: 'Posts' }],
					},
				],
			}),
		),
	)

	const sources = await listRuleSources()

	expect(sources).toHaveLength(1)
	expect(sources[0].source).toBe('content_type')
	expect(sources[0].operators).toEqual(['==', '!='])
	expect(sources[0].values).toEqual([{ value: 'post', label: 'Posts' }])
})

test('declares a field inside a group', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE_ROW, { status: 201 })
		}),
	)

	const declared = await createFieldInGroup(3, {
		key: 'subtitle',
		label: 'Subtitle',
		kind: 'text',
		required: true,
	})

	expect(sent).toMatchObject({ key: 'subtitle', label: 'Subtitle', kind: 'text', required: true })
	expect(declared.key).toBe('subtitle')
})

test('renames a field inside its group', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE_ROW, label: 'Renamed' })
		}),
	)

	const renamed = await renameFieldInGroup(3, 'subtitle', 'Renamed', '2026-08-01T10:00:00Z')

	expect(sent).toEqual({ label: 'Renamed', updated_at: '2026-08-01T10:00:00Z' })
	expect(renamed.label).toBe('Renamed')
})

test('stores whether a field gates publishing', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE_ROW, required: false })
		}),
	)

	const eased = await setFieldRequiredInGroup(3, 'subtitle', false, '2026-08-01T10:00:00Z')

	expect(sent).toEqual({ required: false, updated_at: '2026-08-01T10:00:00Z' })
	expect(eased.required).toBe(false)
})

test('stores the settings a field carries', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE_ROW, settings: { maxlength: 80 } })
		}),
	)

	const bounded = await setFieldSettingsInGroup(3, 'subtitle', { maxlength: 80 }, '2026-08-01T10:00:00Z')

	expect(sent).toEqual({ settings: { maxlength: 80 }, updated_at: '2026-08-01T10:00:00Z' })
	expect(bounded.settings).toEqual({ maxlength: 80 })
})

test('stores the declaration order of a group', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/3/fields/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [SUBTITLE_ROW] })
		}),
	)

	const ordered = await reorderFieldsInGroup(3, ['subtitle'])

	expect(sent).toEqual({ order: ['subtitle'] })
	expect(ordered[0].key).toBe('subtitle')
})

test('removes a field from its group', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)

	await deleteFieldInGroup(3, 'subtitle')

	expect(hit).toBe(true)
})

test.each([
	['updateGroup', () => updateGroup(3, { title: 'Renamed' }), 'patch', '/api/groups/3'],
	['deleteGroup', () => deleteGroup(3), 'delete', '/api/groups/3'],
	['reorderGroups', () => reorderGroups([3]), 'put', '/api/groups/order'],
	[
		'createFieldInGroup',
		() => createFieldInGroup(3, { key: 'a', label: 'A', kind: 'text' }),
		'post',
		'/api/groups/3/fields',
	],
	[
		'renameFieldInGroup',
		() => renameFieldInGroup(3, 'a', 'B', '2026-08-01T10:00:00Z'),
		'patch',
		'/api/groups/3/fields/a',
	],
	['reorderFieldsInGroup', () => reorderFieldsInGroup(3, ['a']), 'put', '/api/groups/3/fields/order'],
	['deleteFieldInGroup', () => deleteFieldInGroup(3, 'a'), 'delete', '/api/groups/3/fields/a'],
	['moveField', () => moveField(3, 'a', 7), 'post', '/api/groups/3/fields/a/move'],
] as const)('carries the reason %s was refused', async (_name, run, method, path) => {
	server.use(
		http[method](path, () =>
			HttpResponse.json({ error: 'content: field group not found', code: 'group_not_found' }, { status: 404 }),
		),
	)

	await expect(run()).rejects.toThrow(/group/i)
})

test('answers something readable when an error response carries no JSON', async () => {
	server.use(http.delete('/api/groups/3', () => new HttpResponse('gone wrong', { status: 500 })))

	await expect(deleteGroup(3)).rejects.toThrow()
})

test('reports rule sources it could not read', async () => {
	server.use(http.get('/api/groups/params', () => new HttpResponse(null, { status: 500 })))

	await expect(listRuleSources()).rejects.toThrow(/500/)
})

test('carries a field into another group', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE_ROW)
		}),
	)

	const moved = await moveField(3, 'subtitle', 7)

	expect(sent).toEqual({ to_group: 7 })
	expect(moved.key).toBe('subtitle')
})
