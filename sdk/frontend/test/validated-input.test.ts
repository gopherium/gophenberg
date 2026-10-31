// SPDX-License-Identifier: Apache-2.0

import * as wpds from '@wordpress/ui'
import { expect, test } from 'vitest'

import { ValidatedInputControl } from '../index'

test('hands plugins the field WPDS marks as required and shows its message under', () => {
	expect(ValidatedInputControl).toBe(wpds.ValidatedInputControl)
})
