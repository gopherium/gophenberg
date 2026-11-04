import * as wpds from '@wordpress/ui'
import { expect, test } from 'vitest'

import { Popover } from '../index'

test('hands plugins the popover WPDS anchors a small panel to a button with', () => {
	expect(Popover).toBe(wpds.Popover)
})
