// SPDX-License-Identifier: Apache-2.0

import { ADMIN, AUTHOR, EDITOR } from './users/ranks'

/** A named permission a screen or control asks for. */
export type Capability =
	| 'manage_users'
	| 'manage_themes'
	| 'manage_types'
	| 'manage_settings'

/** The capability administering accounts. */
export const MANAGE_USERS: Capability = 'manage_users'

/** The capability installing and switching themes. */
export const MANAGE_THEMES: Capability = 'manage_themes'

/** The capability reshaping the content model. */
export const MANAGE_TYPES: Capability = 'manage_types'

/** The capability writing the site wide settings. */
export const MANAGE_SETTINGS: Capability = 'manage_settings'

/** The capabilities each rank carries. */
const carried: Record<string, Capability[]> = {
	[ADMIN]: [MANAGE_USERS, MANAGE_THEMES, MANAGE_TYPES, MANAGE_SETTINGS],
	[EDITOR]: [],
	[AUTHOR]: [],
}

/**
 * Reports whether a rank holds the capability, an unknown rank holding none.
 * @param rank - The rank the session carries, missing counting as none.
 * @param capability - The capability the decision point asks for.
 * @returns Whether the rank holds it.
 */
export function can(rank: string | undefined, capability: Capability): boolean {
	return (carried[rank ?? ''] ?? []).includes(capability)
}

declare module '@tanstack/react-router' {
	interface StaticDataRouteOption {
		capability?: Capability
	}
}
