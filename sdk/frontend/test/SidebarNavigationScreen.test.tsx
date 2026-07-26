// SPDX-License-Identifier: Apache-2.0

import {
	RouterProvider,
	createMemoryHistory,
	createRootRoute,
	createRoute,
	createRouter,
} from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { expect, test } from 'vitest'

import { SidebarNavigationScreen } from '../SidebarNavigationScreen'

function renderScreen(ui: ReactNode) {
	const rootRoute = createRootRoute({
		component: function Host() {
			return <>{ui}</>
		},
	})
	const indexRoute = createRoute({
		getParentRoute: () => rootRoute,
		path: '/',
		component: function Blank() {
			return null
		},
	})
	const router = createRouter({
		routeTree: rootRoute.addChildren([indexRoute]),
		history: createMemoryHistory({ initialEntries: ['/'] }),
	})
	return render(<RouterProvider router={router} />)
}

test('renders the title, back link, description, actions, and content', async () => {
	renderScreen(
		<SidebarNavigationScreen
			title="Posts"
			backTo="/"
			description="Your posts"
			actions={<button type="button">New</button>}
			footer={<span>footer text</span>}
		>
			<p>screen content</p>
		</SidebarNavigationScreen>,
	)

	expect(
		await screen.findByRole('heading', { name: 'Posts' }),
	).toBeInTheDocument()
	expect(screen.getByRole('link', { name: 'Back' })).toBeInTheDocument()
	expect(screen.getByText('Your posts')).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'New' })).toBeInTheDocument()
	expect(screen.getByText('screen content')).toBeInTheDocument()
	expect(screen.getByText('footer text')).toBeInTheDocument()
	expect(screen.queryByRole('contentinfo')).toBeNull()
	expect(screen.getByRole('heading', { name: 'Posts' })).toHaveFocus()
})

test('omits the back link and optional regions on a root screen', async () => {
	const { container } = renderScreen(
		<SidebarNavigationScreen title="Home">
			<p>root content</p>
		</SidebarNavigationScreen>,
	)

	expect(
		await screen.findByRole('heading', { name: 'Home' }),
	).toBeInTheDocument()
	expect(screen.queryByRole('link', { name: 'Back' })).toBeNull()
	expect(screen.getByText('root content')).toBeInTheDocument()
	expect(
		container.querySelector('.gophenberg-nav-screen__description'),
	).toBeNull()
	expect(container.querySelector('.gophenberg-nav-screen__actions')).toBeNull()
	expect(container.querySelector('.gophenberg-nav-screen__footer')).toBeNull()
})
