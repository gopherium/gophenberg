// SPDX-License-Identifier: Apache-2.0

import * as wpds from '@wordpress/ui'
import { expect, test } from 'vitest'

import { Collapsible } from '../index'

test('hands plugins the collapsible of the WordPress UI package', () => {
	expect(Collapsible).toBe(wpds.Collapsible)
})
