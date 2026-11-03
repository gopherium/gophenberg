// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { badgeClasses, buttonClasses } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { rollbackLabel, servingLine } from '../themes/ThemesScreen'
import { gate } from './gate'
import { renderAt } from './render'
import { findToast } from './typeFixtures'

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
 * @param state - What differs from a theme that loads, declares 1.0.0 and answers the site while chosen.
 * @returns The theme.
 */
function theme(
	name: string,
	active: boolean,
	state: { broken?: string, version?: string, serving?: boolean, startFailed?: boolean } = {},
) {
	return { name, version: '1.0.0', broken: '', active, serving: active, startFailed: false, ...state }
}

test('names the theme serving the public site', async () => {
	renderAt('/themes')

	expect(await screen.findByText('aurora 1.0.0 is serving the public site.')).toBeTruthy()
})

test('says the chosen theme is still starting while it has not answered yet', async () => {
	listing([chosenButDown('aurora')])
	renderAt('/themes')

	expect(
		await screen.findByText('aurora is still starting, so the built-in renderer is serving.'),
	).toBeTruthy()
	const listed = await screen.findByRole('row', { name: /aurora/ })
	expect(within(listed).getByText('Not serving')).toBeTruthy()
})

test('says the chosen theme failed to start rather than calling it still starting', async () => {
	listing([{ ...chosenButDown('aurora'), startFailed: true }])
	renderAt('/themes')

	expect(
		await screen.findByText('aurora failed to start, so the built-in renderer is serving.'),
	).toBeTruthy()
})

test('says the built-in renderer is serving when no theme is active', async () => {
	listing([row('riverbed', false)])
	renderAt('/themes')

	expect(await screen.findByText('The built-in renderer is serving the public site.')).toBeTruthy()
})

test('says the renderer is serving when the active theme will not load', async () => {
	listing([theme('aurora', true, { broken: 'the manifest is missing', serving: false })])
	renderAt('/themes')

	expect(
		await screen.findByText('aurora will not load, so the built-in renderer is serving.'),
	).toBeTruthy()
})

test('still offers to deactivate a chosen theme whose files are gone', async () => {
	listing([theme('aurora', true, { broken: 'the theme is not installed', version: '', serving: false })])
	renderAt('/themes')

	const listed = await screen.findByRole('row', { name: /aurora/ })

	expect(within(listed).getByText('the theme is not installed')).toBeTruthy()
	expect(screen.getByRole('button', { name: 'Deactivate aurora' })).toBeTruthy()
})

test('keeps the Themes title and shows a spinner under it while the themes are on their way', async () => {
	server.use(http.get('/api/themes', () => new Promise(() => {})))
	renderAt('/themes')

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('heading', { level: 2 })).toHaveTextContent('Themes')
	const status = await within(main).findByRole('status')
	expect(status).toHaveTextContent('Loading themes.')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
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
	expect(servingLine({ themes: [theme('aurora', true, { version: '' })], rollback: null })).toBe(
		'aurora is serving the public site.',
	)
	expect(
		servingLine({
			themes: [theme('aurora', true, { broken: 'the manifest is missing' })],
			rollback: null,
		}),
	).toBe('aurora will not load, so the built-in renderer is serving.')
	expect(
		servingLine({ themes: [theme('aurora', true, { serving: false })], rollback: null }),
	).toBe('aurora is still starting, so the built-in renderer is serving.')
	expect(
		servingLine({ themes: [theme('aurora', true, { serving: false, startFailed: true })], rollback: null }),
	).toBe('aurora failed to start, so the built-in renderer is serving.')
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

test('draws an installed theme as an outline badge, the WordPress badge for an inactive state', async () => {
	renderAt('/themes')

	const idle = await screen.findByRole('row', { name: /riverbed/ })

	expect([...within(idle).getByText('Installed').classList]).toEqual(badgeClasses('none'))
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
	expect(screen.getByRole('heading', { level: 2, name: 'Themes' })).toBeTruthy()
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

	expect(await findToast('Could not activate the theme. the theme did not start')).toBeTruthy()
	expect(screen.queryByRole('alert')).toBeNull()
})

test('shows the reason a deactivation was refused', async () => {
	server.use(
		http.delete('/api/themes/active', () =>
			HttpResponse.json({ error: 'the theme is pinned by the operator' }, { status: 422 }),
		),
	)
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: 'Deactivate aurora' }))

	expect(await findToast('Could not deactivate the theme. the theme is pinned by the operator')).toBeTruthy()
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

test.each([
	{
		press: 'Activate riverbed',
		lead: 'Could not activate the theme.',
		unreachable: http.post('/api/themes/active', () => HttpResponse.error()),
	},
	{
		press: 'Deactivate aurora',
		lead: 'Could not deactivate the theme.',
		unreachable: http.delete('/api/themes/active', () => HttpResponse.error()),
	},
	{
		press: 'Roll back to riverbed',
		lead: 'Could not roll back the theme.',
		unreachable: http.post('/api/themes/rollback', () => HttpResponse.error()),
	},
])('says so when $press never reached the server', async ({ press, lead, unreachable }) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	listing([row('aurora', true), row('riverbed', false)], 'riverbed')
	server.use(unreachable)
	renderAt('/themes')

	await userEvent.click(await screen.findByRole('button', { name: press }))

	expect(await findToast(`${lead} The server could not be reached, so nothing was changed.`)).toBeTruthy()
})

test('says so when an upload never reached the server', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/themes', () => HttpResponse.error()))
	renderAt('/themes')
	await screen.findByRole('button', { name: 'Upload theme' })

	await userEvent.upload(screen.getByLabelText('Upload theme'), ARCHIVE)

	expect(
		await findToast('Could not install the theme. The server could not be reached, so nothing was changed.'),
	).toBeTruthy()
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

	expect(await findToast('Could not roll back the theme. the theme is pinned by the operator')).toBeTruthy()
})

test('offers Upload theme as the compact solid header button, drawn with the upload icon', async () => {
	renderAt('/themes')

	const button = await within(await screen.findByRole('main')).findByRole('button', { name: 'Upload theme' })

	expect([...button.classList]).toEqual(buttonClasses('solid', 'compact'))
	expect(button.querySelector('svg')).not.toBeNull()
})

test('keeps the file field out of the page body, behind Upload theme', async () => {
	renderAt('/themes')

	await screen.findByRole('button', { name: 'Upload theme' })
	const input = screen.getByLabelText('Upload theme')

	expect(input).toHaveAttribute('accept', '.zip')
	expect(input).not.toHaveAttribute('multiple')
	expect(input).not.toBeVisible()
	expect(screen.queryByText('Theme archive')).toBeNull()
	expect(screen.queryByRole('button', { name: 'Install theme' })).toBeNull()
})

test('installs a theme as soon as the reader picks its archive, and says so in a toast', async () => {
	let uploaded = 0
	server.use(
		http.post('/api/themes', () => {
			uploaded += 1
			return HttpResponse.json({ name: 'aurora' }, { status: 201 })
		}),
	)
	renderAt('/themes')
	await screen.findByRole('button', { name: 'Upload theme' })

	await userEvent.upload(screen.getByLabelText('Upload theme'), ARCHIVE)

	await waitFor(() => expect(uploaded).toBe(1))
	expect(await screen.findByText('aurora was installed.', { selector: '.godmin-toast' })).toBeTruthy()
})

test('holds Upload theme busy while the archive uploads', async () => {
	const held = gate()
	server.use(
		http.post('/api/themes', async () => {
			await held.held
			return HttpResponse.json({ name: 'aurora' }, { status: 201 })
		}),
	)
	renderAt('/themes')
	const button = await screen.findByRole('button', { name: 'Upload theme' })

	await userEvent.upload(screen.getByLabelText('Upload theme'), ARCHIVE)

	await waitFor(() => expect(button).toHaveAttribute('aria-disabled', 'true'))
	held.release()
	await waitFor(() => expect(button).not.toHaveAttribute('aria-disabled', 'true'))
})

test('shows the reason an upload was refused in a toast', async () => {
	server.use(
		http.post('/api/themes', () =>
			HttpResponse.json({ error: 'the manifest is missing' }, { status: 422 }),
		),
	)
	renderAt('/themes')
	await screen.findByRole('button', { name: 'Upload theme' })

	await userEvent.upload(screen.getByLabelText('Upload theme'), ARCHIVE)

	expect(await findToast('Could not install the theme. the manifest is missing')).toBeTruthy()
	expect(screen.queryByRole('alert')).toBeNull()
})

test('draws a theme status badge at the width of its words, not stretched across its column', async () => {
	renderAt('/themes')

	const serving = await screen.findByRole('row', { name: /aurora/ })

	expect(within(serving).getByText('Serving').parentElement).toHaveStyle({ alignItems: 'flex-start' })
})
