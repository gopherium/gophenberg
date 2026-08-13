// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { renderAt } from './render'

test('the main menu offers the posts section', async () => {
	renderAt('/')

	expect(await screen.findByRole('link', { name: 'Posts' })).toBeInTheDocument()
})

test('the posts section replaces the menu with its own drill-down screen', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
	expect(screen.getByRole('link', { name: 'All Posts' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Add New' })).toBeInTheDocument()
})

test('the drill-down screen returns to the main menu', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('link', { name: 'Back' })).toHaveAttribute('href', '/admin/')
})

test('the drill-down screen takes the reader to its title on arrival', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toHaveFocus()
})

test('the main menu is hidden while the posts section is open', async () => {
	renderAt('/content/post')

	await screen.findByRole('heading', { name: 'Posts', level: 2 })

	expect(screen.queryByRole('link', { name: 'Users' })).not.toBeInTheDocument()
})
