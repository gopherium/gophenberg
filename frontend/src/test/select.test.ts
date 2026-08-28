// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { chosenOf } from '../content/select'

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
