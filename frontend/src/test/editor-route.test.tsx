// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { renderAt } from './render'

const EDITOR_PATH = '/posts/019fb000-0000-7000-8000-000000000001/edit'

test('the editor route carries none of the admin chrome', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('heading', { name: /editor/i })).toBeInTheDocument()
	expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
	expect(screen.queryByRole('link', { name: 'Gophenberg' })).not.toBeInTheDocument()
	expect(document.querySelector('.gophenberg-layout')).toBeNull()
	expect(document.querySelector('.gophenberg-layout__canvas')).toBeNull()
})

test('the editor route names the post it will edit', async () => {
	renderAt(EDITOR_PATH)

	expect(
		await screen.findByText(/019fb000-0000-7000-8000-000000000001/),
	).toBeInTheDocument()
})

test('framed routes still render the admin chrome', async () => {
	renderAt('/')

	expect(await screen.findByRole('navigation')).toBeInTheDocument()
	expect(document.querySelector('.gophenberg-layout')).not.toBeNull()
})
