// SPDX-License-Identifier: Apache-2.0

import { installTestEnvironment } from '@gophenberg/frontend-sdk/testing'
import { beforeAll } from 'vitest'

installTestEnvironment()

beforeAll(async () => {
	await Promise.all([
		import('../posts/postsRoutes.lazy'),
		import('../posts/editorRoute.lazy'),
		import('../userRoutes.lazy'),
	])
})
