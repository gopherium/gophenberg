// SPDX-License-Identifier: Apache-2.0

import { afterEach, expect, test } from 'vitest'

import { DEFAULT_LOCALE } from '../i18n/catalog'
import { displayLocale, formatDate, rememberFormatLocale, rememberLocale } from '@gopherium/gottext'
import { fileSize } from '../media/format'

const NOON = '2026-08-16T12:00:00Z'

afterEach(() => {
	rememberLocale(DEFAULT_LOCALE)
})

test('shows dates in the site format whatever language the reader settled on', () => {
	rememberFormatLocale('es-ES')
	rememberLocale('en-US')

	expect(formatDate(NOON)).toBe('16/08/2026')
})

test('shows dates in the language the reader settled on until the site names a format', () => {
	rememberFormatLocale(undefined)

	expect(formatDate(NOON)).toBe('08/16/2026')
})

test('answers nothing for a timestamp that is not there', () => {
	expect(formatDate('')).toBe('')
})

test('shows file sizes in the site format to an English reader', () => {
	rememberFormatLocale('es-ES')
	rememberLocale('en-US')

	expect(fileSize(1.5 * 1024 * 1024)).toBe('1,5 MB')
})

test('shows a file size of thousands of bytes grouped in the site format', () => {
	rememberFormatLocale('es-ES')

	expect(fileSize(1000)).toBe('1.000 B')
})

test('reports the language it settled on', () => {
	rememberLocale('es-ES')

	expect(displayLocale()).toBe('es-ES')
})
