// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses, stubLayout } from '@gopherium/godmin/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { renderAt } from './render'
import {
	PAGE_TYPE,
	POST_TYPE,
	findToast,
	openRowConfirm,
	openRowMenu,
	rowOf,
	serveTypes,
	waitBusy,
	warmTypesScreen,
} from './typeFixtures'

warmTypesScreen()

beforeEach(() => {
	serveTypes(POST_TYPE, PAGE_TYPE)
})

/** Answers a delete the registry let through. */
const gone = () => new HttpResponse(null, { status: 204 })

/**
 * Records every delete the registry is asked for and answers each with the given response.
 * @param answer - Builds the response each delete gets.
 * @returns The paths asked for, filled as requests arrive.
 */
function recordDeletes(answer: () => Response | Promise<Response> = gone): string[] {
	const asked: string[] = []
	server.use(
		http.delete('/api/types/:key', ({ request }) => {
			asked.push(new URL(request.url).pathname)
			return answer()
		}),
	)
	return asked
}

/**
 * Opens the delete confirm of the Pages row.
 * @returns The confirm dialog.
 */
function openDelete(): Promise<HTMLElement> {
	return openRowConfirm('Pages', 'Delete…')
}

test('asks before deleting a type in a confirm titled by the question, and deletes nothing yet', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()

	expect(within(dialog).getByRole('heading', { name: 'Delete "Pages"?' })).toBeInTheDocument()
	expect(dialog).toHaveAccessibleDescription('This action cannot be undone.')
	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	expect(asked).toEqual([])
})

test('draws Delete red after a neutral Cancel with no border, since a deleted type cannot come back', async () => {
	renderAt('/content-types')

	const dialog = await openDelete()

	const [cancel, confirm] = within(dialog).getAllByRole('button')
	expect([cancel.textContent, confirm.textContent]).toEqual(['Cancel', 'Delete'])
	expect([...cancel.classList]).toEqual(buttonClasses('minimal', undefined, 'neutral'))
	expect(confirm.className).toMatch(/irreversible/)
})

test('opens the delete confirm on Cancel, as WordPress opens its row confirms', async () => {
	stubLayout()
	renderAt('/content-types')

	const dialog = await openDelete()

	await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus())
})

test('deletes nothing when the confirm is cancelled', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
	expect(asked).toEqual([])
})

test('deletes a type once confirmed and names it in a toast', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	await waitFor(() => expect(asked).toEqual(['/api/types/page']))
	expect(await screen.findByText('"Pages" permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
})

test('keeps the confirm busy with Cancel greyed while the delete runs, then closes it', async () => {
	const answer = gate()
	recordDeletes(async () => {
		await answer.held
		return new HttpResponse(null, { status: 204 })
	})
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	await waitBusy(dialog, 'Delete')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
	answer.release()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
})

test.each([
	{
		locale: 'es-ES',
		region: 'Tipos de contenido',
		pages: 'Páginas',
		remove: 'Borrar…',
		confirm: 'Borrar',
		removed: 'Se ha borrado permanentemente el tipo de contenido «Páginas».',
	},
	{
		locale: 'fr-FR',
		region: 'Types de contenu',
		pages: 'Pages',
		remove: 'Supprimer…',
		confirm: 'Supprimer',
		removed: 'Type de contenu « Pages » supprimé définitivement.',
	},
])('names a removed built-in type by its kind for a $locale reader, no word agreeing with it', async (reader) => {
	setLocaleData(await catalogFor(reader.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	recordDeletes()
	renderAt('/content-types')

	const dialog = await openRowConfirm(reader.pages, reader.remove, reader)
	await userEvent.click(within(dialog).getByRole('button', { name: reader.confirm }))

	expect(await screen.findByText(reader.removed, { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('lands the focus on the row in its place once the deleted type has left the list', async () => {
	let deleted = false
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: deleted ? [POST_TYPE] : [POST_TYPE, PAGE_TYPE] })),
		http.delete('/api/types/:key', () => {
			deleted = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	expect(await screen.findByText('"Pages" permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
	const list = screen.getByRole('region', { name: 'Content Types' })
	await waitFor(() => expect(within(list).queryByText('Pages', { exact: true })).not.toBeInTheDocument())
	await waitFor(() => expect(within(list).getByRole('button', { name: 'Actions' })).toHaveFocus())
})

test('shows the reason a type could not be deleted in a toast, the confirm closed and the row kept', async () => {
	recordDeletes(() =>
		HttpResponse.json({ error: 'content: the type is in use', code: 'type_in_use' }, { status: 422 }),
	)
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	expect(
		await findToast('This content type still holds content, so it cannot be deleted. Move or delete its items first.'),
	).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
	expect(await rowOf('Pages')).toBeTruthy()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('says the registry could not be reached when the network drops a delete', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	recordDeletes(() => HttpResponse.error())
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	expect(await findToast('The registry could not be reached.')).toBeInTheDocument()
})

test('deletes a type only once an edit of it still in flight has answered', async () => {
	const edit = gate()
	const heard: string[] = []
	server.use(
		http.patch('/api/types/page', async () => {
			heard.push('edit sent')
			await edit.held
			heard.push('edit answered')
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	recordDeletes(() => {
		heard.push('delete sent')
		return new HttpResponse(null, { status: 204 })
	})
	renderAt('/content-types')
	await openRowMenu('Pages')
	await userEvent.click(screen.getByRole('menuitem', { name: 'Stop nesting' }))
	await waitFor(() => expect(heard).toEqual(['edit sent']))

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
	await act(() => new Promise((resolve) => setTimeout(resolve, 50)))
	edit.release()

	await waitFor(() => expect(heard).toEqual(['edit sent', 'edit answered', 'delete sent']))
})
