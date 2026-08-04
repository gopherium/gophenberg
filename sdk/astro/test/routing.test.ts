// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { pageNumber } from '../routing.ts'

describe('pageNumber', () => {
	test('serves the front page when the address names none', () => {
		expect(pageNumber(undefined)).toBe(1)
	})

	test('reads a plain numbered segment', () => {
		expect(pageNumber('7')).toBe(7)
	})

	test('reads the signs and leading zeros the renderer also accepts', () => {
		expect(pageNumber('007')).toBe(7)
		expect(pageNumber('+5')).toBe(5)
	})

	test('clamps zero and below to the first page like the renderer', () => {
		expect(pageNumber('0')).toBe(1)
		expect(pageNumber('-3')).toBe(1)
	})

	test('rejects a segment that is not a number', () => {
		expect(pageNumber('banana')).toBeUndefined()
		expect(pageNumber('12abc')).toBeUndefined()
		expect(pageNumber('2.5')).toBeUndefined()
		expect(pageNumber('')).toBeUndefined()
	})

	test('rejects a segment past the range the renderer parses', () => {
		expect(pageNumber('9223372036854775807')).toBeDefined()
		expect(pageNumber('9223372036854775808')).toBeUndefined()
		expect(pageNumber('-9223372036854775809')).toBeUndefined()
	})
})
