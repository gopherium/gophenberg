// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses } from '@gopherium/godmin/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { renderAt } from './render'
import { siteSettings } from './siteSettings'

const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor.jpg',
	title: 'Harbor at dawn',
	alt_text: 'Boats at sunrise',
	caption: 'Before the market opens',
	description: 'From the eastern pier',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {
		thumbnail: {
			file: '2026/08/harbor-150x150.jpg',
			width: 150,
			height: 150,
			mime_type: 'image/jpeg',
			filesize: 9000,
		},
	},
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	created_at: '2026-08-10T10:00:00Z',
	updated_at: '2026-08-10T10:00:00Z',
}

const MANUAL = {
	id: 8,
	type: 'file',
	file: '2026/08/manual.pdf',
	title: 'Manual',
	mime_type: 'application/pdf',
	filesize: 12000,
	sizes: {},
	created_at: '2026-08-09T10:00:00Z',
	updated_at: '2026-08-09T10:00:00Z',
}

/**
 * Serves the given media rows from the listing route.
 * @param items - The rows the listing answers.
 */
function listing(items: unknown[] = [HARBOR, MANUAL]) {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({ items, total: items.length, per_page: siteSettings.list_page_size }),
		),
	)
}

beforeEach(() => {
	listing()
})

test('lists every media item the API returned', async () => {
	renderAt('/media')

	expect(await screen.findByText('Harbor at dawn')).toBeInTheDocument()
	expect(screen.getByText('Manual')).toBeInTheDocument()
})

test('asks the page size the settings name', async () => {
	const asked: (string | null)[] = []
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, list_page_size: 100 })),
		http.get('/api/media', ({ request }) => {
			asked.push(new URL(request.url).searchParams.get('per_page'))
			return HttpResponse.json({ items: [HARBOR], total: 1, per_page: 100 })
		}),
	)
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	expect(asked).toEqual(['100'])
})

test('counts its pages by the size the server served', async () => {
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, list_page_size: 50 })),
		http.get('/api/media', () => HttpResponse.json({ items: [HARBOR, MANUAL], total: 25, per_page: 10 })),
	)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	const pages = within(screen.getByRole('combobox', { name: 'Current page' })).getAllByRole('option')

	expect(pages.map((page) => page.textContent)).toEqual(['1', '2', '3'])
})

test('draws a picture from its smallest fitting sub-size', async () => {
	renderAt('/media')

	const picture = await screen.findByAltText('Boats at sunrise')

	expect(picture).toHaveAttribute('src', '/media/2026/08/harbor-150x150.jpg')
	expect(picture).toHaveAttribute('loading', 'lazy')
})

test('names a picture that carries no alt text by its title', async () => {
	listing([{ ...HARBOR, alt_text: '' }])
	renderAt('/media')

	expect(await screen.findByAltText('Harbor at dawn')).toBeInTheDocument()
})

test('stands a file in for a picture it cannot draw', async () => {
	renderAt('/media')

	const stand = await screen.findByText('manual.pdf')

	expect(stand).toBeInTheDocument()
	expect(screen.queryByAltText('Manual')).not.toBeInTheDocument()
})

test('stands in for a picture whose file will not load', async () => {
	listing([HARBOR])
	renderAt('/media')
	const picture = await screen.findByAltText('Boats at sunrise')

	picture.dispatchEvent(new Event('error'))

	expect(await screen.findByText('harbor.jpg')).toBeInTheDocument()
})

test('reports a library that could not be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/media')

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not/i)
})

test('says the library holds nothing yet, and offers the add button, when nothing narrows it', async () => {
	listing([])
	renderAt('/media')

	const empty = (await screen.findByText('No media items yet')).closest('.godmin-empty') as HTMLElement

	expect([...within(empty).getByRole('button', { name: 'Add Media File' }).classList]).toEqual(buttonClasses('solid'))
	expect(empty.querySelector('svg')).not.toBeNull()
})

test('says no results are found when a search narrows the library to nothing', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	listing([])

	await userEvent.type(screen.getByRole('searchbox'), 'lighthouse')

	const empty = (await screen.findByText('No results found')).closest('.godmin-empty') as HTMLElement

	expect(within(empty).queryByRole('button')).not.toBeInTheDocument()
})

test('says under the Media title what the library holds', async () => {
	renderAt('/media')

	expect(await screen.findByText('Manage the images and files this site holds.')).toHaveClass('godmin-page__subtitle')
})

test('draws the library in the list region of the page, named Media', async () => {
	renderAt('/media')

	const region = await screen.findByRole('region', { name: 'Media' })

	expect(await within(region).findByText('Harbor at dawn')).toBeInTheDocument()
	expect(region).toHaveClass('godmin-page__list')
	expect(region.parentElement).toHaveClass('godmin-page')
	expect(region).toHaveAttribute('tabindex', '-1')
})

test('lays a drop zone reading Drop files to upload over the list region', async () => {
	renderAt('/media')

	const region = await screen.findByRole('region', { name: 'Media' })
	const zone = region.querySelector(':scope > .godmin-list-overlay') as HTMLElement

	expect(zone).not.toBeNull()
	expect(zone).toHaveTextContent('Drop files to upload')
})

test('words the drop zone through the admin catalogue', async () => {
	setLocaleData(
		{ '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, 'Drop files to upload': ['Release files to upload'] },
		'gophenberg',
	)
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/media')

	const region = await screen.findByRole('region', { name: 'Media' })

	expect(region.querySelector(':scope > .godmin-list-overlay')).toHaveTextContent('Release files to upload')
})

test('ignores a drop that carries no file, while the drop after it carrying one uploads it', async () => {
	let uploaded = 0
	server.use(
		http.post('/api/media', () => {
			uploaded += 1
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	const zone = document.querySelector('.godmin-list-overlay') as HTMLElement
	const dropped = (files: File[]) =>
		zone.dispatchEvent(
			Object.assign(new Event('drop', { bubbles: true, cancelable: true }), {
				dataTransfer: { files, items: [], types: files.length > 0 ? ['Files'] : [], getData: () => '' },
			}),
		)

	dropped([])
	dropped([new File(['bytes'], 'harbor.jpg', { type: 'image/jpeg' })])

	expect(await screen.findByText('"harbor.jpg" uploaded.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(uploaded).toBe(1)
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

