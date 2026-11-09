// SPDX-License-Identifier: Apache-2.0

import { CANVAS_LAYOUT } from '@gophenberg/frontend-sdk/editor'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, expect, onTestFinished, test, vi } from 'vitest'

import { adminBasepath } from '../basepath'
import { adminUser, renderAt } from './render'
import { storedPost } from './postFixture'
import { siteSettings } from './siteSettings'

const FOREIGN = '019fb000-0000-7000-8000-0000000000ff'

const maria = { ...adminUser, role: 'author', capabilities: [] }

const grace = { ...adminUser, role: 'editor', capabilities: ['change_others_work'] }

beforeAll(async () => {
	await import('../content/EditorScreen')
	await import('../media/MediaScreen')
}, 120000)

/**
 * Returns a listed post authored by the given account.
 * @param authorId - The account the post belongs to.
 * @returns The post row.
 */
function postBy(authorId: string) {
	return {
		id: '019fb000-0000-7000-8000-000000000001',
		type: 'post',
		slug: 'welcome',
		title: 'Welcome to Gophenberg',
		excerpt: '',
		status: 'published',
		author_id: authorId,
		author_name: 'Maria Perez',
		published_at: '2026-07-20T10:00:00Z',
		created_at: '2026-07-19T10:00:00Z',
		updated_at: '2026-07-20T10:00:00Z',
	}
}

/**
 * Serves a one-post listing authored by the given account.
 * @param authorId - The account the listed post belongs to.
 */
function servePosts(authorId: string) {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [postBy(authorId)], total: 1 }),
		),
	)
}

/**
 * Serves a one-item media library authored by the given account.
 * @param authorId - The account the item belongs to.
 */
function serveMedia(authorId: string) {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({
				items: [
					{
						id: 7,
						type: 'image',
						file: '2026/08/harbor.jpg',
						title: 'Harbor at dawn',
						alt_text: 'Boats at sunrise',
						caption: '',
						description: '',
						mime_type: 'image/jpeg',
						width: 1600,
						height: 1000,
						filesize: 254000,
						sizes: {},
						author_id: authorId,
						created_at: '2026-08-10T10:00:00Z',
						updated_at: '2026-08-10T10:00:00Z',
					},
				],
				total: 1,
				per_page: siteSettings.list_page_size,
			}),
		),
	)
}

/**
 * Serves the stored post and its empty autosave for the editor.
 * @param authorId - The account the stored post belongs to.
 * @param fields - The stored post's fields that differ from the fixture.
 */
function serveEditor(authorId: string, fields: Record<string, unknown> = {}) {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, ...fields, author_id: authorId }),
		),
		http.get(`/api/content/${storedPost.id}/autosave`, () =>
			HttpResponse.json({}, { status: 404 }),
		),
	)
}

test('offers an author only a duplicate and a view of a published post another account wrote', async () => {
	servePosts(FOREIGN)
	renderAt('/content/post', maria)
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	await screen.findByRole('menuitem', { name: 'Duplicate…' })
	expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['View', 'Duplicate…'])
})

test('offers an author every action on its own post', async () => {
	servePosts(maria.id)
	renderAt('/content/post', maria)
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	expect(await screen.findByRole('menuitem', { name: 'Trash' })).toBeInTheDocument()
	expect(screen.queryByRole('menuitem', { name: 'Edit' })).not.toBeInTheDocument()
})

test('offers an editor every action on a post another account wrote', async () => {
	servePosts(FOREIGN)
	renderAt('/content/post', grace)
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	expect(await screen.findByRole('menuitem', { name: 'Trash' })).toBeInTheDocument()
})

test('opens a post another account wrote as a reading view', async () => {
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	expect(await screen.findByText(/another account wrote this/i)).toBeInTheDocument()
	expect(screen.getByText('Welcome to Gophenberg')).toBeInTheDocument()
	expect(screen.queryByRole('textbox', { name: 'Title' })).toBeNull()
	expect(screen.queryByRole('button', { name: /publish|update/i })).toBeNull()
})

test('leads a reading view back to the list of its type, never as the current page', async () => {
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	const breadcrumbs = await screen.findByRole('navigation', { name: 'Breadcrumbs' })
	const back = await within(breadcrumbs).findByRole('link', { name: 'Posts' })

	expect(back).toHaveAttribute('href', `${adminBasepath}/content/post`)
	expect(back).not.toHaveAttribute('aria-current')
})

test('leads a reading view back to the list its post is stored under, whatever type its address names', async () => {
	serveEditor(FOREIGN)
	renderAt(`/content/page/${storedPost.id}/edit`, maria)

	const breadcrumbs = await screen.findByRole('navigation', { name: 'Breadcrumbs' })
	const back = await within(breadcrumbs).findByRole('link', { name: 'Posts' })

	expect(back).toHaveAttribute('href', `${adminBasepath}/content/post`)
})

test('leads a reading view back to its list under Content until the registry answers', async () => {
	server.use(http.get('/api/types', () => new Promise<Response>(() => {})))
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	const breadcrumbs = await screen.findByRole('navigation', { name: 'Breadcrumbs' })

	expect(within(breadcrumbs).getByRole('link', { name: 'Content' })).toHaveAttribute(
		'href',
		`${adminBasepath}/content/post`,
	)
})

test('reads an untitled foreign post under a stand in title, its excerpt beside it', async () => {
	serveEditor(FOREIGN, { title: '', excerpt: 'A few words about it.' })
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	expect(await screen.findByText('(no title)')).toBeInTheDocument()
	expect(screen.getByText('A few words about it.')).toBeInTheDocument()
})

/**
 * Replaces ResizeObserver with one that reports every element it watches at the given width, until the test ends.
 * @param width - The width in pixels each watched element reports.
 */
function measureAt(width: number) {
	vi.stubGlobal(
		'ResizeObserver',
		class {
			reported: ResizeObserverCallback
			constructor(reported: ResizeObserverCallback) {
				this.reported = reported
			}
			observe(target: Element) {
				const entry = { target, contentBoxSize: [{ inlineSize: width, blockSize: 200 }] }
				this.reported([entry as unknown as ResizeObserverEntry], this as unknown as ResizeObserver)
			}
			unobserve() {}
			disconnect() {}
		},
	)
	onTestFinished(() => {
		vi.unstubAllGlobals()
	})
}

/**
 * Returns the frame the reading view draws the post in, once it has a width to draw at.
 * @returns The preview frame.
 */
async function previewFrame(): Promise<HTMLIFrameElement> {
	await waitFor(() => expect(document.querySelector('.block-editor-block-preview__content iframe')).not.toBeNull())
	return document.querySelector('.block-editor-block-preview__content iframe') as HTMLIFrameElement
}

test('draws the post of a reading view at the width it is given, never scaled', async () => {
	measureAt(390)
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	const frame = await previewFrame()

	expect(frame.style.width).toBe('390px')
	expect((frame.closest('.block-editor-block-preview__content') as HTMLElement).style.transform).toBe('scale(1)')
})

test('draws the post of a reading view no wider than the column the editor gives it', async () => {
	measureAt(390)
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	const frame = await previewFrame()

	const column = frame.closest('.block-editor-block-preview__container')?.parentElement
	expect(column?.style.maxWidth).toBe(CANVAS_LAYOUT.contentSize)
})

test('reads a trashed post another account wrote as a foreign post', async () => {
	serveEditor(FOREIGN, { status: 'trash' })
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	expect(await screen.findByText(/another account wrote this/i)).toBeInTheDocument()
	expect(screen.queryByText(/This item is in the trash/)).toBeNull()
	expect(screen.queryByRole('button', { name: 'Restore' })).toBeNull()
})

test('offers an editor a restore on a trashed post another account wrote', async () => {
	serveEditor(FOREIGN, { status: 'trash' })
	renderAt(`/content/post/${storedPost.id}/edit`, grace)

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Restore' })).toBeInTheDocument()
})

test('opens an author s own post as the editor', async () => {
	serveEditor(maria.id)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
})

test('opens a foreign post as the editor for an editor', async () => {
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, grace)

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
})

/** Every action the media library offers on a file the session may change, in order. */
const EVERY_MEDIA_ACTION = ['View', 'Edit…', 'Rename…', 'Permanently delete…']

/**
 * Opens the actions menu of the one listed file and returns the words of its items.
 * @returns The words, in order.
 */
async function mediaMenu(): Promise<(string | null)[]> {
	await screen.findByText('Harbor at dawn')
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))
	await screen.findByRole('menuitem', { name: 'View' })
	return screen.getAllByRole('menuitem').map((item) => item.textContent)
}

test('offers an author only a view of media another account uploaded', async () => {
	serveMedia(FOREIGN)
	renderAt('/media', maria)

	expect(await mediaMenu()).toEqual(['View'])
})

test('offers an editor every action on media another account uploaded', async () => {
	serveMedia(FOREIGN)
	renderAt('/media', grace)

	expect(await mediaMenu()).toEqual(EVERY_MEDIA_ACTION)
})

test('offers an author every action on its own upload', async () => {
	serveMedia(maria.id)
	renderAt('/media', maria)

	expect(await mediaMenu()).toEqual(EVERY_MEDIA_ACTION)
})
