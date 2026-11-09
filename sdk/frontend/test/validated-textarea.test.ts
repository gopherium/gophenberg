// SPDX-License-Identifier: Apache-2.0

import * as wpds from '@wordpress/ui'
import { expect, test } from 'vitest'

import { ValidatedTextareaControl } from '../index'

test('hands plugins the textarea a DataForm draws for a field of several lines', () => {
	expect(ValidatedTextareaControl).toBe(wpds.ValidatedTextareaControl)
})
