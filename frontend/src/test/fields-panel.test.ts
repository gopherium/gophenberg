// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { clearedEdits, editableFields, fieldDescriptors, fieldValidity } from '../content/FieldsPanel'
import type { ContentField } from '../content/types'

/**
 * Returns a declared field of the given kind.
 * @param kind - The kind the field holds.
 * @param required - Whether publishing demands a value.
 * @returns The declared field.
 */
function declared(kind: string, required = false): ContentField {
	return {
		key: `a-${kind}`,
		label: kind,
		kind,
		relatesTo: '',
		many: false,
		required,
		settings: {},
		fields: [],
		updatedAt: '2026-08-01T10:00:00Z',
	}
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
	return {
		key: `a-${kind}`,
		label: kind,
		kind,
		relatesTo: '',
		many: false,
		required: false,
		settings,
		fields: [],
		updatedAt: '2026-08-01T10:00:00Z',
	}
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

const PAIRS = [
	{ value: 'ipa', label: 'IPA' },
	{ value: 'stout', label: 'Stout' },
]

test('renders a choice field as its pairs to pick from', () => {
	const held = fieldDescriptors([carrying('choice', { choices: PAIRS })])

	expect(held[0].type).toBe('text')
	expect(held[0].elements).toEqual(PAIRS)
	expect(held[0].Edit).toBeUndefined()
})

test('offers an empty entry when the choice allows one', () => {
	const held = fieldDescriptors([carrying('choice', { choices: PAIRS, allow_null: true })])

	expect(held[0].elements).toEqual([{ value: '', label: 'None' }, ...PAIRS])
})

test('gives each presentation its own control', () => {
	const radio = fieldDescriptors([carrying('choice', { choices: PAIRS, presentation: 'radio' })])
	const boxes = fieldDescriptors([carrying('choice', { choices: PAIRS, presentation: 'checkbox' })])
	const buttons = fieldDescriptors([carrying('choice', { choices: PAIRS, presentation: 'buttons' })])
	const select = fieldDescriptors([carrying('choice', { choices: PAIRS, presentation: 'select' })])

	expect(radio[0].Edit).toBe('radio')
	expect(boxes[0].Edit).toBe('radio')
	expect(buttons[0].Edit).toBe('toggleGroup')
	expect(select[0].Edit).toBeUndefined()
})

test('renders a multiple choice as a list of tokens', () => {
	const held = fieldDescriptors([carrying('choice', { choices: PAIRS, multiple: true })])

	expect(held[0].type).toBe('array')
	expect(held[0].elements).toEqual(PAIRS)
})

test('leaves a choice with no pairs listing nothing', () => {
	const held = fieldDescriptors([carrying('choice', {})])

	expect(held[0].elements).toBeUndefined()
})

test('gives a text variant its own input', () => {
	const contact = fieldDescriptors([carrying('text', { variant: 'email' })])
	const homepage = fieldDescriptors([carrying('text', { variant: 'url' })])
	const notes = fieldDescriptors([carrying('text', { variant: 'textarea' })])

	expect(contact[0].type).toBe('email')
	expect(homepage[0].type).toBe('url')
	expect(notes[0].type).toBe('text')
	expect(notes[0].Edit).toBe('textarea')
})

test('renders a range between its bounds as a slider', () => {
	const held = fieldDescriptors([
		carrying('number', { presentation: 'range', min: 1, max: 10 }),
	])

	expect(typeof held[0].Edit).toBe('function')
})

test('leaves a range missing a bound on the plain number input', () => {
	const held = fieldDescriptors([carrying('number', { presentation: 'range', min: 1 })])

	expect(held[0].Edit).toBeUndefined()
})

test('renders the choice kind among the editable fields', () => {
	const held = [declared('choice'), declared('text')]

	expect(editableFields(held)).toHaveLength(2)
})

test('ignores a setting whose value is the wrong shape', () => {
	const held = fieldDescriptors([carrying('number', { min: 'low', max: true })])

	expect(held[0].isValid).toEqual({ required: false })
})

test('hands a radio group taking custom answers its own control', () => {
	const held = fieldDescriptors([
		carrying('choice', {
			presentation: 'radio',
			allow_custom: true,
			choices: [{ value: 'ipa', label: 'IPA' }],
		}),
	])

	expect(typeof held[0].Edit).toBe('function')
})

test('hands a radio group taking customs its own control even listing nothing', () => {
	const held = fieldDescriptors([carrying('choice', { presentation: 'radio', allow_custom: true })])

	expect(typeof held[0].Edit).toBe('function')
})

test('leaves a radio group taking only its listed answers on the stock control', () => {
	const held = fieldDescriptors([
		carrying('choice', { presentation: 'radio', choices: [{ value: 'ipa', label: 'IPA' }] }),
	])

	expect(held[0].Edit).toBe('radio')
})

test('hands a checkbox group its own control', () => {
	const held = fieldDescriptors([
		carrying('choice', {
			presentation: 'checkbox',
			multiple: true,
			choices: [{ value: 'ipa', label: 'IPA' }],
		}),
	])

	expect(typeof held[0].Edit).toBe('function')
})

test('leaves a checkbox group listing nothing on the many values box', () => {
	const held = fieldDescriptors([carrying('choice', { presentation: 'checkbox', multiple: true })])

	expect(held[0].Edit).toBeUndefined()
})

test('names the floor under a number below its min', () => {
	const spoken = fieldValidity([carrying('number', { min: 5 })], { 'a-number': 2 })

	expect(spoken).toEqual({
		'a-number': {
			custom: {
				type: 'invalid',
				message: 'number goes no lower than 5. Raise the value and save again.',
			},
		},
	})
})

test('names the ceiling over a number above its max', () => {
	const spoken = fieldValidity([carrying('number', { max: 10 })], { 'a-number': 50 })

	expect(spoken).toEqual({
		'a-number': {
			custom: {
				type: 'invalid',
				message: 'number goes no higher than 10. Lower the value and save again.',
			},
		},
	})
})

test('holds no complaint for what the bounds allow', () => {
	const bounded = [carrying('number', { min: 1, max: 10 })]

	expect(fieldValidity(bounded, { 'a-number': 5 })).toBeUndefined()
	expect(fieldValidity(bounded, { 'a-number': 1 })).toBeUndefined()
	expect(fieldValidity(bounded, { 'a-number': 10 })).toBeUndefined()
	expect(fieldValidity(bounded, {})).toBeUndefined()
	expect(fieldValidity(bounded, { 'a-number': null })).toBeUndefined()
	expect(fieldValidity(bounded, { 'a-number': 'five' })).toBeUndefined()
})

test('names an answer a choice field does not list', () => {
	const listing = carrying('choice', { choices: [{ value: 'ipa', label: 'IPA' }] })

	expect(fieldValidity([listing], { 'a-choice': 'homebrew' })).toEqual({
		'a-choice': {
			custom: {
				type: 'invalid',
				message: 'choice only takes one of its listed choices. Pick one and save again.',
			},
		},
	})
})

test('names an answer a many values field does not list', () => {
	const listing = carrying('choice', {
		multiple: true,
		choices: [{ value: 'ipa', label: 'IPA' }],
	})

	expect(fieldValidity([listing], { 'a-choice': ['ipa', 'homebrew'] })).toEqual({
		'a-choice': {
			custom: {
				type: 'invalid',
				message: 'choice only takes one of its listed choices. Pick one and save again.',
			},
		},
	})
})

test('holds no complaint for a choice field that takes what it does not list', () => {
	const taking = carrying('choice', {
		allow_custom: true,
		choices: [{ value: 'ipa', label: 'IPA' }],
	})

	expect(fieldValidity([taking], { 'a-choice': 'homebrew' })).toBeUndefined()
})

test('holds no complaint for a listed answer, an emptied one, or a field listing none', () => {
	const listing = carrying('choice', { choices: [{ value: 'ipa', label: 'IPA' }] })

	expect(fieldValidity([listing], { 'a-choice': 'ipa' })).toBeUndefined()
	expect(fieldValidity([listing], { 'a-choice': '' })).toBeUndefined()
	expect(fieldValidity([listing], { 'a-choice': 7 })).toBeUndefined()
	expect(fieldValidity([carrying('choice', {})], { 'a-choice': 'homebrew' })).toBeUndefined()
})

test('holds no complaint for an unbounded number or a bounded text', () => {
	expect(fieldValidity([carrying('number', {})], { 'a-number': 50 })).toBeUndefined()
	expect(
		fieldValidity([carrying('text', { maxlength: 2 })], { 'a-text': 'far too long' }),
	).toBeUndefined()
})
