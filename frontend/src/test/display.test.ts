// SPDX-License-Identifier: Apache-2.0

import { afterEach, expect, test } from 'vitest'

import { DEFAULT_LOCALE } from '../i18n/catalog'
import { displayLocale, formatDate, rememberLocale } from '@gopherium/gottext'
import { fileSize } from '../media/format'

const NOON = '2026-08-16T12:00:00Z'

afterEach(() => {
	rememberLocale(DEFAULT_LOCALE)
})

test('shows dates in the language the reader settled on', () => {
	rememberLocale('es-ES')

	expect(formatDate(NOON)).toBe('16/8/2026')
})

test('shows dates in the fallback language until one is settled', () => {
	expect(formatDate(NOON)).toBe('8/16/2026')
})

test('answers nothing for a timestamp that is not there', () => {
	expect(formatDate('')).toBe('')
})

test('shows file sizes in the language the reader settled on', () => {
	rememberLocale('es-ES')

	expect(fileSize(1536)).toBe('1,5 KB')
})

test('reports the language it settled on', () => {
	rememberLocale('es-ES')

	expect(displayLocale()).toBe('es-ES')
})
