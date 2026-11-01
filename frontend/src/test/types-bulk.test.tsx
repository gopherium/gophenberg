// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { setViewport } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { bulkBar } from './bulkBar'
import { renderAt } from './render'
import {
	GUIDE_TYPE,
	PAGE_TYPE,
	POST_TYPE,
	SPANISH,
	findToast,
	rowOf,
	serveTypes,
	warmTypesScreen,
} from './typeFixtures'
import type { Reader } from './typeFixtures'

warmTypesScreen()

const asked: string[] = []

beforeEach(() => {
	asked.length = 0
	serveTypes(POST_TYPE, PAGE_TYPE, GUIDE_TYPE)
	server.use(
		http.delete('/api/types/:key', ({ params }) => {
			asked.push(`delete ${String(params.key)}`)
			return new HttpResponse(null, { status: 204 })
		}),
		http.patch('/api/types/:key', async ({ params, request }) => {
			asked.push(`edit ${String(params.key)} ${JSON.stringify(await request.json())}`)
			return HttpResponse.json(params.key === 'page' ? PAGE_TYPE : GUIDE_TYPE)
		}),
	)
})

/**
 * Ticks the rows of the given types.
 * @param names - The names the rows show.
 * @param reader - How the reader finds the list, an English one unless a test names another.
 */
async function pick(names: string[], reader?: Reader) {
	for (const name of names) {
		await userEvent.click((await rowOf(name, reader)).getByRole('checkbox'))
	}
}

/**
 * Returns the words of the action buttons the bulk bar holds, leaving out its unworded Cancel.
 * @returns The labels, in order.
 */
function bulkActions(): (string | null)[] {
	return bulkBar()
		.getAllByRole('button')
		.map((button) => button.textContent)
		.filter((label) => label !== '')
}

/**
 * Refuses every request of the given kind made on the Guides type.
 * @param kind - The request refused, the delete or the edit.
 */
function refuseGuides(kind: 'delete' | 'patch') {
	server.use(
		http[kind]('/api/types/guide', () => HttpResponse.json({ error: 'content: the type is in use' }, { status: 422 })),
	)
}

test('offers Activate, Deactivate and Delete… on the picked types, each only where some type takes it', async () => {
	serveTypes(POST_TYPE, PAGE_TYPE, { ...GUIDE_TYPE, active: false })
	renderAt('/content-types')

	await pick(['Pages'])
	expect(bulkActions()).toEqual(['Deactivate', 'Delete…'])
	await pick(['Guides'])

	expect(bulkActions()).toEqual(['Activate', 'Deactivate', 'Delete…'])
})

test('leaves the default type out of the picks, since no bulk action reaches it', async () => {
	renderAt('/content-types')

	expect((await rowOf('Posts')).getByRole('checkbox')).toHaveAttribute('aria-disabled', 'true')
})

test('deletes every picked type behind one confirm, counting them as content types in one toast', async () => {
	renderAt('/content-types')
	await pick(['Pages', 'Guides'])

	await userEvent.click(bulkBar().getByRole('button', { name: 'Delete…' }))
	const dialog = await screen.findByRole('dialog')
	expect(within(dialog).getByText('Delete 2 content types for good? This cannot be undone.')).toBeInTheDocument()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	expect(await findToast('2 content types permanently deleted.')).toBeInTheDocument()
	expect([...asked].sort()).toEqual(['delete guide', 'delete page'])
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('says only what a partly refused bulk delete could not remove, in one toast', async () => {
	refuseGuides('delete')
	renderAt('/content-types')
	await pick(['Pages', 'Guides'])

	await userEvent.click(bulkBar().getByRole('button', { name: 'Delete…' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Delete' }))

	expect(await findToast('1 content type could not be permanently deleted.')).toBeInTheDocument()
	expect(screen.queryByText('1 content type permanently deleted.')).not.toBeInTheDocument()
})

test('counts the content types a bulk delete removed in Spanish, with no word agreeing with a name', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content-types')
	await pick(['Páginas', 'Guides'], SPANISH)

	await userEvent.click(bulkBar().getByRole('button', { name: 'Borrar…' }))
	const dialog = await screen.findByRole('dialog')
	expect(
		within(dialog).getByText('¿Borrar permanentemente 2 tipos de contenido? Esto no se puede deshacer.'),
	).toBeInTheDocument()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Borrar' }))

	expect(await findToast('Se han borrado permanentemente 2 tipos de contenido.')).toBeInTheDocument()
})

test('closes every picked type at once, counting them in one toast', async () => {
	renderAt('/content-types')
	await pick(['Pages', 'Guides'])

	await userEvent.click(bulkBar().getByRole('button', { name: 'Deactivate' }))

	expect(await screen.findByText('2 content types deactivated.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect([...asked].sort()).toEqual(['edit guide {"active":false}', 'edit page {"active":false}'])
})

test('reopens every picked closed type at once, counting them in one toast', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, active: false }, { ...GUIDE_TYPE, active: false })
	renderAt('/content-types')
	await pick(['Pages', 'Guides'])

	await userEvent.click(bulkBar().getByRole('button', { name: 'Activate' }))

	expect(await screen.findByText('2 content types activated.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect([...asked].sort()).toEqual(['edit guide {"active":true}', 'edit page {"active":true}'])
})

test('switches only the picked types a switch reaches', async () => {
	serveTypes(POST_TYPE, PAGE_TYPE, { ...GUIDE_TYPE, active: false })
	renderAt('/content-types')
	await pick(['Pages', 'Guides'])

	await userEvent.click(bulkBar().getByRole('button', { name: 'Activate' }))

	expect(await screen.findByText('"Guides" activated.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(asked).toEqual(['edit guide {"active":true}'])
})

test.each([
	{ action: 'Deactivate', active: true, toast: '1 content type could not be deactivated.' },
	{ action: 'Activate', active: false, toast: '1 content type could not be activated.' },
])('says only what a partly refused $action could not switch, in one toast', async ({ action, active, toast }) => {
	refuseGuides('patch')
	serveTypes(POST_TYPE, { ...PAGE_TYPE, active }, { ...GUIDE_TYPE, active })
	renderAt('/content-types')
	await pick(['Pages', 'Guides'])

	await userEvent.click(bulkBar().getByRole('button', { name: action }))

	expect(await screen.findByText(toast, { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('keeps Deactivate and Delete… in the bulk bar below 782px, where only actions with an icon show', async () => {
	setViewport({ width: 700 })
	renderAt('/content-types')

	await pick(['Pages', 'Guides'])

	expect(bulkBar().getByRole('button', { name: 'Deactivate' })).toBeInTheDocument()
	expect(bulkBar().getByRole('button', { name: 'Delete…' })).toBeInTheDocument()
})

test('keeps Activate in the bulk bar below 782px, where only actions with an icon show', async () => {
	setViewport({ width: 700 })
	serveTypes(POST_TYPE, { ...PAGE_TYPE, active: false }, { ...GUIDE_TYPE, active: false })
	renderAt('/content-types')

	await pick(['Pages', 'Guides'])

	expect(bulkBar().getByRole('button', { name: 'Activate' })).toBeInTheDocument()
})

test('puts the focus back on the bulk bar after a switch, since every row stays in the list', async () => {
	renderAt('/content-types')
	await pick(['Pages', 'Guides'])
	const deactivate = bulkBar().getByRole('button', { name: 'Deactivate' })

	await userEvent.click(deactivate)

	await screen.findByText('2 content types deactivated.', { selector: '.godmin-toast' })
	await waitFor(() => expect(bulkBar().getByRole('button', { name: 'Deactivate' })).toHaveFocus())
})

test('lands the focus on the list once a bulk Activate takes the picked types out of the Closed filter', async () => {
	let opened = false
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, active: opened }, { ...GUIDE_TYPE, active: opened }] }),
		),
		http.patch('/api/types/:key', () => {
			opened = true
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	const closed = encodeURIComponent('[{"field":"status","operator":"isAny","value":["closed"]}]')
	renderAt(`/content-types?filters=${closed}`)
	await pick(['Pages', 'Guides'])

	await userEvent.click(bulkBar().getByRole('button', { name: 'Activate' }))

	await findToast('2 content types activated.')
	await waitFor(() => expect(screen.getByRole('region', { name: 'Content Types' })).toHaveFocus())
})

test('counts the types a bulk switch reached in Spanish, with no word agreeing with a name', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content-types')
	await pick(['Páginas', 'Guides'], SPANISH)

	await userEvent.click(bulkBar().getByRole('button', { name: 'Desactivar' }))

	expect(await findToast('Se han desactivado 2 tipos de contenido.')).toBeInTheDocument()
})
