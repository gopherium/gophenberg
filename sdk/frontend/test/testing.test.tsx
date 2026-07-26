// SPDX-License-Identifier: Apache-2.0

import { createRoute } from '@tanstack/react-router'
import { screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import type { FrontendPlugin } from '../index'
import { renderPluginAt } from '../testing'

const sectionPlugin: FrontendPlugin = {
	id: 'section',
	nav: [
		{
			label: 'Section',
			to: '/section',
			icon: <svg viewBox="0 0 24 24" />,
		},
	],
	routes: (parent) => [
		createRoute({
			getParentRoute: () => parent,
			path: '/section',
			staticData: {
				Sidebar: function SectionSidebar() {
					return <p>section sidebar</p>
				},
			},
			component: function SectionCanvas() {
				return <p>section canvas</p>
			},
		}),
	],
}

test('renders the plugin nav on routes without a section screen', async () => {
	renderPluginAt(sectionPlugin, '/')

	expect(await screen.findByText('Test host home')).toBeInTheDocument()
	expect(
		screen.getByRole('link', { name: 'Section' }),
	).toBeInTheDocument()
})

test('renders the section sidebar and canvas on a section route', async () => {
	renderPluginAt(sectionPlugin, '/section')

	expect(await screen.findByText('section sidebar')).toBeInTheDocument()
	expect(screen.getByText('section canvas')).toBeInTheDocument()
})
