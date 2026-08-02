// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import dataviews from '../dataviews.ts?raw'

const PARENT_SHEETS = [
	'@wordpress/components/build-style/style.css',
	'@wordpress/dataviews/build-style/style.css',
]

test.each(PARENT_SHEETS)('loads %s into the document the listing renders in', (sheet) => {
	expect(dataviews).toContain(sheet)
})
