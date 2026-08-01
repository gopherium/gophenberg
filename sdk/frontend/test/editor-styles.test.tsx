// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import ambient from '../editor/ambient.ts?raw'
import { CANVAS_STYLES } from '../editor'

const PARENT_SHEETS = [
	'@wordpress/components/build-style/style.css',
	'@wordpress/block-editor/build-style/style.css',
	'@wordpress/block-library/build-style/editor.css',
]

test.each(PARENT_SHEETS)('loads %s into the document the chrome renders in', (sheet) => {
	expect(ambient).toContain(sheet)
})

test('leaves the format popover stylesheet alone, since the package exports no path to it', () => {
	expect(ambient).not.toContain('@wordpress/format-library/build-style')
})

test('registers the formats a writer marks text up with', async () => {
	await import('../editor/ambient')
	const { select } = await import('@wordpress/data')
	const { store } = await import('@wordpress/rich-text')

	const formats = select(store as never) as { getFormatTypes: () => { name: string }[] }
	const registered = formats.getFormatTypes()
	const names = registered.map((format) => format.name)

	expect(names).toEqual(
		expect.arrayContaining(['core/bold', 'core/italic', 'core/link', 'core/code']),
	)
})

test('holds the blocks in a readable column', () => {
	const css = CANVAS_STYLES.map((entry) => entry.css).join('\n')

	expect(css).toMatch(/\.wp-block\s*\{[^}]*max-width:\s*700px/)
	expect(css).toMatch(/margin-left:\s*auto/)
})

test('lets a wide block run wider and a full block run edge to edge', () => {
	const css = CANVAS_STYLES.map((entry) => entry.css).join('\n')

	expect(css).toMatch(/alignwide[^}]*max-width:\s*900px/)
	expect(css).toMatch(/alignfull[^}]*max-width:\s*none/)
})
