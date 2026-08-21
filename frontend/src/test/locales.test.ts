// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/config.ts'
import { supportedLocales } from '../../scripts/locales.ts'

test('reads the languages the application says it answers in', () => {
	expect(supportedLocales(repositoryRoot())).toEqual(['en-US', 'es-ES', 'fr-FR'])
})

test('refuses a source that declares no supported languages', () => {
	const root = mkdtempSync(join(tmpdir(), 'gophenberg-locale-'))
	mkdirSync(join(root, 'internal', 'content'), { recursive: true })
	writeFileSync(join(root, 'internal', 'content', 'locale.go'), 'package content\n')

	expect(() => supportedLocales(root)).toThrow(/no supported languages/)
})
