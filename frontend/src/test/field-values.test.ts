// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { changedFieldValues, sameFieldValues } from '../content/fieldValues'

test('holds two sets of scalar values equal when every key matches', () => {
	expect(sameFieldValues({ color: 'red', doors: 4 }, { color: 'red', doors: 4 })).toBe(true)
	expect(sameFieldValues({ color: 'red' }, { color: 'blue' })).toBe(false)
})

test('parts two sets declaring different keys', () => {
	expect(sameFieldValues({ color: 'red' }, { color: 'red', doors: 4 })).toBe(false)
	expect(sameFieldValues({ color: 'red', doors: 4 }, { color: 'red' })).toBe(false)
})

test('holds two relation lists equal when they name the same targets in order', () => {
	expect(sameFieldValues({ categories: ['a', 'b'] }, { categories: ['a', 'b'] })).toBe(true)
	expect(sameFieldValues({ categories: ['a', 'b'] }, { categories: ['b', 'a'] })).toBe(false)
	expect(sameFieldValues({ categories: ['a'] }, { categories: ['a', 'b'] })).toBe(false)
})

test('parts a relation list from a value that is no list', () => {
	expect(sameFieldValues({ categories: ['a'] }, { categories: 'a' })).toBe(false)
	expect(sameFieldValues({ categories: 'a' }, { categories: ['a'] })).toBe(false)
})

test('takes only the values that moved', () => {
	const settled = { color: 'red', doors: 4, categories: ['a'] }

	expect(changedFieldValues({ ...settled, color: 'blue' }, settled)).toEqual({ color: 'blue' })
	expect(changedFieldValues(settled, settled)).toEqual({})
	expect(changedFieldValues({ ...settled, categories: ['a'] }, settled)).toEqual({})
	expect(changedFieldValues({ ...settled, color: null }, settled)).toEqual({ color: null })
})
