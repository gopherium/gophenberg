// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { hoistStylesheet } from '../../stylesheetFirst'

const CSS = '<link rel="stylesheet" crossorigin href="/admin/assets/index-abc.css">'
const SCRIPT = '<script type="module" crossorigin src="/admin/assets/index-def.js"></script>'
const PRELOAD = '<link rel="modulepreload" crossorigin href="/admin/assets/router-ghi.js">'

/**
 * Returns the built page with the given head tags in order.
 * @param head - The head tags to place, in source order.
 * @returns The page source.
 */
function page(head: string[]): string {
	return `<!doctype html><html><head><style>.boot{}</style>${head.join('')}</head><body></body></html>`
}

test('requests the stylesheet before the script that would queue ahead of it', () => {
	const out = hoistStylesheet(page([SCRIPT, PRELOAD, CSS]))

	expect(out.indexOf(CSS)).toBeLessThan(out.indexOf(SCRIPT))
	expect(out.indexOf(CSS)).toBeLessThan(out.indexOf(PRELOAD))
})

test('keeps the inline boot styles ahead of everything', () => {
	const out = hoistStylesheet(page([SCRIPT, CSS]))

	expect(out.indexOf('<style>')).toBeLessThan(out.indexOf(CSS))
})

test('moves every stylesheet, not only the first', () => {
	const second = '<link rel="stylesheet" href="/admin/assets/editor-jkl.css">'
	const out = hoistStylesheet(page([SCRIPT, CSS, second]))

	expect(out.indexOf(CSS)).toBeLessThan(out.indexOf(SCRIPT))
	expect(out.indexOf(second)).toBeLessThan(out.indexOf(SCRIPT))
})

test('leaves a page with no stylesheet untouched', () => {
	const source = page([SCRIPT, PRELOAD])

	expect(hoistStylesheet(source)).toBe(source)
})

test('leaves a page whose stylesheet already leads untouched', () => {
	const source = page([CSS, SCRIPT])

	expect(hoistStylesheet(source)).toBe(source)
})

test('leaves a page with no script alone', () => {
	const source = page([CSS])

	expect(hoistStylesheet(source)).toBe(source)
})
