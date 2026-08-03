// SPDX-License-Identifier: Apache-2.0

import { screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'

import { coreNav } from '../menu/coreNav'
import { plugins } from '../plugins'
import { createAppRouter } from '../router'
import { renderAt } from './render'

test('mounts the application under the admin basepath', () => {
	expect(createAppRouter().options.basepath).toBe('/admin')
})

test('serves the home screen at the root path', async () => {
	renderAt('/')

	expect(await screen.findByText(/welcome to gophenberg/i)).toBeInTheDocument()
})

test('leaves the heading rank to the screen, not the masthead', async () => {
	renderAt('/')

	expect(await screen.findByRole('link', { name: 'Gophenberg' })).toBeInTheDocument()
	expect(screen.queryByRole('heading', { name: 'Gophenberg' })).toBeNull()
})

test('renders a navigation entry for every core and plugin section', async () => {
	renderAt('/')

	const nav = await screen.findByRole('navigation')
	expect(within(nav).queryAllByRole('link')).toHaveLength(
		coreNav.length + plugins.flatMap((plugin) => plugin.nav).length,
	)
})

test('frames the active route inside the main content region', async () => {
	renderAt('/')

	const main = await screen.findByRole('main')
	expect(within(main).getByText(/welcome to gophenberg/i)).toBeInTheDocument()
	expect(within(main).queryByRole('navigation')).toBeNull()
})
