// SPDX-License-Identifier: Apache-2.0

import { parse } from '@wordpress/block-serialization-default-parser'
import type { AstroComponentFactory } from 'astro/runtime/server/index.js'

import type { PostSummary } from './content.ts'

/** One parsed block with its raw inner HTML and its children. */
export interface BlockNode {
	name: string | null
	attrs: Record<string, unknown>
	innerHTML: string
	innerContent: (string | null)[]
	innerBlocks: BlockNode[]
}

/** Binds fully qualified block names to the components rendering them. */
export type BlockComponentMap = Record<string, AstroComponentFactory>

/** What every block component receives. */
export interface BlockProps {
	block: BlockNode
	components?: BlockComponentMap
	post?: PostSummary
}

/** One piece of a block rendered verbatim: saved markup, or a child to render in its place. */
export type BlockSegment = { html: string } | { child: BlockNode }

/** One block as the parser reports it, whose attributes the parser documents as nullable. */
export interface ParsedBlock {
	blockName: string | null
	attrs: Record<string, unknown> | null
	innerHTML: string
	innerContent: (string | null)[]
	innerBlocks: ParsedBlock[]
}

/**
 * Returns the blocks a stored document holds.
 * @param html - The delimiter-bearing markup the editor saved.
 * @returns The blocks in the order they were saved.
 */
export function parseBlocks(html: string): BlockNode[] {
	return (parse(html) as ParsedBlock[]).map(toBlockNode)
}

/**
 * Returns a parsed block as the shape a theme reads.
 * @param parsed - The block as the parser reports it.
 * @returns The block node.
 */
export function toBlockNode(parsed: ParsedBlock): BlockNode {
	return {
		name: parsed.blockName,
		attrs: parsed.attrs ?? {},
		innerHTML: parsed.innerHTML,
		innerContent: parsed.innerContent,
		innerBlocks: parsed.innerBlocks.map(toBlockNode),
	}
}

/**
 * Returns the pieces a block renders as when no component overrides it.
 * @param block - The block to render.
 * @returns The saved markup interleaved with the children it wraps.
 */
export function verbatimSegments(block: BlockNode): BlockSegment[] {
	const segments: BlockSegment[] = []
	let taken = 0
	for (const piece of block.innerContent) {
		if (piece !== null) {
			segments.push({ html: piece })
			continue
		}
		const child = block.innerBlocks[taken]
		taken += 1
		if (child) {
			segments.push({ child })
		}
	}
	return segments
}
