// SPDX-License-Identifier: Apache-2.0

import { configure } from '@testing-library/react'
import { installTestEnvironment } from '@gophenberg/frontend-sdk/testing'
import { beforeAll } from 'vitest'

installTestEnvironment()
configure({ asyncUtilTimeout: 2000 })

beforeAll(async () => {
	await Promise.all([
		import('../content/PostsScreen'),
		import('../media/MediaScreen'),
		import('../users/UsersScreen'),
		import('../users/NewUserScreen'),
		import('../themes/ThemesScreen'),
	])
})
