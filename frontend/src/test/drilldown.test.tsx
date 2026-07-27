// SPDX-License-Identifier: Apache-2.0

import { createAuthQueryClient } from '@gopherium/react-auth'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { QueryClientProvider } from '@tanstack/react-query'
import {
	RouterProvider,
	createMemoryHistory,
	createRootRoute,
	createRoute,
	createRouter,
} from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { Layout } from '../Layout'
import { versionQueryKey } from '../version'

function renderSectionAt(path: string) {
	const rootRoute = createRootRoute({ component: Layout })
	const homeRoute = createRoute({
		getParentRoute: () => rootRoute,
		path: '/',
		component: function SectionHome() {
			return <p>home canvas</p>
		},
	})
	const sectionRoute = createRoute({
		getParentRoute: () => rootRoute,
		path: '/section',
		staticData: {
			Sidebar: function SectionSidebar() {
				return <p>section sidebar</p>
			},
		},
		component: function SectionCanvas() {
			return <p>section canvas</p>
		},
	})
	const router = createRouter({
		routeTree: rootRoute.addChildren([homeRoute, sectionRoute]),
		history: createMemoryHistory({ initialEntries: [path] }),
	})
	const client = createAuthQueryClient({
		queries: { retry: false, staleTime: Infinity },
	})
	seedSession(client, defaultUser)
	client.setQueryData(versionQueryKey, '0.0.0')
	render(
		<QueryClientProvider client={client}>
			<RouterProvider router={router} />
		</QueryClientProvider>,
	)
}

test('swaps the sidebar for the section screen on a section route', async () => {
	renderSectionAt('/section')

	expect(await screen.findByText('section sidebar')).toBeInTheDocument()
	expect(screen.getByText('section canvas')).toBeInTheDocument()
	expect(screen.queryByRole('link', { name: 'Users' })).toBeNull()
})

test('shows the main menu, not a section screen, at the root', async () => {
	renderSectionAt('/')

	expect(await screen.findByText('home canvas')).toBeInTheDocument()
	expect(screen.queryByText('section sidebar')).toBeNull()
})
