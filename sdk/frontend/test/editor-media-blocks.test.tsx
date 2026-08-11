// SPDX-License-Identifier: Apache-2.0

import { createBlock } from '@wordpress/blocks'
import { beforeAll, expect, test } from 'vitest'

import { parse, registerCuratedBlocks, serialize } from '../editor'

beforeAll(async () => {
	registerCuratedBlocks()
	await Promise.resolve()
}, 120000)

const placements: [string, Record<string, unknown>][] = [
	['core/image', { id: 7, url: '/media/2026/08/harbor.jpg', alt: 'Boats at sunrise' }],
	[
		'core/gallery',
		{
			images: [
				{ id: 7, url: '/media/2026/08/harbor.jpg' },
				{ id: 9, url: '/media/2026/08/cliff.jpg' },
			],
		},
	],
	['core/video', { id: 11, src: '/media/2026/08/launch.mp4' }],
	['core/audio', { id: 12, src: '/media/2026/08/chime.mp3' }],
	['core/file', { id: 13, href: '/media/2026/08/manual.pdf', fileName: 'manual.pdf' }],
	['core/cover', { id: 7, url: '/media/2026/08/harbor.jpg' }],
	['core/media-text', { mediaId: 7, mediaUrl: '/media/2026/08/harbor.jpg', mediaType: 'image' }],
]

test.each(placements)('%s serialized markup survives a reload whole', (name, attributes) => {
	const placed = createBlock(name, attributes)

	const reloaded = parse(serialize([placed]))

	expect(reloaded).toHaveLength(1)
	expect(reloaded[0].name).toBe(name)
	expect(reloaded[0].isValid).toBe(true)
	expect(serialize(reloaded)).toBe(serialize([placed]))
})
