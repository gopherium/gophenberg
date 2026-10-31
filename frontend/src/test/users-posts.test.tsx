// SPDX-License-Identifier: Apache-2.0

import { Link } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { adminBasepath } from '../basepath'
import { catalogFor } from '../i18n/catalog'
import { classesOf } from './classes'
import { renderAt } from './render'
import { POST_TYPE, PAGE_TYPE, serveTypes } from './typeFixtures'
import { EDITOR, SIGNED_IN, rowOf, serveAccounts, servePostCounts, warmUsersScreen } from './usersFixtures'

warmUsersScreen()

beforeEach(() => serveAccounts(SIGNED_IN, EDITOR))

/**
 * Returns the link the Posts cell of a row holds.
 * @param name - The name of the account the row shows.
 * @returns The link.
 */
async function postsLink(name: string) {
	return (await rowOf(name)).findByRole('link')
}

test('counts the posts of each account in a column named after the post type', async () => {
	servePostCounts({ [EDITOR.id]: 3 })
	renderAt('/users')

	expect(await screen.findByRole('columnheader', { name: 'Posts' })).toBeInTheDocument()
	expect(await postsLink(EDITOR.name)).toHaveTextContent('3')
})

test('names the column as the site names its post type', async () => {
	serveTypes({ ...POST_TYPE, plural_label: 'Articles' })
	servePostCounts({ [EDITOR.id]: 3 })
	renderAt('/users')

	expect(await screen.findByRole('columnheader', { name: 'Articles' })).toBeInTheDocument()
})

test('names the column in the reader s language while the post type keeps its shipped name', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	servePostCounts({ [EDITOR.id]: 3 })
	renderAt('/users')

	expect(await screen.findByRole('columnheader', { name: 'Entradas' })).toBeInTheDocument()
})

test('links a count to the posts list narrowed to that author', async () => {
	servePostCounts({ [EDITOR.id]: 3 })
	renderAt('/users')

	const address = new URL((await postsLink(EDITOR.name)).getAttribute('href') as string, 'https://example.com')

	expect(address.pathname).toBe(`${adminBasepath}/content/post`)
	expect(JSON.parse(address.searchParams.get('filters') as string)).toEqual([
		{ field: 'author', operator: 'isAny', value: [EDITOR.id] },
	])
})

test('draws a count as the admin draws a link, not as a bare browser link', async () => {
	servePostCounts({ [EDITOR.id]: 3 })
	renderAt('/users')

	expect((await postsLink(EDITOR.name)).className).toBe(classesOf(<Link href="/">3</Link>))
})

test('tells a screen reader whose posts a count holds, as WordPress does', async () => {
	servePostCounts({ [EDITOR.id]: 3, [SIGNED_IN.id]: 1 })
	renderAt('/users')

	expect(await postsLink(EDITOR.name)).toHaveAccessibleName('3 posts by this author')
	expect(await postsLink(SIGNED_IN.name)).toHaveAccessibleName('1 post by this author')
})

test('writes a count in the site s number format', async () => {
	servePostCounts({ [EDITOR.id]: 12345 })
	renderAt('/users')

	expect(await postsLink(EDITOR.name)).toHaveAccessibleName('12.345 posts by this author')
})

test('shows a plain 0 with no link for an account that wrote no post', async () => {
	servePostCounts({ [EDITOR.id]: 3 })
	renderAt('/users')

	const row = await rowOf(SIGNED_IN.name)

	expect(await row.findByText('0')).toBeInTheDocument()
	expect(row.queryByRole('link')).toBeNull()
})

test('leaves the Posts cells empty when the counts cannot be read', async () => {
	server.use(http.get('/api/users/post-counts', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/users')

	await screen.findByRole('columnheader', { name: 'Posts' })

	expect((await rowOf(EDITOR.name)).queryByText('0')).toBeNull()
})

test.each([
	{ when: 'the post type is turned off', types: [{ ...POST_TYPE, active: false, default: false }, PAGE_TYPE] },
	{ when: 'no post type exists', types: [PAGE_TYPE] },
])('draws no column of counts when $when, not even one named after another type', async ({ types }) => {
	serveTypes(...types)
	renderAt('/users')

	await rowOf(EDITOR.name)
	await within(await screen.findByRole('region', { name: 'Navigation' })).findByRole('link', { name: 'Pages' })

	const headers = screen.getAllByRole('columnheader').map((header) => header.textContent)

	expect(headers.filter(Boolean)).toEqual(['Name', 'Status', 'Role', expect.stringMatching(/^Created/), 'Actions'])
})
