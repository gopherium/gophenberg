// SPDX-License-Identifier: Apache-2.0

import { beforeAll, beforeEach, expect, test } from 'vitest'

import {
	CURATED_BLOCKS,
	apiFetchAttempts,
	clearApiFetchAttempts,
	getBlockTypes,
	installApiFetchGuard,
	registerCuratedBlocks,
} from '../editor'

beforeAll(async () => {
	registerCuratedBlocks()
	await Promise.resolve()
}, 120000)

beforeEach(() => {
	clearApiFetchAttempts()
})

test('registers every curated block and nothing else', () => {
	const registered = getBlockTypes().map((type) => type.name)

	expect(registered.toSorted()).toEqual([...CURATED_BLOCKS].toSorted())
})

test('curates blocks that need a server, a media library or post meta away', () => {
	const registered = new Set(getBlockTypes().map((type) => type.name))

	for (const excluded of ['core/embed', 'core/cover', 'core/footnotes', 'core/rss', 'core/query']) {
		expect(registered.has(excluded)).toBe(false)
	}
})

test('offers the blocks a daily driver writes with', () => {
	const registered = new Set(getBlockTypes().map((type) => type.name))

	for (const wanted of ['core/paragraph', 'core/heading', 'core/list', 'core/image']) {
		expect(registered.has(wanted)).toBe(true)
	}
})

test('keeps the unregistered content fallback so foreign markup survives a load', () => {
	const registered = new Set(getBlockTypes().map((type) => type.name))

	expect(registered.has('core/missing')).toBe(true)
})

test('records no api-fetch attempt before anything calls one', () => {
	expect(apiFetchAttempts()).toEqual([])
})

test('refuses an api-fetch call and names the path it attempted', async () => {
	installApiFetchGuard()
	const apiFetch = (await import('@wordpress/api-fetch')).default

	await expect(apiFetch({ path: '/wp/v2/types' })).rejects.toThrow(/\/wp\/v2\/types/)

	expect(apiFetchAttempts()).toHaveLength(1)
	expect(apiFetchAttempts()[0]).toContain('/wp/v2/types')
})

test('names a caller that asked by url rather than by path', async () => {
	installApiFetchGuard()
	const apiFetch = (await import('@wordpress/api-fetch')).default

	await expect(apiFetch({ url: 'https://example.com/wp-json/wp/v2/media' })).rejects.toThrow()

	expect(apiFetchAttempts()[0]).toContain('/wp-json/wp/v2/media')
})

test('refuses a caller that named neither a path nor a url', async () => {
	installApiFetchGuard()
	const apiFetch = (await import('@wordpress/api-fetch')).default

	await expect(apiFetch({})).rejects.toThrow(/no path/)

	expect(apiFetchAttempts()).toEqual(['(no path)'])
})

test('records an attempt even when the caller swallows the refusal', async () => {
	installApiFetchGuard()
	const apiFetch = (await import('@wordpress/api-fetch')).default

	try {
		await apiFetch({ path: '/wp/v2/blocks' })
	} catch {
		// Core data resolvers swallow their fetch errors, so the spy is the only witness.
	}

	expect(apiFetchAttempts()).toHaveLength(1)
	expect(apiFetchAttempts()[0]).toContain('/wp/v2/blocks')
})
