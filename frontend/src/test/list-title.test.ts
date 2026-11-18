// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { declarations } from './stylesheet'

/** The box a list table draws a row's title and description in. */
const PRIMARY = '.godmin-page__list .dataviews-view-table tbody .dataviews-view-table__primary-column-content'

test('sizes a table title by the room the other columns leave, never by its own length', () => {
	expect(declarations(PRIMARY)).toMatchObject({
		'flex-grow': '1',
		contain: 'inline-size',
	})
})

test('cuts a table title that is no link at the edge of its column', () => {
	expect(declarations(`${PRIMARY} > .dataviews-title-field`)).toMatchObject({
		display: 'block',
		'min-inline-size': '0',
		overflow: 'hidden',
	})
})
