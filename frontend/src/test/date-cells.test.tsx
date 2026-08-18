// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import type { ReactElement } from 'react'
import { afterEach, expect, test } from 'vitest'

import { postFields } from '../content/fields'
import { DEFAULT_LOCALE } from '../i18n/catalog'
import { rememberLocale } from '../i18n/display'
import { mediaFields } from '../media/fields'
import type { MediaItem } from '../media/api'
import type { Post } from '../content/api'

const NOON = '2026-08-16T12:00:00Z'

afterEach(() => {
	rememberLocale(DEFAULT_LOCALE)
})

/**
 * Renders the date cell a field list carries.
 * @param fields - The fields the screen lists with.
 * @param item - The row to render.
 */
function renderDate<T>(fields: { id: string, render?: unknown }[], item: T) {
	const field = fields.find((held) => held.id === 'date')!
	const Cell = field.render as (props: { item: T }) => ReactElement
	render(<Cell item={item} />)
}

const post = {
	id: '1',
	type: 'post',
	parentId: null,
	path: '',
	slug: 'one',
	title: 'One',
	status: 'publish',
	excerpt: '',
	authorName: '',
	publishedAt: NOON,
	createdAt: NOON,
	updatedAt: NOON,
} as Post

const item: MediaItem = {
	id: 1,
	type: 'image',
	file: 'one.png',
	title: 'One',
	altText: '',
	caption: '',
	description: '',
	mimeType: 'image/png',
	width: 1,
	height: 1,
	filesize: 1,
	sizes: {},
	authorId: '1',
	createdAt: NOON,
	updatedAt: NOON,
}

test('shows a post date in the language the reader settled on', () => {
	rememberLocale('es-ES')

	renderDate(postFields, post)

	expect(screen.getByText(/16\/8\/2026/)).toBeInTheDocument()
})

test('shows a media date in the language the reader settled on', () => {
	rememberLocale('es-ES')

	renderDate(mediaFields, item)

	expect(screen.getByText('16/8/2026')).toBeInTheDocument()
})
