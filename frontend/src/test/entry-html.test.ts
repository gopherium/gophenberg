// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { expect, test } from 'vitest'

const entry = readFileSync(resolve('index.html'), 'utf8')

test('stands a ghost in the root before any script runs', () => {
	const root = entry.slice(entry.indexOf('<div id="root"'))

	expect(root).toMatch(/gophenberg-boot__bar/)
	expect(root).toMatch(/aria-hidden="true"/)
})

test('holds that ghost back so a quick load never shows it', () => {
	expect(entry).toMatch(/\.gophenberg-boot\s*\{[^}]*opacity:\s*0/)
	expect(entry).toMatch(/\.gophenberg-boot\s*\{[^}]*animation:[^;]*150ms/)
	expect(entry).toMatch(/@keyframes gophenberg-boot-in/)
})
