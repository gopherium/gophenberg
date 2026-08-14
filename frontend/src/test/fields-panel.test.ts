// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { clearedEdits, editableFields, fieldDescriptors } from '../content/FieldsPanel'
import type { ContentField } from '../content/types'

/**
 * Returns a declared field of the given kind.
 * @param kind - The kind the field holds.
 * @param required - Whether publishing demands a value.
 * @returns The declared field.
 */
function declared(kind: string, required = false): ContentField {
	return { key: `a-${kind}`, label: kind, kind, relatesTo: '', many: false, required }
}

test('writes an emptied control as the value that clears its key', () => {
	expect(clearedEdits({ doors: undefined, 'sold-on': '', color: 'red', boxed: false })).toEqual({
		doors: null,
		'sold-on': null,
		color: 'red',
		boxed: false,
	})
})

test('keeps a zero, which is a value rather than an empty control', () => {
	expect(clearedEdits({ doors: 0 })).toEqual({ doors: 0 })
})

test('renders the kinds a control covers', () => {
	const held = [declared('text'), declared('number'), declared('boolean'), declared('date')]

	expect(editableFields(held)).toHaveLength(4)
})

test('leaves the kinds a picker owns to the part that builds them', () => {
	const held = [declared('media'), declared('relation'), declared('text')]

	expect(editableFields(held).map((field) => field.kind)).toEqual(['text'])
})

test('gives every kind its own field type', () => {
	const held = [declared('text'), declared('number'), declared('boolean'), declared('date')]

	expect(fieldDescriptors(held).map((descriptor) => descriptor.type)).toEqual([
		'text',
		'number',
		'boolean',
		'date',
	])
})

test('marks a required field so the form can say so', () => {
	expect(fieldDescriptors([declared('text', true)])[0].isValid).toEqual({ required: true })
})
