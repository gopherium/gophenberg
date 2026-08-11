// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import editorScreenSource from '../posts/EditorScreen.tsx?raw'

test('the editor screen installs the api-fetch guard beside its blocks', () => {
	expect(editorScreenSource).toContain('installApiFetchGuard()')
	expect(editorScreenSource).toContain('registerCuratedBlocks()')
})
