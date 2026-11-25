// SPDX-License-Identifier: Apache-2.0

import * as wpds from '@wordpress/ui'
import { expect, test } from 'vitest'

import { Fieldset } from '../index'

test('hands plugins the fieldset of the WordPress UI package', () => {
	expect(Fieldset).toBe(wpds.Fieldset)
})
