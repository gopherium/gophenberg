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
	clickDimmedPage,
	findToast,
	openRowDialog,
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
	return openRowDialog('Pages', 'Delete…')
}

test('asks before deleting a type under no header, naming it in quotes, and deletes nothing yet', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()

	expect(within(dialog).getByText('Delete "Pages" for good? This cannot be undone.')).toBeInTheDocument()
	expect(within(dialog).queryByRole('heading')).not.toBeInTheDocument()
	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	expect(asked).toEqual([])
})

test('asks at the width of a WordPress row confirm, Delete in the accent after an accent text Cancel', async () => {
	renderAt('/content-types')

	const dialog = await openDelete()

	expect(dialog).toHaveClass('has-size-medium')
	const [cancel, confirm] = within(dialog).getAllByRole('button')
	expect([cancel.textContent, confirm.textContent]).toEqual(['Cancel', 'Delete'])
	expect([...cancel.classList]).toEqual(buttonClasses('minimal'))
	expect([...confirm.classList]).toEqual(buttonClasses('solid'))
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

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(asked).toEqual([])
})

test('deletes a type once confirmed and names it in a toast', async () => {
	const asked = recordDeletes()
	renderAt('/content-types')

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

	await waitFor(() => expect(asked).toEqual(['/api/types/page']))
	expect(await screen.findByText('"Pages" permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
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
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
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

	const dialog = await openRowDialog(reader.pages, reader.remove, reader)
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
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
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

test('leaves a dialog opened on the row after a delete dismissed by the dimmed page open once it ends', async () => {
	const answer = gate()
	recordDeletes(async () => {
		await answer.held
		return HttpResponse.json({ error: 'content: down' }, { status: 500 })
	})
	renderAt('/content-types')
	const confirm = await openDelete()
	await userEvent.click(within(confirm).getByRole('button', { name: 'Delete' }))
	await waitBusy(confirm, 'Delete')
	await clickDimmedPage()
	const describe = await openRowDialog('Pages', 'Describe…')

	answer.release()
	await screen.findByText('content: down', { selector: '.godmin-toast' })
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Describe' })).toBe(describe)
})

/**
 * Starts the delete of the Pages row, held until the returned gate opens, and closes its busy confirm on the overlay.
 * @returns The gate the delete waits on.
 */
async function deletePagesThenLeave() {
	const answer = gate()
	recordDeletes(async () => {
		await answer.held
		return new HttpResponse(null, { status: 204 })
	})
	renderAt('/content-types')
	const confirm = await openDelete()
	await userEvent.click(within(confirm).getByRole('button', { name: 'Delete' }))
	await waitBusy(confirm, 'Delete')
	await clickDimmedPage()
	return answer
}

test('keeps the focus in a Describe opened after a dismissed delete once that delete lands', async () => {
	stubLayout()
	const answer = await deletePagesThenLeave()
	const describe = await openRowDialog('Posts', 'Describe…')
	const field = within(describe).getByLabelText('Description')
	await waitFor(() => expect(field).toHaveFocus())

	answer.release()
	await findToast('"Pages" permanently deleted.')
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Describe' })).toBe(describe)
	expect(field).toHaveFocus()
})

test('lands the focus on the list when a dismissed delete ends while nothing holds the focus', async () => {
	const answer = await deletePagesThenLeave()
	act(() => (document.activeElement as HTMLElement).blur())

	answer.release()
	await findToast('"Pages" permanently deleted.')

	await waitFor(() => expect(screen.getByRole('region', { name: 'Content Types' })).toHaveFocus())
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
	const describe = await openRowDialog('Pages', 'Describe…')
	await userEvent.click(within(describe).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(heard).toEqual(['edit sent']))

	const dialog = await openDelete()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
	await act(() => new Promise((resolve) => setTimeout(resolve, 50)))
	edit.release()

	await waitFor(() => expect(heard).toEqual(['edit sent', 'edit answered', 'delete sent']))
})
