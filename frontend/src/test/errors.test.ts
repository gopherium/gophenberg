// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { execFileSync } from 'node:child_process'
import { join } from 'node:path'

import { errorText } from '../i18n/errors'
import { errorTemplates } from '../i18n/errorTemplates'
import { repositoryRoot } from '../../scripts/config.ts'

test('reads the message a code stands for', () => {
	const held = errorText({ error: 'content: type still holds content', code: 'type_in_use' })

	expect(held).not.toBe('content: type still holds content')
	expect(held).toMatch(/content/i)
})

test('puts the data an error carries into its message', () => {
	const held = errorText({
		error: 'content: unknown field: colour',
		code: 'field_unknown',
		meta: { field: 'colour' },
	})

	expect(held).toContain('colour')
})

test('falls back to the server message for a code it does not know', () => {
	const held = errorText({ error: 'content: something new', code: 'a_code_from_the_future' })

	expect(held).toBe('content: something new')
})

test('falls back to the server message when no code rides at all', () => {
	expect(errorText({ error: 'the server answered 500' })).toBe('the server answered 500')
})

test('answers something readable when the server said nothing', () => {
	expect(errorText({ error: '' })).not.toBe('')
})

test('leaves a placeholder alone when the data it names is absent', () => {
	const held = errorText({ error: 'content: unknown field: colour', code: 'field_unknown' })

	expect(held).toBe('content: unknown field: colour')
})

test('carries a message for every code the server can answer with', () => {
	const script = join(repositoryRoot(), 'frontend', 'scripts', 'emittedCodes.sh')
	const emitted = execFileSync('sh', [script], { cwd: repositoryRoot(), encoding: 'utf8' })
		.split('\n')
		.filter((held) => held !== '')
	const held = errorTemplates()

	expect(emitted.filter((code) => held[code] === undefined)).toEqual([])
})
