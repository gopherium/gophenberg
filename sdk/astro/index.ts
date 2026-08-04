// SPDX-License-Identifier: Apache-2.0

/** The package a theme depends on. */
export const kitName = '@gophenberg/astro'

/** The address a theme reads published content through. */
export const contentApiPath = '/api/content/v1'

/** A block name is a lowercase slug, optionally namespaced by a vendor. */
const blockName = /^[a-z][a-z0-9-]*(\/[a-z][a-z0-9-]*)?$/

/**
 * Reports whether a value is a block name the editor serializes.
 * @param value - The candidate name.
 * @returns True when the editor could have written it.
 */
export function isBlockName(value: string): boolean {
	return blockName.test(value)
}

export type { BlockNode, BlockProps } from './blocks.ts'
export type { Page, Post, PostSummary } from './content.ts'
