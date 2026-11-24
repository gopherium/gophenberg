// SPDX-License-Identifier: Apache-2.0

import { category, customPostType, list, page, post } from '@gophenberg/frontend-sdk'
import type { MenuItem } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'
import { useQuery } from '@tanstack/react-query'

import { pluralOf } from './typeText'
import { listTypes } from './types'
import type { ContentType } from './types'

/** The registry section, which is not itself a content type. */
export const typesMenuItem = {
	get label() {
		return _x('Content Types', 'admin section', 'gophenberg')
	},
	to: '/content-types',
	icon: customPostType,
} satisfies MenuItem

/** The key the registry is cached under. */
export const typesQueryKey = ['content-types']

/** The field groups section, where fields live and their rules are set. */
export const groupsMenuItem: MenuItem = {
	get label() {
		return _x('Field Groups', 'admin section', 'gophenberg')
	},
	to: '/field-groups',
	icon: list,
}

/** The glyphs of the built-in types WordPress draws apart from posts, by type key. */
const TYPE_GLYPHS = new Map([
	['page', page],
	['category', category],
])

/**
 * Returns the glyph of a type's row: WordPress's own for a built-in type no plugin declared, the Posts glyph otherwise.
 * @param registered - The type the row stands for.
 * @returns The glyph the row draws.
 */
function glyphOf(registered: ContentType): MenuItem['icon'] {
	const shipped = (registered.origin ?? '') === '' ? TYPE_GLYPHS.get(registered.key) : undefined
	return shipped ?? post
}

/**
 * Returns the menu row a content type is reached through.
 * @param registered - The type the row stands for.
 * @returns The menu row for that type.
 */
function typeMenuItem(registered: ContentType): MenuItem {
	return {
		label: pluralOf(registered),
		to: `/content/${registered.key}`,
		icon: glyphOf(registered),
	}
}

/**
 * Returns one menu row per active content type, the root type first.
 * @returns The rows, empty until the registry answers.
 */
export function useContentNav(): MenuItem[] {
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	return (types.data ?? [])
		.filter((registered) => registered.active)
		.sort((left, right) => Number(right.isDefault) - Number(left.isDefault))
		.map(typeMenuItem)
}

/** What the registry says of the type the root opens. */
interface DefaultType {
	/** The key of the default type, undefined until the registry answers or when it could not be read. */
	key: string | undefined
	/** Whether the registry could not be read. */
	failed: boolean
}

/**
 * Returns the type the root opens, read from the same registry the menu rows come from.
 * @returns The default type's key once the registry answers, and whether it failed.
 */
export function useDefaultType(): DefaultType {
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	return {
		key: types.data?.find((registered) => registered.isDefault)?.key,
		failed: types.isError,
	}
}
