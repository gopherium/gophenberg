// SPDX-License-Identifier: Apache-2.0

import { setViewport } from '@gopherium/godmin/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { renderAt } from './render'

test('holds the navigation behind a menu button on a narrow viewport', async () => {
	setViewport({ matches: true })

	renderAt('/')

	expect(await screen.findByRole('button', { name: 'Open navigation' })).toBeInTheDocument()
	expect(screen.queryByRole('navigation')).toBeNull()
})

test('opens the navigation in a drawer', async () => {
	setViewport({ matches: true })
	renderAt('/')

	await userEvent.click(await screen.findByRole('button', { name: 'Open navigation' }))

	const drawer = await screen.findByRole('dialog')
	expect(within(drawer).getByRole('navigation')).toBeInTheDocument()
	expect(within(drawer).getByRole('link', { name: 'Posts' })).toBeInTheDocument()
})

test('puts the navigation away once it takes the reader somewhere', async () => {
	setViewport({ matches: true })
	renderAt('/')
	await userEvent.click(await screen.findByRole('button', { name: 'Open navigation' }))
	const drawer = await screen.findByRole('dialog')

	await userEvent.click(within(drawer).getByRole('link', { name: 'Posts' }))

	expect(await screen.findByRole('button', { name: 'Open navigation' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).toBeNull()
})

test('keeps the rail beside the canvas when there is room', async () => {
	setViewport({ matches: false })

	renderAt('/')

	expect(await screen.findByRole('navigation')).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Open navigation' })).toBeNull()
})
