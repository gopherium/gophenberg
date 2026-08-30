// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { seededValues } from '../content/fieldDefaults'
import type { ContentField } from '../content/types'

/**
 * Returns a declared field carrying the given settings.
 * @param key - The key the field is stored under.
 * @param kind - The kind the field holds.
 * @param settings - The settings the field carries.
 * @returns The declared field.
 */
function declared(key: string, kind: string, settings: Record<string, unknown>): ContentField {
	return {
		key,
		label: key,
		kind,
		relatesTo: '',
		many: false,
		required: false,
		settings,
		updatedAt: '2026-08-01T10:00:00Z',
	}
}

test('seeds the default a choice field names', () => {
	expect(
		seededValues([declared('style', 'choice', { default: 'ipa' })]),
	).toEqual({ style: 'ipa' })
})

test('seeds the list a multiple choice names', () => {
	expect(
		seededValues([
			declared('styles', 'choice', { multiple: true, default: ['ipa', 'stout'] }),
		]),
	).toEqual({ styles: ['ipa', 'stout'] })
})

test('seeds nothing from a choice default of the wrong shape', () => {
	expect(seededValues([declared('style', 'choice', { default: 5 })])).toEqual({})
	expect(
		seededValues([declared('style', 'choice', { multiple: true, default: 'ipa' })]),
	).toEqual({})
	expect(
		seededValues([declared('styles', 'choice', { multiple: true, default: ['ipa', 5] })]),
	).toEqual({})
})

test('seeds nothing when no field names a default', () => {
	expect(seededValues([])).toEqual({})
	expect(seededValues([declared('subtitle', 'text', {})])).toEqual({})
	expect(seededValues([declared('subtitle', 'text', { placeholder: 'Maria Perez' })])).toEqual({})
})

test('seeds the default a field names', () => {
	expect(
		seededValues([
			declared('subtitle', 'text', { default: 'unnamed' }),
			declared('rating', 'number', { default: 5 }),
			declared('boxed', 'boolean', { default: true }),
		]),
	).toEqual({ subtitle: 'unnamed', rating: 5, boxed: true })
})

test('seeds nothing from a kind that takes no default', () => {
	expect(seededValues([declared('cover', 'media', { instructions: 'A cover.' })])).toEqual({})
	expect(seededValues([declared('sold-on', 'date', { instructions: 'The day.' })])).toEqual({})
	expect(seededValues([declared('categories', 'relation', { instructions: 'Files under.' })])).toEqual({})
})

test('seeds only the fields whose default matches the kind they hold', () => {
	expect(
		seededValues([
			declared('subtitle', 'text', { default: 5 }),
			declared('rating', 'number', { default: 'five' }),
			declared('boxed', 'boolean', { default: 'yes' }),
		]),
	).toEqual({})
})

test('seeds an empty string a text field names', () => {
	expect(seededValues([declared('subtitle', 'text', { default: '' })])).toEqual({ subtitle: '' })
})

test('seeds a false a boolean field names', () => {
	expect(seededValues([declared('boxed', 'boolean', { default: false })])).toEqual({ boxed: false })
})

test('seeds a zero a number field names', () => {
	expect(seededValues([declared('rating', 'number', { default: 0 })])).toEqual({ rating: 0 })
})
