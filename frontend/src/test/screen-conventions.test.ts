// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

const sources = import.meta.glob(
	['../**/*.tsx', '../../../sdk/frontend/**/*.tsx', '../../../plugins/*/frontend/**/*.tsx'],
	{ eager: true, query: '?raw', import: 'default' },
) as Record<string, string>

/**
 * Returns the source files a convention applies to.
 * @returns The path and text of every non test source file.
 */
function screens(): [string, string][] {
	return Object.entries(sources).filter(([path]) => !path.includes('/test/'))
}

/**
 * Returns the files whose source matches the given pattern.
 * @param pattern - The shape a convention forbids.
 * @returns The paths of the offending files.
 */
function offenders(pattern: RegExp): string[] {
	return screens()
		.filter(([, text]) => pattern.test(text))
		.map(([path]) => path)
}

test('reads every screen source', () => {
	expect(screens().length).toBeGreaterThan(10)
})

test('leaves the page heading to the kit', () => {
	expect(offenders(/<h1[\s>]|render=\{<h1/)).toEqual([])
})

test('leaves the drill-down heading to the kit', () => {
	expect(offenders(/<h2[\s>]|render=\{<h2/)).toEqual([])
})

test('gives every kit table a scroll region to live in', () => {
	const tables = screens().filter(([, text]) => text.includes('godmin-table"'))

	expect(tables.length).toBeGreaterThan(0)
	for (const [path, text] of tables) {
		expect(text, path).toContain('godmin-table-scroll')
	}
})
