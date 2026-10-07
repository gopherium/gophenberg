// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { strykerPlugins, textDomainReason } from '../../scripts/textDomain.ts'

/**
 * Returns the paths of the arguments of a call, as Stryker hands them to an ignorer.
 * @param callee - The name the call is made through.
 * @param values - The string arguments of the call.
 * @returns One path per argument.
 */
function argumentPaths(callee: string, ...values: string[]) {
	const args = values.map((value) => ({ type: 'StringLiteral', value }))
	const call = { type: 'CallExpression', callee: { type: 'Identifier', name: callee }, arguments: args }
	const callPath = { node: call, parentPath: null }
	return args.map((node) => ({ node, parentPath: callPath }))
}

test.each(['__', '_x', '_n', '_nx'])('leaves the text domain of %s unmutated', (callee) => {
	const paths = argumentPaths(callee, 'one', 'many', 'gophenberg')

	expect(textDomainReason(paths[2])).toBeTypeOf('string')
})

test('mutates the words a translation call reads', () => {
	const paths = argumentPaths('_n', 'one', 'many', 'gophenberg')

	expect(textDomainReason(paths[0])).toBeUndefined()
	expect(textDomainReason(paths[1])).toBeUndefined()
})

test('mutates the last argument of a call that translates nothing', () => {
	const paths = argumentPaths('sprintf', 'Hello %s', 'gophenberg')

	expect(textDomainReason(paths[1])).toBeUndefined()
})

test('mutates a text domain held in a variable', () => {
	const domain = { type: 'Identifier', name: 'domain' }
	const call = { type: 'CallExpression', callee: { type: 'Identifier', name: '__' }, arguments: [domain] }

	expect(textDomainReason({ node: domain, parentPath: { node: call, parentPath: null } })).toBeUndefined()
})

test('mutates a string outside any call', () => {
	expect(textDomainReason({ node: { type: 'StringLiteral' }, parentPath: null })).toBeUndefined()
})

test('mutates a string inside a list', () => {
	const domain = { type: 'StringLiteral' }
	const list = { type: 'ArrayExpression', elements: [domain] }

	expect(textDomainReason({ node: domain, parentPath: { node: list, parentPath: null } })).toBeUndefined()
})

test('mutates a string inside a call made through an object', () => {
	const domain = { type: 'StringLiteral' }
	const call = { type: 'CallExpression', callee: { type: 'MemberExpression' }, arguments: [domain] }

	expect(textDomainReason({ node: domain, parentPath: { node: call, parentPath: null } })).toBeUndefined()
})

test('declares the ignorer to Stryker under its name', () => {
	expect(strykerPlugins).toEqual([{ kind: 'Ignore', name: 'text-domain', value: { shouldIgnore: textDomainReason } }])
})
