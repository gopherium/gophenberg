// SPDX-License-Identifier: Apache-2.0

/** The role holding every authority over the site. */
export const ADMIN = 'admin'

/** The role working every account's content and media. */
export const EDITOR = 'editor'

/** The role working only its own content and media. */
export const AUTHOR = 'author'

/** A named permission a screen or control asks for. */
export type Capability =
	| 'manage_users'
	| 'manage_themes'
	| 'manage_types'
	| 'manage_settings'
	| 'change_others_work'

/** The capability administering accounts. */
export const MANAGE_USERS: Capability = 'manage_users'

/** The capability installing and switching themes. */
export const MANAGE_THEMES: Capability = 'manage_themes'

/** The capability reshaping the content model. */
export const MANAGE_TYPES: Capability = 'manage_types'

/** The capability writing the site wide settings. */
export const MANAGE_SETTINGS: Capability = 'manage_settings'

/** The capability changing content and media another account wrote. */
export const CHANGE_OTHERS_WORK: Capability = 'change_others_work'

/** The capabilities each role carries. */
const carried: Record<string, Capability[]> = {
	[ADMIN]: [MANAGE_USERS, MANAGE_THEMES, MANAGE_TYPES, MANAGE_SETTINGS, CHANGE_OTHERS_WORK],
	[EDITOR]: [CHANGE_OTHERS_WORK],
	[AUTHOR]: [],
}

/**
 * Reports whether a role holds the capability, an unknown role holding none.
 * @param role - The role the session carries, missing counting as none.
 * @param capability - The capability the decision point asks for.
 * @returns Whether the role holds it.
 */
export function can(role: string | undefined, capability: Capability): boolean {
	return (carried[role ?? ''] ?? []).includes(capability)
}

/**
 * Reports whether an account of the given role may change work the author owns.
 * @param role - The role the session carries, missing counting as none.
 * @param actor - The signed in account.
 * @param author - The account that wrote the work.
 * @returns Whether the change is allowed.
 */
export function mayChange(role: string | undefined, actor: string, author: string): boolean {
	return can(role, CHANGE_OTHERS_WORK) || actor === author
}

/**
 * Reports whether the session may change work the author owns, no session changing nothing.
 * @param session - The signed in account, or nothing.
 * @param author - The account that wrote the work.
 * @returns Whether the change is allowed.
 */
export function sessionMayChange(
	session: { role?: string, id?: string } | null | undefined,
	author: string,
): boolean {
	return mayChange(session?.role, session?.id ?? '', author)
}
