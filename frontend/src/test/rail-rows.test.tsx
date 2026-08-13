// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import '../index.css'
import { renderAt } from './render'

const ROW = 'gophenberg-menu__item'

/**
 * Returns the rules the application stylesheet holds.
 * @returns The stylesheet text.
 */
function stylesheet(): string {
	return Array.from(document.styleSheets)
		.flatMap((sheet) => Array.from(sheet.cssRules ?? []))
		.map((rule) => rule.cssText)
		.join('\n')
}

test('dresses a section entry like every other rail row', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('link', { name: 'All Posts' })).toHaveClass(ROW)
})

test('dresses a section action like every other rail row', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('button', { name: 'Add New' })).toHaveClass(ROW)
})

test('dresses the top level rows the same way', async () => {
	renderAt('/')

	expect(await screen.findByRole('link', { name: 'Posts' })).toHaveClass(ROW)
})

test('gives a rail row its own colour rather than the browser default', () => {
	const rules = stylesheet()

	expect(rules).toMatch(/menu__item[^}]*color:/)
	expect(rules).toMatch(/menu__item[^}]*text-decoration:\s*none/)
})
