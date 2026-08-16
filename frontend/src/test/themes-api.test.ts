// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import {
	activateTheme,
	deactivateTheme,
	listThemes,
	rollbackTheme,
	uploadTheme,
} from '../themes/api'

const ARCHIVE = new File(['a packaged theme'], 'aurora.zip', { type: 'application/zip' })

test('lists the installed themes and what a rollback would return to', async () => {
	server.use(
		http.get('/api/themes', () =>
			HttpResponse.json({
				themes: [
					{ name: 'aurora', version: '1.2.0', active: true, serving: true },
					{ name: 'riverbed', version: '0.9.0', active: false, serving: false },
				],
				rollback: { theme: 'riverbed' },
			}),
		),
	)

	const listed = await listThemes()

	expect(listed.themes).toEqual([
		{ name: 'aurora', version: '1.2.0', broken: '', active: true, serving: true },
		{ name: 'riverbed', version: '0.9.0', broken: '', active: false, serving: false },
	])
	expect(listed.rollback).toBe('riverbed')
})

test('tells a rollback to the built-in renderer from no rollback at all', async () => {
	server.use(
		http.get('/api/themes', () => HttpResponse.json({ themes: [], rollback: { theme: '' } })),
	)

	expect((await listThemes()).rollback).toBe('')

	server.use(http.get('/api/themes', () => HttpResponse.json({ themes: [] })))

	expect((await listThemes()).rollback).toBeNull()
})

test('lists a theme that will not load with the reason and no version', async () => {
	server.use(
		http.get('/api/themes', () =>
			HttpResponse.json({
				themes: [{ name: 'driftwood', broken: 'the manifest is missing', active: false, serving: false }],
			}),
		),
	)

	const listed = await listThemes()

	expect(listed.themes[0]).toEqual({
		name: 'driftwood',
		version: '',
		broken: 'the manifest is missing',
		active: false,
		serving: false,
	})
})

test('reports a themes list the server would not give', async () => {
	server.use(http.get('/api/themes', () => new HttpResponse(null, { status: 500 })))

	await expect(listThemes()).rejects.toThrow(/500/)
})

test('uploads an archive as multipart under the field the server reads', async () => {
	let contentType: string | null = null
	let body = ''
	server.use(
		http.post('/api/themes', async ({ request }) => {
			contentType = request.headers.get('content-type')
			body = await request.text()
			return HttpResponse.json({ name: 'aurora' }, { status: 201 })
		}),
	)

	const outcome = await uploadTheme(ARCHIVE)

	expect(outcome).toEqual({ kind: 'done', name: 'aurora' })
	expect(contentType).toMatch(/^multipart\/form-data; boundary=/)
	expect(body).toMatch(/name="theme"; filename=/)
	expect(body).toContain('Content-Type: application/zip')
})

test('reports the reason an upload was refused', async () => {
	server.use(
		http.post('/api/themes', () =>
			HttpResponse.json({ error: 'the manifest is missing' }, { status: 422 }),
		),
	)

	expect(await uploadTheme(ARCHIVE)).toEqual({
		kind: 'refused',
		reason: 'the manifest is missing',
	})
})

test('falls back to the status when a refusal carries no reason', async () => {
	server.use(http.post('/api/themes', () => new HttpResponse('<html>gateway</html>', { status: 502 })))

	expect(await uploadTheme(ARCHIVE)).toEqual({
		kind: 'refused',
		reason: 'Something went wrong. Try again.',
	})
})

test('activates a theme by name', async () => {
	let asked: unknown = null
	server.use(
		http.post('/api/themes/active', async ({ request }) => {
			asked = await request.json()
			return HttpResponse.json({ active: 'aurora' })
		}),
	)

	expect(await activateTheme('aurora')).toEqual({ kind: 'done', name: 'aurora' })
	expect(asked).toEqual({ name: 'aurora' })
})

test('reports the reason an activation was refused', async () => {
	server.use(
		http.post('/api/themes/active', () =>
			HttpResponse.json({ error: 'the theme did not start' }, { status: 422 }),
		),
	)

	expect(await activateTheme('brokenboot')).toEqual({
		kind: 'refused',
		reason: 'the theme did not start',
	})
})

test('returns the site to the built-in renderer', async () => {
	server.use(http.delete('/api/themes/active', () => HttpResponse.json({ active: '' })))

	expect(await deactivateTheme()).toEqual({ kind: 'done', name: '' })
})

test('reports the reason a deactivation was refused', async () => {
	server.use(
		http.delete('/api/themes/active', () =>
			HttpResponse.json({ error: 'the theme is pinned by the operator' }, { status: 422 }),
		),
	)

	expect(await deactivateTheme()).toEqual({
		kind: 'refused',
		reason: 'the theme is pinned by the operator',
	})
})

test('rolls back to the previous choice', async () => {
	server.use(http.post('/api/themes/rollback', () => HttpResponse.json({ active: 'riverbed' })))

	expect(await rollbackTheme()).toEqual({ kind: 'done', name: 'riverbed' })
})

test('reports the reason a rollback was refused', async () => {
	server.use(
		http.post('/api/themes/rollback', () =>
			HttpResponse.json({ error: 'there is nothing to roll back to' }, { status: 422 }),
		),
	)

	expect(await rollbackTheme()).toEqual({
		kind: 'refused',
		reason: 'there is nothing to roll back to',
	})
})
