// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { declarations } from './stylesheet'

/** The boxes an author cell draws before the name, its avatar or the author icon. */
const MARKS = ['.gophenberg-author__avatar', '.gophenberg-author__icon']

test.each(MARKS)(
	'keeps the box of %s one line tall, as WordPress does, so the author row is as tall as its label',
	(mark) => {
		expect(declarations(mark)).toMatchObject({ 'align-items': 'center', height: '1lh' })
	},
)

test('sets the avatar on the first line of an author name that wraps, as WordPress does', () => {
	expect(declarations('.gophenberg-author')).toMatchObject({ 'align-items': 'flex-start' })
})
