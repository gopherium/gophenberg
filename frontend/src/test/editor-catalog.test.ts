// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import {
	EDITOR_RELEASE,
	editorCatalogURL,
	editorStrings,
	filterEditorCatalog,
	wordpressLocale,
} from '../../scripts/editorCatalog.ts'
import { repositoryRoot } from '../../scripts/config.ts'

/** The separator a context keyed message carries between its parts. */
const CONTEXT = ''

test('names the platform slug for a language', () => {
	expect(wordpressLocale('es-ES')).toBe('es')
})

test('addresses the catalogue for a language', () => {
	expect(editorCatalogURL('es-ES')).toContain('/es/')
})

test('pins the release whose packages this repository holds', () => {
	expect(EDITOR_RELEASE).toMatch(/^\d+\.\d+\.\d+$/)
})

test('keeps a string the pinned packages hold', () => {
	const kept = filterEditorCatalog({ Bold: ['Negrita'] }, new Set(['Bold']))

	expect(kept).toHaveProperty('Bold')
})

test('drops a string the pinned packages never hold', () => {
	const kept = filterEditorCatalog({ Bold: ['Negrita'] }, new Set(['Italic']))

	expect(kept).not.toHaveProperty('Bold')
})

test('keeps a context keyed string by the message behind its context', () => {
	const key = `block title${CONTEXT}Paragraph`

	const kept = filterEditorCatalog({ [key]: ['Parrafo'] }, new Set(['Paragraph']))

	expect(kept).toHaveProperty(key)
})

test('always keeps the metadata entry', () => {
	const kept = filterEditorCatalog({ '': { 'plural-forms': 'nplurals=2' } }, new Set())

	expect(kept).toHaveProperty('')
})

test('reads the strings a package passes to a gettext call', () => {
	const held = editorStrings(repositoryRoot(), ['frontend/testdata/potfixture/wrapped.tsx'])

	expect(held.has('Zulu label')).toBe(true)
	expect(held.has('Alpha label')).toBe(true)
})

test('reads a source string whose text carries an escaped quote', () => {
	const root = mkdtempSync(join(tmpdir(), 'gophenberg-editor-'))
	mkdirSync(join(root, 'pkg'), { recursive: true })
	writeFileSync(join(root, 'pkg', 'held.js'), String.raw`__('It\'s here')` + '\n')

	expect(editorStrings(root, ['pkg/held.js'])).toContain("It's here")
})

test('keeps a source string whose escape it cannot read', () => {
	const root = mkdtempSync(join(tmpdir(), 'gophenberg-editor-'))
	mkdirSync(join(root, 'pkg'), { recursive: true })
	writeFileSync(join(root, 'pkg', 'held.js'), String.raw`__('a \x41 b')` + '\n')

	expect(editorStrings(root, ['pkg/held.js'])).toContain(String.raw`a \x41 b`)
})
