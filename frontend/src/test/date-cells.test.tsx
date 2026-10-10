// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { getSettings, setSettings } from '@wordpress/date'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import type { ReactElement } from 'react'
import { afterEach, expect, onTestFinished, test } from 'vitest'

import { postFields } from '../content/fields'
import { placeholderType } from '../content/useContentType'
import { DEFAULT_LOCALE, catalogFor } from '../i18n/catalog'
import { startDates } from '../i18n/dates'
import { rememberFormatLocale, rememberLocale } from '@gopherium/gottext'
import { mediaFields } from '../media/fields'
import type { MediaItem } from '../media/api'
import type { Post } from '../content/api'

const NOON = '2026-08-16T12:00:00Z'

/** The date settings in place before any test hands the dates a language. */
const DATE_DEFAULTS = getSettings()

afterEach(() => {
	rememberLocale(DEFAULT_LOCALE)
	setSettings(DATE_DEFAULTS)
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

const post: Post = {
	id: '1',
	type: 'post',
	parentId: null,
	parentTitle: '',
	path: '',
	slug: 'one',
	title: 'One',
	status: 'published',
	excerpt: '',
	authorId: '',
	authorName: '',
	publishedAt: NOON,
	createdAt: NOON,
	updatedAt: NOON,
	date: NOON,
}

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

test('shows a post date in the abbreviated date and time format WordPress lists dates in, not the site format', () => {
	rememberFormatLocale('es-ES')
	startDates(DEFAULT_LOCALE)

	renderDate(postFields(placeholderType('post')), post)

	expect(screen.getByText('Published: Aug 16, 2026 12:00 pm')).toBeInTheDocument()
})

test('captions and writes a publication date as WordPress es_ES does for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	rememberFormatLocale('es-ES')
	startDates('es-ES')

	renderDate(postFields(placeholderType('post')), post)

	expect(screen.getByText('Publicado: 16 Ago 2026 12:00')).toBeInTheDocument()
})

test.each(['es-ES', 'en-US'])('shows a media date in the site format to a reader of %s', (reader) => {
	rememberFormatLocale('es-ES')
	rememberLocale(reader)

	renderDate(mediaFields, item)

	expect(screen.getByText('16/08/2026')).toBeInTheDocument()
})
