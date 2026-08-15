// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { chosenOf, fieldsQueryKey } from '../content/FieldsDialog'
import { createField, deleteField, listFields, renameField, slugifyKey } from '../content/types'

const COLOR = { key: 'color', label: 'Color', kind: 'text', many: false, required: false }

test('names the query a type is read for', () => {
	expect(fieldsQueryKey('post')).toEqual(['content-fields', 'post'])
})

test('keeps the choice held when the select reported nothing it offers', () => {
	const offered = [
		{ label: 'Text', value: 'text' },
		{ label: 'Number', value: 'number' },
	]

	expect(chosenOf(null, offered, offered[0])).toBe(offered[0])
	expect(chosenOf({ value: null }, offered, offered[0])).toBe(offered[0])
	expect(chosenOf({ value: 'nothing' }, offered, offered[0])).toBe(offered[0])
	expect(chosenOf({ value: 'number' }, offered, offered[0])).toBe(offered[1])
})

test('reduces a label to the key it declares', () => {
	expect(slugifyKey('Sold On')).toBe('sold-on')
	expect(slugifyKey('  Colour!  ')).toBe('colour')
})

test('reads the fields a type declares', async () => {
	server.use(http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })))

	const held = await listFields('post')

	expect(held).toEqual([
		{ key: 'color', label: 'Color', kind: 'text', relatesTo: '', many: false, required: false },
	])
})

test('carries the reason a field could not be read', async () => {
	server.use(
		http.get('/api/types/post/fields', () =>
			HttpResponse.json({ error: 'content: type not found' }, { status: 404 }),
		),
	)

	await expect(listFields('post')).rejects.toThrow('content: type not found')
})

test('carries the reason a field was refused', async () => {
	server.use(
		http.post('/api/types/post/fields', () =>
			HttpResponse.json({ error: 'content: field key taken' }, { status: 422 }),
		),
	)

	await expect(createField('post', { key: 'color', label: 'Color', kind: 'text' })).rejects.toThrow(
		'content: field key taken',
	)
})

test('carries the reason a rename was refused', async () => {
	server.use(
		http.patch('/api/types/post/fields/color', () =>
			HttpResponse.json({ error: 'content: field not found' }, { status: 404 }),
		),
	)

	await expect(renameField('post', 'color', 'Paint')).rejects.toThrow('content: field not found')
})

test('carries the reason a delete was refused', async () => {
	server.use(
		http.delete('/api/types/post/fields/color', () =>
			HttpResponse.json({ error: 'content: field not found' }, { status: 404 }),
		),
	)

	await expect(deleteField('post', 'color')).rejects.toThrow('content: field not found')
})

test('declares a relation field with the target it names', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types/post/fields', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...COLOR, kind: 'relation', relates_to: 'category', many: true })
		}),
	)

	const held = await createField('post', {
		key: 'categories',
		label: 'Categories',
		kind: 'relation',
		relatesTo: 'category',
		many: true,
	})

	expect(sent[0]).toMatchObject({ kind: 'relation', relates_to: 'category', many: true })
	expect(held.relatesTo).toBe('category')
})
