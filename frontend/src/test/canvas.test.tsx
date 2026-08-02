// SPDX-License-Identifier: Apache-2.0

import type { CanvasMode } from '@gophenberg/frontend-sdk'
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

import '../index.css'
import { Layout } from '../Layout'
import { versionQueryKey } from '../version'

const BLEED = 'godmin-layout__canvas--bleed'

/**
 * Renders a section whose route and parent declare the given canvas modes.
 * @param section - The mode the section route declares, or nothing.
 * @param outer - The mode the route above it declares, or nothing.
 */
function renderCanvas(section?: CanvasMode, outer?: CanvasMode) {
	const rootRoute = createRootRoute({ component: Layout })
	const outerRoute = createRoute({
		getParentRoute: () => rootRoute,
		id: 'outer',
		staticData: outer === undefined ? {} : { canvas: outer },
	})
	const sectionRoute = createRoute({
		getParentRoute: () => outerRoute,
		path: '/section',
		staticData: section === undefined ? {} : { canvas: section },
		component: function SectionCanvas() {
			return <p>section canvas</p>
		},
	})
	const router = createRouter({
		routeTree: rootRoute.addChildren([outerRoute.addChildren([sectionRoute])]),
		history: createMemoryHistory({ initialEntries: ['/section'] }),
	})
	const client = createAuthQueryClient({ queries: { retry: false, staleTime: Infinity } })
	seedSession(client, defaultUser)
	client.setQueryData(versionQueryKey, '0.0.0')
	render(
		<QueryClientProvider client={client}>
			<RouterProvider router={router} />
		</QueryClientProvider>,
	)
}

/**
 * Returns the canvas element the layout rendered.
 * @returns The canvas element.
 */
function canvas(): HTMLElement {
	return document.querySelector('.godmin-layout__canvas') as HTMLElement
}

test('pads the canvas of a route that asks for nothing', async () => {
	renderCanvas()

	expect(await screen.findByText('section canvas')).toBeInTheDocument()
	expect(canvas()).not.toHaveClass(BLEED)
})

test('bleeds the canvas where the route asks for it', async () => {
	renderCanvas('bleed')

	expect(await screen.findByText('section canvas')).toBeInTheDocument()
	expect(canvas()).toHaveClass(BLEED)
})

test('pads the canvas where the route asks for it', async () => {
	renderCanvas('padded')

	expect(await screen.findByText('section canvas')).toBeInTheDocument()
	expect(canvas()).not.toHaveClass(BLEED)
})

test('takes the canvas mode from the nearest route that declares one', async () => {
	renderCanvas('padded', 'bleed')

	expect(await screen.findByText('section canvas')).toBeInTheDocument()
	expect(canvas()).not.toHaveClass(BLEED)
})

test('inherits the canvas mode when the route declares none', async () => {
	renderCanvas(undefined, 'bleed')

	expect(await screen.findByText('section canvas')).toBeInTheDocument()
	expect(canvas()).toHaveClass(BLEED)
})
