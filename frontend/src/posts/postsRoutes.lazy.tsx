// SPDX-License-Identifier: Apache-2.0

import { createLazyRoute } from '@tanstack/react-router'

import { EditorScreen } from './EditorScreen'
import { PostsScreen } from './PostsScreen'

export const PostsLazyRoute = createLazyRoute('/posts')({
	component: PostsScreen,
})

export const EditorLazyRoute = createLazyRoute('/posts/$postId/edit')({
	component: EditorScreen,
})
