// SPDX-License-Identifier: AGPL-3.0-or-later

import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { goMessages, goString, repositoryRoot } from '../../scripts/pot.ts'

const found = goMessages(repositoryRoot())

test('finds the messages the Go templates ask the translator for', () => {
	expect(found).toContain('Older posts')
	expect(found).toContain('No page lives at this address.')
})

test('finds the messages the Go source marks', () => {
	expect(found).toContain('January')
	expect(found).toContain('August')
	expect(found).toContain('%[1]d %[2]s %[3]d')
	expect(found).toContain('Not found')
})

test('leaves settings keys and test prose out', () => {
	expect(found).not.toContain('locale.default')
	expect(found).not.toContain('A dated post')
})

test('skips installed trees when it walks', () => {
	const root = mkdtempSync(join(tmpdir(), 'gophenberg-extract-'))
	mkdirSync(join(root, 'src', 'node_modules'), { recursive: true })
	writeFileSync(join(root, 'src', 'held.go'), 'var held = Msgid("A marked message")\n')
	writeFileSync(join(root, 'src', 'node_modules', 'vendored.go'), 'var v = Msgid("A vendored message")\n')

	const held = goMessages(root, ['src'])

	expect(held).toContain('A marked message')
	expect(held).not.toContain('A vendored message')
})

test('resolves the escapes a Go string carries', () => {
	expect(goString('a \\"quoted\\" word', 'held.go')).toBe('a "quoted" word')
})

test('refuses an escape it cannot read rather than guessing', () => {
	expect(() => goString('\\x41', 'held.go')).toThrow(/held\.go/)
})
