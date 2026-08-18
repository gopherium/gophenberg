// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import { typesQueryKey } from './nav'
import { listTypes } from './types'
import type { ContentType } from './types'

/** The type a screen falls back to before the registry answers. */
const unknownType: ContentType = {
	key: '',
	get singularLabel() {
		return __('Content', 'gophenberg')
	},
	get pluralLabel() {
		return __('Content', 'gophenberg')
	},
	routeWord: '',
	hierarchical: false,
	revisions: true,
	revisionCap: 0,
	pageKind: 'single',
	isDefault: false,
	active: true,
	fields: [],
}

/**
 * Returns the type a screen stands in with until the registry answers.
 * @param key - The type key the route carries, absent off a content route.
 * @returns The placeholder type.
 */
export function placeholderType(key: string | undefined): ContentType {
	return { ...unknownType, key: key ?? '' }
}

/**
 * Returns the content type the current route addresses.
 * @returns The registered type, or a placeholder until the registry answers.
 */
export function useContentType(): ContentType {
	const { typeKey } = useParams({ strict: false })
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const found = types.data?.find((registered) => registered.key === typeKey)
	return found ?? placeholderType(typeKey)
}
