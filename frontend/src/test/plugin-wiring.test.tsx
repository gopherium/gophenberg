// SPDX-License-Identifier: Apache-2.0

import type { AnyRoute } from '@tanstack/react-router'
import { screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { renderAt } from './render'

vi.mock('../plugins', async () => {
	const { createRoute } = await import('@tanstack/react-router')
	const { createElement } = await import('react')
	const plugin = {
		id: 'demo',
		nav: [
			{
				label: 'Demo',
				to: '/demo',
				icon: createElement('svg', { viewBox: '0 0 24 24' }),
			},
		],
		routes: (parent: AnyRoute) => [
			createRoute({
				getParentRoute: () => parent,
				path: '/demo',
				component: function DemoCanvas() {
					return createElement('p', null, 'demo canvas')
				},
			}),
		],
	}
	return { plugins: [plugin] }
})

test('mounts plugin routes in the app router', async () => {
	renderAt('/demo')

	expect(await screen.findByText('demo canvas')).toBeInTheDocument()
})

test('renders plugin nav entries in the menu', async () => {
	renderAt('/')

	expect(
		await screen.findByRole('link', { name: 'Demo' }),
	).toBeInTheDocument()
})
