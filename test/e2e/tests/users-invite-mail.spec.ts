// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

import { mailURL } from '../env'
import { ACTIVATE_PATH, DEAD_LINK_WORDS, accountFor, activate, freshContext } from '../invites'
import { linkFrom, startMailSink } from '../mail'
import type { MailSink } from '../mail'

let sink: MailSink

test.use({ baseURL: mailURL })

test.beforeAll(async () => {
	sink = await startMailSink()
})

test.afterAll(async () => {
	await sink.close()
})

test.beforeEach(() => {
	sink.clear()
})

/**
 * Returns the toast reading the words.
 * @param page - The page holding the toast.
 * @param words - The words of the toast.
 * @returns The toast locator.
 */
function toastOf(page: Page, words: string) {
	return page.locator('.godmin-toast', { hasText: words })
}

test('mails the invitation from Add User to a name with accents, says so on the list, and the mailed link activates the account', async ({
	page,
	browser,
}) => {
	const account = accountFor('mailed', 'María Pérez')
	await page.goto('/admin/users/new')
	await page.getByRole('textbox', { name: /^Email/ }).fill(account.email)
	await page.getByRole('textbox', { name: /^Name/ }).fill(account.name)

	await page.getByRole('button', { name: 'Send invitation', exact: true }).click()

	await expect(page).toHaveURL(/\/admin\/users$/)
	await expect(toastOf(page, 'Invitation sent.')).toBeVisible()
	const body = await sink.waitFor(account.email)
	expect(body).toContain(`Hello ${account.name},`)
	const context = await freshContext(browser, mailURL)
	const activation = await context.newPage()
	await activate(activation, linkFrom(body, ACTIVATE_PATH))
	await expect(activation.getByRole('heading', { name: 'Posts', level: 2 })).toBeVisible()
	await expect(activation.locator('.godmin-sidebar__footer').getByText(account.name)).toBeVisible()
	await context.close()
})

test('mails a new link from Resend invitation, the link mailed before no longer valid', async ({ page, browser }) => {
	const account = accountFor('remailed')
	const invited = await page.request.post('/api/users/invite', { data: { ...account, role: 'editor' } })
	expect(invited.status()).toBe(200)
	expect(((await invited.json()) as { delivered: boolean }).delivered).toBe(true)
	const first = linkFrom(await sink.waitFor(account.email), ACTIVATE_PATH)
	sink.clear()
	await page.goto(`/admin/users?search=${encodeURIComponent(account.email)}`)
	const row = page.getByRole('row').filter({ hasText: account.email })
	await expect(row.getByText('Invited', { exact: true })).toBeVisible()

	await row.getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Resend invitation', exact: true }).click()
	const dialog = page.getByRole('dialog', { name: 'Resend invitation' })
	await dialog.getByRole('button', { name: 'Resend invitation', exact: true }).click()

	await expect(toastOf(page, `Invitation sent to "${account.name}".`)).toBeVisible()
	await expect(dialog).toBeHidden()
	const second = linkFrom(await sink.waitFor(account.email), ACTIVATE_PATH)
	expect(second).not.toBe(first)
	const context = await freshContext(browser, mailURL)
	const activation = await context.newPage()
	await activate(activation, first)
	await expect(activation.getByRole('alert')).toHaveText(DEAD_LINK_WORDS)
	await activate(activation, second)
	await expect(activation.getByRole('heading', { name: 'Posts', level: 2 })).toBeVisible()
	await context.close()
})
