// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Page } from '@playwright/test'

import { baseURL, credentials } from '../env'
import { ACTIVATE_PATH, DEAD_LINK_WORDS, accountFor, activate, freshContext } from '../invites'

/** The link the no-mail server answers, a path on the site since that server names no public address. */
const BARE_LINK = /^\/admin\/activate\?token=\S+$/

/** The words Add User and Resend invitation show in place of a mail that went nowhere. */
const NO_MAIL_WORDS = 'No mail server is configured. Deliver the activation link by hand.'

/** An account invited under an address no other run uses, and its activation link. */
interface Invited {
	email: string
	name: string
	link: string
}

/**
 * Invites an account through the API Add User calls and returns the link the no-mail server answers.
 * @param page - The page of the signed in administrator.
 * @param condition - The condition the account stands for, naming it.
 * @param role - The role the account starts under.
 * @returns The account and its activation link.
 */
async function invitedAs(page: Page, condition: string, role: string): Promise<Invited> {
	const account = accountFor(condition)
	const answer = await page.request.post('/api/users/invite', { data: { ...account, role } })
	expect(answer.status()).toBe(200)
	const invitation = (await answer.json()) as { delivered: boolean; activation_link: string }
	expect(invitation.delivered).toBe(false)
	return { ...account, link: invitation.activation_link }
}

/** What the activation page met, kept up to date while the page shows it. */
interface ActivationRecord {
	problems: string[]
	answered: string[]
}

/**
 * Follows the page's main frame and answers whether it still shows the activation page.
 * @param page - The page that opens the activation link.
 * @returns A check that answers true while the activation page is shown.
 */
function activationShown(page: Page): () => boolean {
	let open = true
	page.on('framenavigated', (frame) => {
		if (frame === page.mainFrame()) {
			open = new URL(frame.url()).pathname === ACTIVATE_PATH
		}
	})
	return () => open
}

/**
 * Records every console error and every failed request the page meets while the activation page is shown.
 * @param page - The page that opens the activation link.
 * @param shown - Answers whether the page still shows the activation page.
 * @param problems - The list each failure joins.
 */
function recordFailures(page: Page, shown: () => boolean, problems: string[]): void {
	page.on('console', (message) => {
		if (shown() && message.type() === 'error') {
			problems.push(`console error: ${message.text()}`)
		}
	})
	page.on('requestfailed', (request) => {
		if (shown()) {
			problems.push(`${request.failure()?.errorText}: ${request.method()} ${request.url()}`)
		}
	})
}

/**
 * Records every API answer, and every answer that failed, the page meets while the activation page is shown.
 * @param page - The page that opens the activation link.
 * @param shown - Answers whether the page still shows the activation page.
 * @param record - The record each answer joins.
 */
function recordAnswers(page: Page, shown: () => boolean, record: ActivationRecord): void {
	page.on('response', (response) => {
		if (!shown()) {
			return
		}
		const path = new URL(response.url()).pathname
		if (path.startsWith('/api/')) {
			record.answered.push(`${response.status()} ${response.request().method()} ${path}`)
		}
		if (response.status() >= 400) {
			record.problems.push(`${response.status()}: ${response.request().method()} ${response.url()}`)
		}
	})
}

/**
 * Records every console error, every failed request and every API answer the page meets while it shows the activation page.
 * @param page - The page that opens the activation link.
 * @returns What went wrong, empty while nothing did, and each API answer as its status, method and path.
 */
function recordActivation(page: Page): ActivationRecord {
	const record: ActivationRecord = { problems: [], answered: [] }
	const shown = activationShown(page)
	recordFailures(page, shown, record.problems)
	recordAnswers(page, shown, record)
	return record
}

/**
 * Returns the address of the account a page is signed in as.
 * @param page - The page whose session is read.
 * @returns The address, none when the page is signed out.
 */
async function signedInAs(page: Page): Promise<string | undefined> {
	const answer = await page.request.get('/api/auth/session')
	return answer.ok() ? ((await answer.json()) as { email: string }).email : undefined
}

test('invites from Add User with no mail server, shows the activation link with the focus on it and lists the account as Invited after Done', async ({
	page,
}) => {
	const account = accountFor('handed')
	await page.goto('/admin/users/new')
	await page.getByRole('textbox', { name: /^Email/ }).fill(account.email)
	await page.getByRole('textbox', { name: /^Name/ }).fill(account.name)

	await page.getByRole('button', { name: 'Send invitation', exact: true }).click()

	await expect(page.getByRole('status').filter({ hasText: NO_MAIL_WORDS })).toBeVisible()
	const link = page.getByRole('textbox', { name: 'Activation link' })
	await expect(link).toHaveValue(BARE_LINK)
	await expect(link).toBeFocused()
	await expect(page.getByRole('heading', { name: 'Add User', level: 2 })).toBeVisible()
	await page.getByRole('button', { name: 'Done', exact: true }).click()

	await expect(page).toHaveURL(/\/admin\/users$/)
	await page.goto(`/admin/users?search=${encodeURIComponent(account.email)}`)
	const row = page.getByRole('row').filter({ hasText: account.email })
	await expect(row.getByText('Invited', { exact: true })).toBeVisible()
})

test('sets the password from the link in a fresh browser and lands an editor in the admin, the activation page logging no error and failing no request', async ({
	page,
	browser,
}) => {
	const invited = await invitedAs(page, 'activated', 'editor')
	const context = await freshContext(browser, baseURL)
	const activation = await context.newPage()
	const record = recordActivation(activation)

	await activate(activation, invited.link)

	await expect(activation.getByRole('heading', { name: 'Posts', level: 2 })).toBeVisible()
	await expect(activation.locator('.godmin-sidebar__footer').getByText(invited.name)).toBeVisible()
	expect(await signedInAs(activation)).toBe(invited.email)
	expect(record.answered).toContain('200 POST /api/auth/activate')
	expect(record.problems).toEqual([])
	await context.close()
})

test('keeps the activation page out of the way back once the password is set', async ({ page, browser }) => {
	const invited = await invitedAs(page, 'gone-back', 'editor')
	const context = await freshContext(browser, baseURL)
	const activation = await context.newPage()
	await activate(activation, invited.link)
	await expect(activation.getByRole('heading', { name: 'Posts', level: 2 })).toBeVisible()

	await activation.goBack()

	await expect(activation).not.toHaveURL(new RegExp(ACTIVATE_PATH))
	await context.close()
})

test('lands an invited member on the public site, signed in', async ({ page, browser }) => {
	const invited = await invitedAs(page, 'public-landing', 'member')
	const context = await freshContext(browser, baseURL)
	const activation = await context.newPage()

	await activate(activation, invited.link)

	await expect(activation).toHaveURL(new URL('/', baseURL).href)
	await expect(activation.getByRole('banner').getByRole('link', { name: 'Gophenberg Starter' })).toBeVisible()
	expect(await signedInAs(activation)).toBe(invited.email)
	await context.close()
})

test('shows the activation page to a reader already signed in who opens a link, leaving that reader signed in', async ({
	page,
}) => {
	const invited = await invitedAs(page, 'opened-signed-in', 'editor')

	await page.goto(invited.link)

	await expect(page.getByRole('heading', { name: 'Gophenberg', level: 1 })).toBeVisible()
	await expect(page.getByLabel('Password')).toBeVisible()
	await expect(page.getByRole('button', { name: 'Set password' })).toBeVisible()
	await expect(page.getByRole('region', { name: 'Navigation' })).toHaveCount(0)
	expect(await signedInAs(page)).toBe(credentials.email)
})

test('says a spent link is no longer valid', async ({ page, browser }) => {
	const invited = await invitedAs(page, 'spent', 'editor')
	const context = await freshContext(browser, baseURL)
	const activation = await context.newPage()
	await activate(activation, invited.link)
	await expect(activation.getByRole('heading', { name: 'Posts', level: 2 })).toBeVisible()

	await activate(activation, invited.link)

	await expect(activation.getByRole('alert')).toHaveText(DEAD_LINK_WORDS)
	await expect(activation).toHaveURL(new RegExp(`${ACTIVATE_PATH}\\?token=`))
	await context.close()
})

test('shows a new activation link in Resend invitation with the focus on it and closes on Done', async ({ page }) => {
	const invited = await invitedAs(page, 'resent', 'editor')
	await page.goto(`/admin/users?search=${encodeURIComponent(invited.email)}`)
	const row = page.getByRole('row').filter({ hasText: invited.email })

	await row.getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Resend invitation', exact: true }).click()
	const dialog = page.getByRole('dialog', { name: 'Resend invitation' })
	await dialog.getByRole('button', { name: 'Resend invitation', exact: true }).click()

	await expect(dialog.getByText(NO_MAIL_WORDS)).toBeVisible()
	const link = dialog.getByRole('textbox', { name: 'Activation link' })
	await expect(link).toHaveValue(BARE_LINK)
	await expect(link).not.toHaveValue(invited.link)
	await expect(link).toBeFocused()
	await dialog.getByRole('button', { name: 'Done', exact: true }).click()
	await expect(dialog).toBeHidden()
})

test('keeps Send invitation on with the fields empty and shows the words under Email, sending nothing', async ({
	page,
}) => {
	const invites: string[] = []
	page.on('request', (request) => {
		if (new URL(request.url()).pathname === '/api/users/invite') {
			invites.push(request.method())
		}
	})
	await page.goto('/admin/users/new')
	const send = page.getByRole('button', { name: 'Send invitation', exact: true })
	await expect(send).toBeEnabled()

	await send.click()

	await expect(page.getByText('Please enter an email address.')).toBeVisible()
	await expect(page.getByRole('textbox', { name: /^Email/ })).toBeFocused()
	await expect(send).toBeEnabled()
	expect(invites).toEqual([])
})
