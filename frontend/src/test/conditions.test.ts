// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { compare, conditionsOf, hiddenKeys, multipleOf, needsValue } from '../content/conditions'
import type { ConditionField } from '../content/conditions'

/**
 * Returns a text field carrying the settings.
 * @param settings - The settings the field holds.
 * @returns The field to read.
 */
function textField(settings: Record<string, unknown>): ConditionField {
	return { key: 'note', kind: 'text', settings }
}

test('names the operators that compare against a value', () => {
	expect(needsValue('==')).toBe(true)
	expect(needsValue('empty')).toBe(false)
	expect(needsValue('not_empty')).toBe(false)
})

test('reads no rules from a field that carries none', () => {
	expect(conditionsOf(textField({}))).toEqual([])
	expect(conditionsOf(textField({ conditions: 'not a list' }))).toEqual([])
})

test('reads a group it cannot understand as holding no rules', () => {
	expect(conditionsOf(textField({ conditions: ['not a group'] }))).toEqual([])
})

test('reads the parts a stored rule leaves out as empty', () => {
	const held = conditionsOf(textField({ conditions: [[{ source: 'on-sale' }]] }))

	expect(held).toEqual([[{ source: 'on-sale', operator: '', value: '' }]])
})

test('drops a rule naming nothing at all, and the group it empties', () => {
	expect(conditionsOf(textField({ conditions: [[null]] }))).toEqual([])
})

test('reads whether a choice field takes several values', () => {
	expect(multipleOf(textField({ multiple: true }))).toBe(true)
	expect(multipleOf(textField({}))).toBe(false)
})

test('compares nothing against a rule value that is not a number', () => {
	expect(compare('==', 10, ' ')).toBe(false)
	expect(compare('<', 10, ' ')).toBe(false)
	expect(compare('>', 10, 'ten')).toBe(false)
})

test('shows a field whose rules hold nothing once the empty groups go', () => {
	const fields: ConditionField[] = [textField({ conditions: [[]] })]

	expect([...hiddenKeys(fields, {})]).toEqual([])
})

test('hides a field whose rule names a sibling no rule may read', () => {
	const fields: ConditionField[] = [
		{ key: 'crew', kind: 'repeater', settings: {} },
		{ key: 'note', kind: 'text', settings: { conditions: [[{ source: 'crew', operator: 'not_empty', value: '' }]] } },
	]

	expect([...hiddenKeys(fields, { crew: [{}] })]).toEqual(['note'])
})

test('leaves a field alone when its rules read a sibling that holds', () => {
	const fields: ConditionField[] = [
		{ key: 'on-sale', kind: 'boolean', settings: {} },
		{ key: 'note', kind: 'text', settings: { conditions: [[{ source: 'on-sale', operator: '==', value: 'true' }]] } },
	]

	expect([...hiddenKeys(fields, { 'on-sale': true })]).toEqual([])
})
