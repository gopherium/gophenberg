// SPDX-License-Identifier: Apache-2.0

/** The package a theme depends on. */
export const kitName = '@gophenberg/astro'

/** The address a theme reads published content through. */
export const contentApiPath = '/api/content/v1'

/** The post type a theme reads when it names none. */
export const defaultPostType = 'post'

/** The version this kit ships at. */
export const kitVersion = '0.0.0'

/** The feature version a themed page reports, the major and minor of kitVersion. */
export const kitFeatureVersion = kitVersion.split('.').slice(0, 2).join('.')

/** How a themed page names what served it. */
export const generator = `Gophenberg ${kitFeatureVersion}`

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
