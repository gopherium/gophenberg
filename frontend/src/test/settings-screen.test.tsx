// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { defaultUser } from '@gopherium/react-auth/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { listenAt } from './announcements'
import { gate } from './gate'
import { adminUser, renderAt, renderRoutedAt } from './render'
import { siteSettings } from './siteSettings'
import { findToast } from './typeFixtures'

const PATH = '/settings'

/** The words under a box that holds no whole number. */
const NOT_WHOLE = 'Type a whole number.'

/** The words the server's answer about a page size of 500 reads as. */
const TOO_MANY =
	'A listing carries between 1 and 100 items, so 500 is not a size Gophenberg takes. Pick a number inside that range.'

/**
 * Answers every save with the server's words about a page size of 500.
 * @param held - The promise the answer waits on.
 */
function refuseSize(held: Promise<void> = Promise.resolve()) {
	server.use(
		http.patch('/api/settings', async () => {
			await held
			return HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			)
		}),
	)
}

beforeAll(async () => {
	await import('../settings/SettingsScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, locale_default: '', content_per_page: 20, jpeg_quality: 82 }),
		),
	)
})

test('keeps the Settings title and shows a spinner under it while the settings are on their way', async () => {
	let reads = 0
	server.use(
		http.get('/api/settings', () => {
			reads += 1
			return reads === 1 ? HttpResponse.json(siteSettings) : new Promise<never>(() => {})
		}),
	)
	renderAt(PATH, adminUser)

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('heading', { level: 2 })).toHaveTextContent('Settings')
	const status = await within(main).findByRole('status')
	expect(status).toHaveTextContent('Loading the settings.')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
})

test('shows what the site chose for itself', async () => {
	renderAt(PATH, adminUser)

	expect(await screen.findByLabelText('Posts per page')).toHaveValue(20)
	expect(await screen.findByLabelText('Picture quality')).toHaveValue(82)
})

test('stores a page size an administrator typed', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 5, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	expect(field).toHaveValue(null)
	await userEvent.type(field, '5')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toContain('"content_per_page":5'))
})

test('stores a picture quality an administrator typed', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 30 })
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Picture quality')
	await userEvent.clear(field)
	await userEvent.type(field, '30')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toContain('"jpeg_quality":30'))
})

test('stores nothing while a box stands empty', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	await userEvent.clear(await screen.findByLabelText('Posts per page'))
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(screen.getByLabelText('Posts per page')).toHaveAccessibleDescription(NOT_WHOLE))
	expect(sent).toHaveLength(0)
})

test('stores nothing while a quality box stands empty', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	await userEvent.clear(await screen.findByLabelText('Picture quality'))
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(screen.getByLabelText('Picture quality')).toHaveAccessibleDescription(NOT_WHOLE))
	expect(sent).toHaveLength(0)
})

test('stores nothing when a page size is not a whole number', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '2.5')
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(field).toHaveAccessibleDescription(NOT_WHOLE))
	expect(sent).toHaveLength(0)
})

test('shows the words under each box that holds no whole number', async () => {
	renderAt(PATH, adminUser)

	const size = await screen.findByLabelText('Posts per page')
	const quality = screen.getByLabelText('Picture quality')
	await userEvent.clear(size)
	await userEvent.clear(quality)
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(size).toHaveAccessibleDescription(NOT_WHOLE))
	expect(quality).toHaveAccessibleDescription(NOT_WHOLE)
})

test('leaves the box that holds a whole number without words', async () => {
	renderAt(PATH, adminUser)

	const quality = await screen.findByLabelText('Picture quality')
	await userEvent.clear(quality)
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(quality).toHaveAccessibleDescription(NOT_WHOLE))
	expect(screen.getByLabelText('Posts per page')).toHaveAccessibleDescription('')
})

test('shows the words under the box Enter was pressed in', async () => {
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(field).toHaveAccessibleDescription(NOT_WHOLE))
})

test('speaks the words under the box Enter was pressed in', async () => {
	const assertive = listenAt('assertive')
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(field).toHaveAccessibleDescription(NOT_WHOLE))
	expect(field).toHaveFocus()
	expect(assertive()).toEqual([NOT_WHOLE])
})

test('leaves the words to the focus when Save moves it to the box', async () => {
	const assertive = listenAt('assertive')
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(field).toHaveAccessibleDescription(NOT_WHOLE))
	expect(field).toHaveFocus()
	expect(assertive()).toEqual([])
})

test('leaves the words to the focus when Enter moves it to the box that holds no whole number', async () => {
	const assertive = listenAt('assertive')
	renderAt(PATH, adminUser)

	const size = await screen.findByLabelText('Posts per page')
	const quality = screen.getByLabelText('Picture quality')
	await userEvent.clear(quality)
	await userEvent.click(size)
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(quality).toHaveFocus())
	expect(assertive()).toEqual([])
})

test('stores what a box holds when Enter is pressed in it', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 5, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '5{Enter}')

	await waitFor(() => expect(sent[0]).toContain('"content_per_page":5'))
})

test('sends a number outside the range on, so the server names the range', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			)
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toContain('"content_per_page":500'))
})

test('offers each box the bounds the server keeps', async () => {
	renderAt(PATH, adminUser)

	const size = await screen.findByLabelText('Posts per page')
	expect(size).toHaveAttribute('min', '1')
	expect(size).toHaveAttribute('max', '100')
	expect(size).toHaveAttribute('step', '1')
	expect(await screen.findByLabelText('Picture quality')).toHaveAttribute('max', '100')
})

test('says why a refused value was not stored', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(field).toHaveAccessibleDescription(expect.stringContaining('100')))
})

test('says under Picture quality why a refused quality was not stored', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json(
				{ error: 'quality invalid', code: 'jpeg_quality_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Picture quality')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(field).toHaveAccessibleDescription(expect.stringContaining('100')))
	expect(screen.getByLabelText('Posts per page')).toHaveAccessibleDescription('')
})

test('says why under the box Enter was pressed in', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500{Enter}')

	await waitFor(() => expect(field).toHaveAccessibleDescription(expect.stringContaining('100')))
})

test('speaks why under the box Enter was pressed in', async () => {
	const assertive = listenAt('assertive')
	refuseSize()
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500{Enter}')

	await waitFor(() => expect(field).toHaveAccessibleDescription(TOO_MANY))
	expect(field).toHaveFocus()
	expect(assertive()).toEqual([TOO_MANY])
})

test('leaves why to the focus when Save moves it back to the box', async () => {
	const assertive = listenAt('assertive')
	refuseSize()
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(field).toHaveAccessibleDescription(TOO_MANY))
	expect(field).toHaveFocus()
	expect(assertive()).toEqual([])
})

test('stops saying why once the box holds another value', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(field).toHaveAccessibleDescription(expect.stringContaining('100')))

	await userEvent.type(field, '{Backspace}')

	expect(field).toHaveValue(50)
	await waitFor(() => expect(field).toHaveAccessibleDescription(''))
})

test('forgets why a value was refused once a save works', async () => {
	refuseSize()
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(field).toHaveAccessibleDescription(TOO_MANY))

	let stored = false
	server.use(
		http.patch('/api/settings', () => {
			stored = true
			return HttpResponse.json({ locale_default: '', content_per_page: 50, jpeg_quality: 82 })
		}),
	)
	await userEvent.type(field, '{Backspace}')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(stored).toBe(true))
	await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).not.toHaveAttribute('aria-disabled', 'true'))
	await userEvent.type(field, '0')

	expect(field).toHaveValue(500)
	expect(field).not.toHaveAccessibleDescription(TOO_MANY)
})

test('says in a toast why a value was not stored when the box changed before the answer', async () => {
	const answer = gate()
	refuseSize(answer.held)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))
	await userEvent.type(field, '{Backspace}')
	answer.release()

	expect(await findToast(`Could not save the settings. ${TOO_MANY}`)).toBeInTheDocument()
	expect(field).toHaveAccessibleDescription('')
	expect(field).toHaveValue(50)
})

test('says in a toast why a value was not stored when the reader left before the answer', async () => {
	const answer = gate()
	refuseSize(answer.held)
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
	const { router } = renderRoutedAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
	await screen.findByRole('heading', { name: 'Posts', level: 2 })
	answer.release()

	expect(await findToast(`Could not save the settings. ${TOO_MANY}`)).toBeInTheDocument()
})

test('says in a toast why a save failed when the words are about no box', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json({ error: 'The disk is full.', code: 'disk_full' }, { status: 500 }),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '5')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	expect(await findToast('Could not save the settings. The disk is full.')).toBeInTheDocument()
	expect(field).toHaveAccessibleDescription('')
	expect(field).toHaveValue(5)
})

test('says in a toast that a save failed when the server gave no reason', async () => {
	server.use(http.patch('/api/settings', () => new HttpResponse('not json at all', { status: 500 })))
	renderAt(PATH, adminUser)

	await screen.findByLabelText('Posts per page')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	expect(await findToast('Could not save the settings. Something went wrong. Try again.')).toBeInTheDocument()
})

test('says in a toast that a save failed when the connection dropped', async () => {
	server.use(http.patch('/api/settings', () => HttpResponse.error()))
	renderAt(PATH, adminUser)

	await screen.findByLabelText('Posts per page')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	expect(await findToast('Could not save the settings. Something went wrong. Try again.')).toBeInTheDocument()
})

test('says in a toast that a save failed when the answer could not be read', async () => {
	server.use(http.patch('/api/settings', () => HttpResponse.json({ stored: true })))
	renderAt(PATH, adminUser)

	await screen.findByLabelText('Posts per page')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	expect(await findToast('Could not save the settings. Something went wrong. Try again.')).toBeInTheDocument()
})

test('offers no boxes to save when the settings could not be read', async () => {
	const sent: string[] = []
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ error: 'boom' }, { status: 500 })),
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	expect(await screen.findByRole('alert')).toBeInTheDocument()
	expect(screen.queryByLabelText('Posts per page')).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
	expect(sent).toHaveLength(0)
})

test('keeps an author out of the settings', async () => {
	renderAt(PATH, { ...defaultUser, role: 'author' })

	await waitFor(() => expect(screen.queryByLabelText('Posts per page')).not.toBeInTheDocument())
})
