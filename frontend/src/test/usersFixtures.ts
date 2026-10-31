// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import type { Account } from '@gopherium/react-auth'
import { accountWithRole, usersOk } from '@gopherium/react-auth/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, expect } from 'vitest'

import { adminUser } from './render'

/** An account holding the editor role, one the signed in administrator may act on. */
export const EDITOR = accountWithRole('editor')

/** The account the tests sign in as, as the list shows its own row. */
export const SIGNED_IN = accountWithRole('admin', { id: adminUser.id, name: adminUser.name, email: adminUser.email })

/** The error of a write sent from a page on another site, a code react-auth has no words for. */
export const REQUEST_CROSS_ORIGIN = { error: 'cross-origin request refused', code: 'request_cross_origin' }

/** The words the admin gives that error. */
export const REQUEST_CROSS_ORIGIN_WORDS =
	'This request came from a page on another site, so it was refused. Open the admin on this site and try again.'

/** Loads the users screen before a file renders its lazily mounted route. */
export function warmUsersScreen() {
	beforeAll(async () => {
		await import('../users/UsersScreen')
	})
}

/**
 * Serves the accounts as the account list.
 * @param accounts - The accounts the list answers.
 */
export function serveAccounts(...accounts: Account[]) {
	server.use(usersOk(accounts))
}

/**
 * Serves how many posts each account wrote.
 * @param counts - The post count of each account, keyed by account id.
 */
export function servePostCounts(counts: Record<string, number>) {
	server.use(http.get('/api/users/post-counts', () => HttpResponse.json({ counts })))
}

/**
 * Returns the table row showing the words, once it shows.
 * @param text - Words the row shows, such as the name of its account.
 * @returns The row.
 */
export function rowShowing(text: string): Promise<HTMLElement> {
	return waitFor(() => {
		const row = screen.getAllByRole('row').find((candidate) => within(candidate).queryByText(text) !== null)
		expect(row).toBeDefined()
		return row as HTMLElement
	})
}

/**
 * Returns the queries of the table row showing the words, once it shows.
 * @param text - Words the row shows, such as the name of its account.
 * @returns The queries bound to the row.
 */
export async function rowOf(text: string) {
	return within(await rowShowing(text))
}

/**
 * Picks an item of the actions menu of the row showing the words.
 * @param text - Words the row shows.
 * @param action - The words of the menu item.
 */
export async function act(text: string, action: string) {
	await userEvent.click((await rowOf(text)).getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: action }))
}

/**
 * Returns the toast showing the words, once it shows.
 * @param text - The words of the toast.
 * @returns The toast.
 */
export function toast(text: string): Promise<HTMLElement> {
	return screen.findByText(text, { selector: '.godmin-toast' })
}
