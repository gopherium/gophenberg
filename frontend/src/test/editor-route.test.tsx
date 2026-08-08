// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/posts/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../posts/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/posts/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('the editor route carries none of the admin chrome', async () => {
	renderAt(EDITOR_PATH)

	await screen.findByTitle('Editor canvas')

	expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
	expect(screen.queryByRole('link', { name: 'Gophenberg' })).not.toBeInTheDocument()
	expect(document.querySelector('.godmin-layout')).toBeNull()
	expect(document.querySelector('.godmin-layout__canvas')).toBeNull()
})

test('framed routes still render the admin chrome', async () => {
	renderAt('/')

	expect(await screen.findByRole('navigation')).toBeInTheDocument()
	expect(document.querySelector('.godmin-layout')).not.toBeNull()
})
