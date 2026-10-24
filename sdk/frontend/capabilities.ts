// SPDX-License-Identifier: Apache-2.0

/** The role holding every authority over the site. */
export const ADMIN = 'admin'

/** The role working every account's content and media. */
export const EDITOR = 'editor'

/** The role working only its own content and media. */
export const AUTHOR = 'author'

/** The role of a signed-in account that never enters the admin. */
export const MEMBER = 'member'

/** A named permission a screen or control asks for, the core's or one a plugin declares. */
export type Capability = string

/** The capability entering the admin and working its content and media. */
export const ACCESS_ADMIN: Capability = 'access_admin'

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

/** The capability checking for and installing a new version of Gophenberg. */
export const UPDATE_CORE: Capability = 'update_core'

/**
 * Reports whether an account carrying the given capabilities may change work the author owns.
 * @param capabilities - The capabilities the session carries, missing counting as none.
 * @param actor - The signed in account.
 * @param author - The account that wrote the work.
 * @returns Whether the change is allowed.
 */
export function mayChange(capabilities: string[] | undefined, actor: string, author: string): boolean {
	return (capabilities ?? []).includes(CHANGE_OTHERS_WORK) || actor === author
}

/**
 * Reports whether the session may change work the author owns, no session changing nothing.
 * @param session - The signed in account, or nothing.
 * @param author - The account that wrote the work.
 * @returns Whether the change is allowed.
 */
export function sessionMayChange(
	session: { id?: string, capabilities?: string[] } | null | undefined,
	author: string,
): boolean {
	return mayChange(session?.capabilities, session?.id ?? '', author)
}
