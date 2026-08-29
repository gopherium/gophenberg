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
	return { key: `a-${kind}`, label: kind, kind, relatesTo: '', many: false, required, settings: {} }
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

/**
 * Returns a declared field of the kind carrying the settings.
 * @param kind - The kind the field holds.
 * @param settings - The settings the field carries.
 * @returns The declared field.
 */
function carrying(kind: string, settings: Record<string, unknown>): ContentField {
	return { key: `a-${kind}`, label: kind, kind, relatesTo: '', many: false, required: false, settings }
}

test('shows the instructions a field carries under its control', () => {
	const held = fieldDescriptors([carrying('text', { instructions: 'Say who wrote it.' })])

	expect(held[0].description).toBe('Say who wrote it.')
})

test('shows the placeholder a field carries in its empty control', () => {
	const held = fieldDescriptors([carrying('text', { placeholder: 'Maria Perez' })])

	expect(held[0].placeholder).toBe('Maria Perez')
})

test('holds a text control to the length its field allows', () => {
	const held = fieldDescriptors([carrying('text', { maxlength: 80 })])

	expect(held[0].isValid).toEqual({ required: false, maxLength: 80 })
})

test('leaves a number control free to hold what the bounds refuse', () => {
	const held = fieldDescriptors([carrying('number', { min: 1, max: 10 })])

	expect(held[0].isValid).toEqual({ required: false })
})

test('leaves a control bare when its field carries no settings', () => {
	const held = fieldDescriptors([carrying('text', {})])

	expect(held[0].description).toBeUndefined()
	expect(held[0].placeholder).toBeUndefined()
	expect(held[0].isValid).toEqual({ required: false })
})

test('passes on a setting the control cannot express', () => {
	const held = fieldDescriptors([carrying('number', { step: 5, default: 3 })])

	expect(held[0].isValid).toEqual({ required: false })
})

test('ignores a setting whose value is the wrong shape', () => {
	const held = fieldDescriptors([carrying('number', { min: 'low', max: true })])

	expect(held[0].isValid).toEqual({ required: false })
})
