// SPDX-License-Identifier: Apache-2.0

import { configure } from '@testing-library/react'
import { installTestEnvironment } from '@gophenberg/frontend-sdk/testing'
import { beforeAll } from 'vitest'

installTestEnvironment()
configure({ asyncUtilTimeout: 2000 })

beforeAll(async () => {
	await Promise.all([import('../posts/postsRoutes.lazy'), import('../userRoutes.lazy')])
})
