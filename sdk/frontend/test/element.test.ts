// SPDX-License-Identifier: Apache-2.0

import { assertElementPatched } from '@gopherium/godmin/testing'
import { expect, test } from 'vitest'

test('the design system runs on React 19 in this workspace', async () => {
	await expect(assertElementPatched()).resolves.toBeUndefined()
})
