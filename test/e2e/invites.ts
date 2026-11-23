// SPDX-License-Identifier: Apache-2.0

import { test } from '@playwright/test'
import type { Browser, BrowserContext, Page } from '@playwright/test'

import { credentials } from './env'

/** A mark this run alone carries, so its accounts never meet those of another run. */
const RUN = Math.random().toString(36).slice(2, 8)

/** The admin page an activation link opens. */
export const ACTIVATE_PATH = '/admin/activate'

/** The words the activation page shows for a link that no longer works. */
export const DEAD_LINK_WORDS = 'This link is no longer valid. Ask the person who invited you for a new one.'

/**
 * Returns an address and a name no other run or repeat shares, named by the condition they stand for.
 * @param condition - The condition the account stands for.
 * @param name - The name the account's own name starts with.
 * @returns The address and the name.
 */
export function accountFor(condition: string, name = 'Maria Perez'): { email: string; name: string } {
	const mark = `${RUN}${test.info().repeatEachIndex}`
	return { email: `${mark}-${condition}@example.com`, name: `${name} ${condition} ${mark}` }
}

/**
 * Opens a browser context with no session, as the invited person's own browser.
 * @param browser - The browser of the test.
 * @param baseURL - The address of the server the context opens.
 * @returns The context.
 */
export async function freshContext(browser: Browser, baseURL: string): Promise<BrowserContext> {
	return browser.newContext({ baseURL, storageState: { cookies: [], origins: [] } })
}

/**
 * Opens an activation link and sets the password on the page it opens.
 * @param page - The page that opens the link.
 * @param link - The activation link.
 */
export async function activate(page: Page, link: string) {
	await page.goto(link)
	await page.getByLabel('Password').fill(credentials.password)
	await page.getByRole('button', { name: 'Set password' }).click()
}
