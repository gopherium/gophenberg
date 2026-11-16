// SPDX-License-Identifier: Apache-2.0

import * as wpds from '@wordpress/ui'
import { expect, test } from 'vitest'

import { Checkbox } from '../index'

test('hands plugins the checkbox of the WordPress UI package', () => {
	expect(Checkbox).toBe(wpds.Checkbox)
})
