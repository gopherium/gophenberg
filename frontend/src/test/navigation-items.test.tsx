// SPDX-License-Identifier: Apache-2.0

import {
	brushIcon,
	categoryIcon,
	customPostTypeIcon,
	languageIcon,
	listIcon,
	mediaIcon,
	pageIcon,
	postIcon,
	settingsIcon,
	updateIcon,
} from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { render, screen, within } from '@testing-library/react'
import { people } from '@wordpress/icons'
import type { ReactElement } from 'react'
import { beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { GUIDE_TYPE, PAGE_TYPE, POST_TYPE, serveTypes } from './typeFixtures'

const ROW = 'godmin-navigation-item'

/** The built-in type that groups posts, as the registry answers it. */
const CATEGORY_TYPE = {
	...POST_TYPE,
	key: 'category',
	singular_label: 'Category',
	plural_label: 'Categories',
	description: 'Manage the categories on this site.',
	route_word: 'categories',
	derived_route_word: 'categories',
	default: false,
}

beforeEach(() => {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/content/counts', () => HttpResponse.json({})),
	)
})

/**
 * Returns what a glyph draws, read from the row holding it or from the glyph itself.
 * @param held - The row link holding the glyph, or the glyph.
 * @returns The markup inside its svg.
 */
function drawingOf(held: Element | ReactElement): string | undefined {
	const element = held instanceof Element ? held : render(held).container
	return element.querySelector('svg')?.innerHTML
}

test('dresses a content type row like every other sidebar row', async () => {
	renderAt('/content/post')

	const sidebar = await screen.findByRole('region', { name: 'Navigation' })
	expect(await within(sidebar).findByRole('link', { name: 'Posts' })).toHaveClass(ROW)
	expect(within(sidebar).getByRole('link', { name: 'Media' })).toHaveClass(ROW)
})

test('lists every sidebar row as an item of one list, the types first', async () => {
	renderAt('/content/post')

	const sidebar = await screen.findByRole('region', { name: 'Navigation' })
	await within(sidebar).findByRole('link', { name: 'Posts' })

	const rows = within(within(sidebar).getByRole('list')).getAllByRole('listitem')
	expect(rows.map((row) => row.textContent)).toEqual([
		'Posts',
		'Media',
		'Themes',
		'Content Types',
		'Field Groups',
		'Users',
		'Language',
		'Settings',
		'Updates',
	])
})

test('draws each sidebar row with the glyph WordPress draws for it, other types with the Posts glyph', async () => {
	serveTypes(POST_TYPE, PAGE_TYPE, CATEGORY_TYPE, GUIDE_TYPE)
	renderAt('/content/post')

	const sidebar = within(await screen.findByRole('region', { name: 'Navigation' }))
	await sidebar.findByRole('link', { name: 'Guides' })

	const glyphs: [string, ReactElement][] = [
		['Posts', postIcon],
		['Pages', pageIcon],
		['Categories', categoryIcon],
		['Guides', postIcon],
		['Media', mediaIcon],
		['Themes', brushIcon],
		['Content Types', customPostTypeIcon],
		['Field Groups', listIcon],
		['Users', people],
		['Language', languageIcon],
		['Settings', settingsIcon],
		['Updates', updateIcon],
	]
	for (const [name, glyph] of glyphs) {
		expect(drawingOf(sidebar.getByRole('link', { name })), name).toBe(drawingOf(glyph))
	}
})

test('draws the Posts glyph for a plugin type under a built-in key, as it keeps the plugin words', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, origin: 'shop' })
	renderAt('/content/post')

	const sidebar = within(await screen.findByRole('region', { name: 'Navigation' }))
	const row = await sidebar.findByRole('link', { name: 'Pages' })

	expect(drawingOf(row)).toBe(drawingOf(postIcon))
})
