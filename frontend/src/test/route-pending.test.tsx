// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { createAppRouter } from '../router'

const SPLIT_ROUTES = [
	'/framed/content/$typeKey',
	'/framed/content-types',
	'/framed/users',
	'/framed/users/new',
	'/content/$typeKey/$postId/edit',
]

test('splits every heavy route behind a component the pending seam can wait on', () => {
	const router = createAppRouter()

	for (const path of SPLIT_ROUTES) {
		const route = router.routesById[path as keyof typeof router.routesById]
		const component = route.options.component as { preload?: unknown } | undefined
		expect(typeof component?.preload, `${path} cannot arm the pending seam`).toBe('function')
	}
})
