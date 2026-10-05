// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { everyDecimal } from '../i18n/numbers'

test('writes a number to its last digit however small it is', () => {
	expect(everyDecimal(1e-21)).toBe('0,000000000000000000001')
})

test('writes a number with no digit it does not hold', () => {
	expect(everyDecimal(0.1)).toBe('0,1')
	expect(everyDecimal(0.30000000000000004)).toBe('0,30000000000000004')
	expect(everyDecimal(1234.5)).toBe('1.234,5')
})
