// SPDX-License-Identifier: Apache-2.0

import { createLazyRoute } from '@tanstack/react-router'

import { PostsScreen } from './PostsScreen'

export const PostsLazyRoute = createLazyRoute('/posts')({
	component: PostsScreen,
})
