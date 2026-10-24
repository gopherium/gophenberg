// SPDX-License-Identifier: Apache-2.0

import * as wpds from '@wordpress/ui'
import { expect, test } from 'vitest'

import { Spinner } from '../index'

test('hands plugins the spinner WPDS marks a step under way with', () => {
	expect(Spinner).toBe(wpds.Spinner)
})
