// SPDX-License-Identifier: Apache-2.0

import { __experimentalGetCoreBlocks, registerCoreBlocks } from '@wordpress/block-library'

export const CURATED_BLOCKS = [
	'core/paragraph',
	'core/heading',
	'core/list',
	'core/list-item',
	'core/quote',
	'core/pullquote',
	'core/code',
	'core/preformatted',
	'core/verse',
	'core/table',
	'core/image',
	'core/separator',
	'core/spacer',
	'core/group',
	'core/columns',
	'core/column',
	'core/buttons',
	'core/button',
	'core/details',
	'core/html',
	'core/more',
	'core/nextpage',
	'core/missing',
] as const

/**
 * Registers the block types the editor offers.
 */
export function registerCuratedBlocks(): void {
	const curated = new Set<string>(CURATED_BLOCKS)
	registerCoreBlocks(
		__experimentalGetCoreBlocks().filter((block) => curated.has(block.metadata.name)),
	)
}
