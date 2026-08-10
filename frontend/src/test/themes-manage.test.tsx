// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { chosenArchive, rollbackLabel, servingLine } from '../themes/ThemesScreen'
import { renderAt } from './render'

const ARCHIVE = new File(['a packaged theme'], 'aurora.zip', { type: 'application/zip' })

/**
 * Returns a listed theme row.
 * @param name - The theme name.
 * @param active - Whether the public site is served through it.
 * @param extra - The version or the reason it will not load.
 * @returns The row the server would list.
 */
function row(name: string, active: boolean, extra: Record<string, string> = { version: '1.0.0' }) {
	return { name, active, serving: active, ...extra }
}

/**
 * Returns a listed theme that is the stored choice but is not answering the site.
 * @param name - The theme name.
 * @returns The row the server would list.
 */
function chosenButDown(name: string) {
	return { name, version: '1.0.0', active: true, serving: false }
}

/**
 * Serves a themes list with the given rows and rollback target.
 * @param themes - The rows to list.
 * @param rollback - The rollback target, or undefined when none is offered.
 */
function listing(themes: unknown[], rollback?: string) {
	server.use(
		http.get('/api/themes', () =>
			HttpResponse.json(rollback === undefined ? { themes } : { themes, rollback: { theme: rollback } }),
		),
	)
}

beforeEach(() =>
	listing([row('aurora', true), row('riverbed', false, { version: '0.9.0' })]),
)

/**
 * Returns a theme as the screen models it.
 * @param name - The theme name.
 * @param active - Whether it is the chosen theme.
 * @param broken - The reason it will not load, empty when it loads.
 * @param version - The version its manifest declares.
 * @returns The theme.
 */
function theme(name: string, active: boolean, broken = '', version = '1.0.0', serving = active) {
	return { name, version, broken, active, serving }
}

test('names the theme serving the public site', async () => {
	renderAt('/themes')

	expect(await screen.findByText('aurora 1.0.0 is serving the public site.')).toBeTruthy()
})

test('says the renderer took over when the chosen theme stopped answering', async () => {
	listing([chosenButDown('aurora')])
	renderAt('/themes')

	expect(
		await screen.findByText('aurora is not answering, so the built-in renderer is serving.'),
	).toBeTruthy()
	const listed = await screen.findByRole('row', { name: /aurora/ })
	expect(within(listed).getByText('Not serving')).toBeTruthy()
})

test('says the built-in renderer is serving when no theme is active', async () => {
	listing([row('riverbed', false)])
	renderAt('/themes')

	expect(await screen.findByText('The built-in renderer is serving the public site.')).toBeTruthy()
})

test('says the renderer is serving when the active theme will not load', async () => {
	listing([theme('aurora', true, 'the manifest is missing', '1.0.0', false)])
	renderAt('/themes')

	expect(
		await screen.findByText('aurora will not load, so the built-in renderer is serving.'),
	).toBeTruthy()
})

test('still offers to deactivate a chosen theme whose files are gone', async () => {
	listing([theme('aurora', true, 'the theme is not installed', '', false)])
	renderAt('/themes')

	const listed = await screen.findByRole('row', { name: /aurora/ })

	expect(within(listed).getByText('the theme is not installed')).toBeTruthy()
	expect(screen.getByRole('button', { name: 'Deactivate aurora' })).toBeTruthy()
})

test('claims nothing about what is serving until the themes have been read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/themes', () => new HttpResponse(null, { status: 500 })))
	renderAt('/themes')

	await screen.findByRole('alert')

	expect(screen.queryByText(/is serving the public site/)).toBeNull()
	expect(screen.queryByText(/is the active theme/)).toBeNull()
})

test('phrases what is serving from the themes alone', () => {
	expect(servingLine(undefined)).toBeUndefined()
	expect(servingLine({ themes: [], rollback: null })).toBe(
		'The built-in renderer is serving the public site.',
	)
	expect(servingLine({ themes: [theme('aurora', true)], rollback: null })).toBe(
		'aurora 1.0.0 is serving the public site.',
	)
	expect(servingLine({ themes: [theme('aurora', true, '', '')], rollback: null })).toBe(
		'aurora is serving the public site.',
	)
	expect(
		servingLine({
			themes: [theme('aurora', true, 'the manifest is missing')],
			rollback: null,
		}),
	).toBe('aurora will not load, so the built-in renderer is serving.')
	expect(
		servingLine({ themes: [theme('aurora', true, '', '1.0.0', false)], rollback: null }),
	).toBe('aurora is not answering, so the built-in renderer is serving.')
})

test('reads the archive out of a file field that may hold none', () => {
	expect(chosenArchive(null)).toBeNull()
	expect(chosenArchive([] as unknown as FileList)).toBeNull()
	expect(chosenArchive([ARCHIVE] as unknown as FileList)).toBe(ARCHIVE)
})

test('phrases a rollback by where it returns to', () => {
	expect(rollbackLabel('')).toBe('Roll back to the built-in renderer')
	expect(rollbackLabel('riverbed')).toBe('Roll back to riverbed')
})

test('lists each installed theme with its own version and state', async () => {
	renderAt('/themes')

	const serving = await screen.findByRole('row', { name: /aurora/ })
	const idle = await screen.findByRole('row', { name: /riverbed/ })

	expect(within(serving).getByText('1.0.0')).toBeTruthy()
	expect(within(serving).getByText('Serving')).toBeTruthy()
	expect(within(idle).getByText('0.9.0')).toBeTruthy()
	expect(within(idle).getByText('Installed')).toBeTruthy()
})

test('marks a theme that will not load with the reason', async () => {
	listing([row('driftwood', false, { broken: 'the manifest is missing' })])
	renderAt('/themes')

	const listed = await screen.findByRole('row', { name: /driftwood/ })

	expect(within(listed).getByText('Broken')).toBeTruthy()
	expect(within(listed).getByText('the manifest is missing')).toBeTruthy()
})

test('says so when no theme is installed at all', async () => {
	listing([])
	renderAt('/themes')

	expect(await screen.findByText('No themes are installed.')).toBeTruthy()
})

test('keeps the themes heading when the themes cannot be listed', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/themes', () => new HttpResponse(null, { status: 500 })))
	renderAt('/themes')

	expect(await screen.findByRole('alert')).toHaveTextContent('Themes could not be loaded.')
	expect(screen.getByRole('heading', { level: 1, name: 'Themes' })).toBeTruthy()
})

test('serves the public site through a theme at the reader s request', async () => {
	let asked: unknown = null
	server.use(
		http.post('/api/themes/active', async ({ request }) => {
			asked = await request.json()
			return HttpResponse.json({ active: 'riverbed' })
		}),
	)
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Activate riverbed' }))

	await waitFor(() => expect(asked).toEqual({ name: 'riverbed' }))
	expect(await screen.findByText('riverbed is now serving the public site.')).toBeTruthy()
})

test('shows the reason an activation was refused', async () => {
	server.use(
		http.post('/api/themes/active', () =>
			HttpResponse.json({ error: 'the theme did not start' }, { status: 422 }),
		),
	)
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Activate riverbed' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('the theme did not start')
})

test('returns the public site to the built-in renderer at the reader s request', async () => {
	let asked = 0
	server.use(
		http.delete('/api/themes/active', () => {
			asked += 1
			return HttpResponse.json({ active: '' })
		}),
	)
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Deactivate aurora' }))

	await waitFor(() => expect(asked).toBe(1))
	expect(await screen.findByText('The built-in renderer is now serving the public site.')).toBeTruthy()
})

test('offers no rollback when nothing has ever been activated', async () => {
	renderAt('/themes')

	await screen.findByRole('row', { name: /aurora/ })

	expect(screen.queryByRole('button', { name: /Roll back/ })).toBeNull()
})

test('re-reads the themes once an action has changed which one is active', async () => {
	let reads = 0
	server.use(
		http.get('/api/themes', () => {
			reads += 1
			return HttpResponse.json({
				themes:
					reads === 1
						? [row('aurora', true), row('riverbed', false, { version: '0.9.0' })]
						: [row('aurora', false), row('riverbed', true, { version: '0.9.0' })],
			})
		}),
		http.post('/api/themes/active', () => HttpResponse.json({ active: 'riverbed' })),
	)
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Activate riverbed' }))

	expect(await screen.findByText('riverbed 0.9.0 is serving the public site.')).toBeTruthy()
	const listed = await screen.findByRole('row', { name: /riverbed/ })
	expect(within(listed).getByText('Serving')).toBeTruthy()
})

test('says so when an action never reached the server', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/themes/active', () => HttpResponse.error()))
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Activate riverbed' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(
		'The server could not be reached, so nothing was changed.',
	)
})

test('rolls the public site back to the previous choice', async () => {
	listing([row('aurora', true), row('riverbed', false)], 'riverbed')
	server.use(http.post('/api/themes/rollback', () => HttpResponse.json({ active: 'riverbed' })))
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Roll back to riverbed' }))

	expect(await screen.findByText('riverbed is now serving the public site.')).toBeTruthy()
})

test('offers a rollback to the built-in renderer by name', async () => {
	listing([row('aurora', true)], '')
	renderAt('/themes')

	expect(await screen.findByRole('button', { name: 'Roll back to the built-in renderer' })).toBeTruthy()
})

test('shows the reason a rollback was refused', async () => {
	listing([row('aurora', true)], 'riverbed')
	server.use(
		http.post('/api/themes/rollback', () =>
			HttpResponse.json({ error: 'the theme is pinned by the operator' }, { status: 422 }),
		),
	)
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Roll back to riverbed' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('the theme is pinned by the operator')
})

test('installs a theme the reader chose from their machine', async () => {
	let uploaded = 0
	server.use(
		http.post('/api/themes', () => {
			uploaded += 1
			return HttpResponse.json({ name: 'aurora' }, { status: 201 })
		}),
	)
	renderAt('/themes')

	await userEvent.upload(await screen.findByLabelText('Theme archive'), ARCHIVE)
	await userEvent.click(screen.getByRole('button', { name: 'Install theme' }))

	await waitFor(() => expect(uploaded).toBe(1))
	expect(await screen.findByText('aurora was installed.')).toBeTruthy()
})

test('waits for an archive before it offers to install one', async () => {
	renderAt('/themes')

	await screen.findByLabelText('Theme archive')

	expect(screen.queryByRole('button', { name: 'Install theme' })).toBeNull()
})

test('shows the reason an upload was refused', async () => {
	server.use(
		http.post('/api/themes', () =>
			HttpResponse.json({ error: 'the manifest is missing' }, { status: 422 }),
		),
	)
	renderAt('/themes')

	await userEvent.upload(await screen.findByLabelText('Theme archive'), ARCHIVE)
	await userEvent.click(screen.getByRole('button', { name: 'Install theme' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('the manifest is missing')
})

test('takes a shown refusal back once the next action works', async () => {
	let attempts = 0
	server.use(
		http.post('/api/themes', () => {
			attempts += 1
			return attempts === 1
				? HttpResponse.json({ error: 'the manifest is missing' }, { status: 422 })
				: HttpResponse.json({ name: 'aurora' }, { status: 201 })
		}),
	)
	renderAt('/themes')
	await userEvent.upload(await screen.findByLabelText('Theme archive'), ARCHIVE)
	await userEvent.click(screen.getByRole('button', { name: 'Install theme' }))
	await screen.findByRole('alert')

	await userEvent.click(screen.getByRole('button', { name: 'Install theme' }))

	await screen.findByText('aurora was installed.')
	expect(screen.queryByText('the manifest is missing')).toBeNull()
})
