// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { listPlugins } from '../pluginsscreen/api'

/** The feed as the server lists it while it runs. */
const FEED = {
	id: 'feed',
	name: 'Feed',
	description: 'Serves your published posts as an RSS feed.',
	version: '0.0.1',
	status: 'active',
	paused_step: '',
	paused_reason: '',
}

/** A plugin the server lists as paused at the step a settings rejection stops it at. */
const NOTES = {
	id: 'notes',
	name: 'Notes',
	description: '',
	version: '',
	status: 'paused',
	paused_step: 'register',
	paused_reason: 'GOPHENBERG_PLUGIN_NOTES__LIMIT: must be a whole number, got "banana"',
}

test('lists every installed plugin with its state and why it paused, in the order the server gives', async () => {
	server.use(http.get('/api/plugin-admin', () => HttpResponse.json({ plugins: [FEED, NOTES] })))

	expect(await listPlugins()).toEqual([FEED, NOTES])
})

test('lists a site with no plugin as an empty list', async () => {
	server.use(http.get('/api/plugin-admin', () => HttpResponse.json({ plugins: [] })))

	expect(await listPlugins()).toEqual([])
})

test('keeps a step it does not know as written, so a newer site never breaks the list', async () => {
	server.use(
		http.get('/api/plugin-admin', () => HttpResponse.json({ plugins: [{ ...NOTES, paused_step: 'crash' }] })),
	)

	expect((await listPlugins())[0].paused_step).toBe('crash')
})

test('leaves out a key it does not know, so a later field never breaks the list', async () => {
	server.use(
		http.get('/api/plugin-admin', () => HttpResponse.json({ plugins: [{ ...FEED, has_settings: true }] })),
	)

	expect(await listPlugins()).toEqual([FEED])
})

test('lists a plugin its record holds as inactive', async () => {
	server.use(
		http.get('/api/plugin-admin', () => HttpResponse.json({ plugins: [{ ...FEED, status: 'inactive' }] })),
	)

	expect((await listPlugins())[0].status).toBe('inactive')
})

test('rejects a list that names a state no plugin stands in', async () => {
	server.use(
		http.get('/api/plugin-admin', () => HttpResponse.json({ plugins: [{ ...FEED, status: 'not installed' }] })),
	)

	await expect(listPlugins()).rejects.toThrow()
})

test.each([
	[403, 'capability_missing'],
	[503, 'database_busy'],
	[500, 'internal'],
])('rejects with the status when the server answers %i %s', async (status, code) => {
	server.use(http.get('/api/plugin-admin', () => HttpResponse.json({ error: 'not listed', code }, { status })))

	await expect(listPlugins()).rejects.toThrow(String(status))
})

test('stops the read when the signal it was handed ends', async () => {
	server.use(http.get('/api/plugin-admin', () => new Promise(() => {})))
	const stop = new AbortController()

	const reading = listPlugins(stop.signal)
	stop.abort()

	await expect(reading).rejects.toThrow()
})
