// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { chosenTarget, targetsHeld } from '../content/RelationPicker'

test('reads the targets a relation field holds', () => {
	expect(targetsHeld(['a', 'b'])).toEqual(['a', 'b'])
})

test('reads no targets when the field holds something else', () => {
	expect(targetsHeld(undefined)).toEqual([])
	expect(targetsHeld('a')).toEqual([])
	expect(targetsHeld([1, 'a'])).toEqual(['a'])
})

test('takes the target a select reported', () => {
	expect(chosenTarget({ value: 'a' })).toEqual(['a'])
})

test('takes no target when the select reported none', () => {
	expect(chosenTarget(null)).toEqual([])
	expect(chosenTarget({ value: null })).toEqual([])
	expect(chosenTarget({ value: '' })).toEqual([])
})
