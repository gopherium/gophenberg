// SPDX-License-Identifier: Apache-2.0

import { createLazyRoute } from '@tanstack/react-router'

import { EditorScreen } from './EditorScreen'

export const EditorLazyRoute = createLazyRoute('/posts/$postId/edit')({
	component: EditorScreen,
})
