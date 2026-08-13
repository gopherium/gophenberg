// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'

import { typesQueryKey } from './nav'
import { listTypes } from './types'
import type { ContentType } from './types'

/** The type a screen falls back to before the registry answers. */
const unknownType: ContentType = {
	key: '',
	singularLabel: 'Content',
	pluralLabel: 'Content',
	routeWord: '',
	hierarchical: false,
	revisions: true,
	revisionCap: 0,
	pageKind: 'single',
	isDefault: false,
	active: true,
}

/**
 * Returns the content type the current route addresses.
 * @returns The registered type, or a placeholder until the registry answers.
 */
export function useContentType(): ContentType {
	const { typeKey } = useParams({ strict: false })
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const found = types.data?.find((registered) => registered.key === typeKey)
	return found ?? { ...unknownType, key: typeKey ?? '' }
}
