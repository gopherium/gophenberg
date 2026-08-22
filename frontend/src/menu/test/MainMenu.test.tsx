// SPDX-License-Identifier: Apache-2.0

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
	Outlet,
	RouterProvider,
	createMemoryHistory,
	createRootRoute,
	createRoute,
	createRouter,
} from '@tanstack/react-router'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { render, screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'

import { plugins } from '../../plugins'
import { coreNav } from '../coreNav'
import { MainMenu } from '../MainMenu'

const navItems = [...coreNav, ...plugins.flatMap((plugin) => plugin.nav)]

function renderMenuAt(path: string, withSections = false) {
	const rootRoute = createRootRoute({
		component: function MenuHost() {
			return (
				<>
					<nav aria-label="Navigation">
						<MainMenu />
					</nav>
					<Outlet />
				</>
			)
		},
	})
	const routes = [{ to: '/' }, ...navItems].map((item) =>
		createRoute({
			getParentRoute: () => rootRoute,
			path: item.to,
			staticData: withSections
				? {
						Sidebar: function SectionSidebar() {
							return null
						},
					}
				: {},
			component: function Blank() {
				return null
			},
		}),
	)
	const router = createRouter({
		routeTree: rootRoute.addChildren(routes),
		history: createMemoryHistory({ initialEntries: [path] }),
	})
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	seedSession(client, { ...defaultUser, role: 'admin' })
	render(
		<QueryClientProvider client={client}>
			<RouterProvider router={router} />
		</QueryClientProvider>,
	)
}

test('renders a menu link for every core and plugin nav entry', async () => {
	renderMenuAt('/')

	const nav = await screen.findByRole('navigation', { name: 'Navigation' })
	expect(within(nav).getAllByRole('link')).toHaveLength(navItems.length)
	for (const item of navItems) {
		expect(
			within(nav).getByRole('link', { name: item.label }),
		).toBeInTheDocument()
	}
})

test('marks the item for the active route as current', async () => {
	const [target] = navItems
	renderMenuAt(target.to)

	const link = await screen.findByRole('link', { name: target.label })
	expect(link).toHaveAttribute('aria-current', 'page')
})

test('omits the drill-down chevron when the target has no section screen', async () => {
	renderMenuAt('/')

	const link = await screen.findByRole('link', { name: navItems[0].label })
	expect(link.querySelector('.gophenberg-menu__chevron')).toBeNull()
})

test('marks entries whose target declares a section screen with a chevron', async () => {
	renderMenuAt('/', true)

	const link = await screen.findByRole('link', { name: navItems[0].label })
	expect(link.querySelector('.gophenberg-menu__chevron')).not.toBeNull()
})
