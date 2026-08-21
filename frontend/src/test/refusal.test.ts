// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { execFileSync } from 'node:child_process'
import { join } from 'node:path'

import { refusalText } from '../i18n/refusal'
import { refusalTemplates } from '../i18n/refusalTemplates'
import { repositoryRoot } from '../../scripts/config.ts'

test('reads the message a code stands for', () => {
	const held = refusalText({ error: 'content: type still holds content', code: 'type_in_use' })

	expect(held).not.toBe('content: type still holds content')
	expect(held).toMatch(/content/i)
})

test('puts the data a refusal carries into its message', () => {
	const held = refusalText({
		error: 'content: unknown field: colour',
		code: 'field_unknown',
		meta: { field: 'colour' },
	})

	expect(held).toContain('colour')
})

test('falls back to the server message for a code it does not know', () => {
	const held = refusalText({ error: 'content: something new', code: 'a_code_from_the_future' })

	expect(held).toBe('content: something new')
})

test('falls back to the server message when no code rides at all', () => {
	expect(refusalText({ error: 'the server answered 500' })).toBe('the server answered 500')
})

test('answers something readable when the server said nothing', () => {
	expect(refusalText({ error: '' })).not.toBe('')
})

test('leaves a placeholder alone when the data it names is absent', () => {
	const held = refusalText({ error: 'content: unknown field: colour', code: 'field_unknown' })

	expect(held).toBe('content: unknown field: colour')
})

test('carries a message for every code the server can answer with', () => {
	const script = join(repositoryRoot(), 'frontend', 'scripts', 'emittedCodes.sh')
	const emitted = execFileSync('sh', [script], { cwd: repositoryRoot(), encoding: 'utf8' })
		.split('\n')
		.filter((held) => held !== '')
	const held = refusalTemplates()

	expect(emitted.filter((code) => held[code] === undefined)).toEqual([])
})
