// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'
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
	get label() {
		return __('Content Types', 'gophenberg')
	},
	to: '/content-types',
	icon: typesIcon,
}

/** The key the registry is cached under. */
export const typesQueryKey = ['content-types']

const groupsIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path d="M3 4h18v4H3V4zm2 2v0h14V6H5zm-2 5h18v4H3v-4zm2 2v0h14v0H5zm-2 5h18v4H3v-4z" />
	</svg>
)

/** The field groups section, where fields live and their rules are set. */
export const groupsNavItem: NavItem = {
	get label() {
		return __('Field Groups', 'gophenberg')
	},
	to: '/field-groups',
	icon: groupsIcon,
}

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
 * Returns one nav entry per active content type, the root type first.
 * @returns The entries, empty until the registry answers.
 */
export function useContentNav(): NavItem[] {
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	return (types.data ?? [])
		.filter((registered) => registered.active)
		.sort((left, right) => Number(right.isDefault) - Number(left.isDefault))
		.map(typeNavItem)
}
