// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { useQuery } from '@tanstack/react-query'

import { listTypes } from './types'
import type { ContentType } from './types'

const contentIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path d="M5 3h11l3 3v15H5V3zm2 2v14h10V7h-3V5H7zm2 4h6v2H9V9zm0 4h6v2H9v-2z" />
	</svg>
)

const typesIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path
			d="M4 4h7v7H4V4zm2 2v3h3V6H6zm7-2h7v7h-7V4zm2 2v3h3V6h-3zM4 13h7v7H4v-7zm2 2v3h3v-3H6zm7-2h7v7h-7v-7zm2 2v3h3v-3h-3z"
		/>
	</svg>
)

/** The registry section, which is not itself a content type. */
export const typesNavItem: NavItem = {
	label: 'Content Types',
	to: '/content-types',
	icon: typesIcon,
}

/** The key the registry is cached under. */
export const typesQueryKey = ['content-types']

/**
 * Returns the nav entry a content type is reached through.
 * @param registered - The type the entry stands for.
 * @returns The nav entry for that type.
 */
export function typeNavItem(registered: ContentType): NavItem {
	return {
		label: registered.pluralLabel,
		to: `/content/${registered.key}`,
		icon: contentIcon,
	}
}

/**
 * Returns one nav entry per active content type, in registration order.
 * @returns The entries, empty until the registry answers.
 */
export function useContentNav(): NavItem[] {
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	return (types.data ?? []).filter((registered) => registered.active).map(typeNavItem)
}
