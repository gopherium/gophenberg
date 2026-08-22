// SPDX-License-Identifier: Apache-2.0

import { beforeAll } from 'vitest'

/** Loads the posts screen before a file renders its lazily mounted route. */
export function warmPostsScreen() {
	beforeAll(async () => {
		await import('../content/PostsScreen')
	})
}
