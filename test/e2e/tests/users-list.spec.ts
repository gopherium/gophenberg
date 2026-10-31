// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { APIRequestContext, Locator, Page } from '@playwright/test'

import { baseURL, credentials } from '../env'

const PHONE = { width: 390, height: 844 }

const RUN = Math.random().toString(36).slice(2, 8)

/** The label the list and Add User read for each role the core declares. */
const ROLE_LABELS: Record<string, string> = {
	admin: 'Administrator',
	editor: 'Editor',
	author: 'Author',
	member: 'Member',
}

/** The role with the fewest capabilities the administrator may give, the one Add User starts on without a default. */
const NARROWEST_ROLE = 'member'

/** An account a test made for itself. */
interface MadeAccount {
	id: string
	name: string
	email: string
}

/** An account as the admin API lists it. */
interface ListedAccount {
	id: string
	email: string
	role: string
	disabled: boolean
}

/**
 * Returns the mark the running test puts in every address it makes, which no other run or repeat shares.
 * @returns The mark.
 */
function markOf(): string {
	return `${RUN}${test.info().repeatEachIndex}`
}

/**
 * Returns the colour the design system draws a brand link in, as the browser resolves it beside an element.
 * @param element - The element whose surroundings resolve the colour.
 * @returns The computed colour.
 */
async function brandLinkColorBeside(element: Locator): Promise<string> {
	return element.evaluate((node) => {
		const probe = document.createElement('span')
		probe.style.color = 'var(--wpds-color-foreground-interactive-brand, var(--wp-admin-theme-color, #3858e9))'
		node.parentElement?.append(probe)
		const color = getComputedStyle(probe).color
		probe.remove()
		return color
	})
}

/**
 * Creates an account through the API the admin uses, under an address no other run uses.
 * @param request - The request context of the signed in administrator.
 * @param condition - The condition the account stands for, naming it.
 * @param role - The role the account starts under.
 * @returns The account.
 */
async function madeAccount(request: APIRequestContext, condition: string, role: string): Promise<MadeAccount> {
	const email = `${markOf()}-${condition}@example.com`
	const name = `Maria Perez ${condition} ${markOf()}`
	const answer = await request.post('/api/users', { data: { email, name, password: credentials.password, role } })
	expect(answer.status()).toBe(201)
	const { id } = (await answer.json()) as { id: string }
	return { id, name, email }
}

/**
 * Returns every account as the admin API lists it.
 * @param request - The request context of the signed in administrator.
 * @returns The accounts.
 */
async function listedAccounts(request: APIRequestContext): Promise<ListedAccount[]> {
	const answer = await request.get('/api/users')
	expect(answer.ok()).toBe(true)
	return (await answer.json()) as ListedAccount[]
}

/**
 * Returns one account as the admin API lists it.
 * @param request - The request context of the signed in administrator.
 * @param email - The address of the account.
 * @returns The account, none when no account holds the address.
 */
async function listedAs(request: APIRequestContext, email: string): Promise<ListedAccount | undefined> {
	return (await listedAccounts(request)).find((account) => account.email === email)
}

/**
 * Opens the users list searched for the words.
 * @param page - The page to drive.
 * @param words - The words the search box holds.
 */
async function openSearched(page: Page, words: string) {
	await page.goto(`/admin/users?search=${encodeURIComponent(words)}`)
}

/**
 * Returns the row of the list showing an account, found by its address.
 * @param page - The page holding the list.
 * @param email - The address of the account.
 * @returns The row locator.
 */
function rowOf(page: Page, email: string): Locator {
	return page.getByRole('row').filter({ hasText: email })
}

/**
 * Returns the toast reading the words.
 * @param page - The page holding the toast.
 * @param words - The words of the toast.
 * @returns The toast locator.
 */
function toastOf(page: Page, words: string): Locator {
	return page.locator('.godmin-toast', { hasText: words })
}

/**
 * Picks a role in the open Change role dialog.
 * @param page - The page holding the dialog.
 * @param label - The label of the role.
 */
async function pickRole(page: Page, label: string) {
	await page.getByRole('dialog', { name: 'Change role' }).getByRole('combobox', { name: 'Role' }).click()
	await page.getByRole('option', { name: label, exact: true }).click()
}

test('narrows the list to the account a search names and keeps the search across a reload', async ({
	page,
	request,
}) => {
	const searched = await madeAccount(request, 'searched', 'author')
	await page.goto('/admin/users')
	const search = page.getByRole('searchbox', { name: 'Search users…', exact: true })

	await search.fill(searched.email)

	await expect(page).toHaveURL(/[?&]search=/)
	await expect(rowOf(page, searched.email)).toBeVisible()
	await expect(rowOf(page, credentials.email)).toHaveCount(0)
	await page.reload()
	await expect(search).toHaveValue(searched.email)
	await expect(rowOf(page, searched.email)).toBeVisible()
	await expect(rowOf(page, credentials.email)).toHaveCount(0)
})

test('changes the role of one account from its row menu and gives the focus back to that menu', async ({
	page,
	request,
}) => {
	const promoted = await madeAccount(request, 'promoted', 'author')
	await openSearched(page, promoted.email)
	const menu = rowOf(page, promoted.email).getByRole('button', { name: 'Actions' })

	await menu.click()
	await page.getByRole('menuitem', { name: 'Change role…', exact: true }).click()
	const dialog = page.getByRole('dialog', { name: 'Change role' })
	await expect(dialog.getByText(`Choose the role of ${promoted.name}.`)).toBeVisible()
	const role = dialog.getByRole('combobox', { name: 'Role' })
	await expect(role).toHaveText('Author')
	await role.click()
	await expect(page.getByRole('option')).toHaveText(['Member', 'Author', 'Editor', 'Administrator'])
	await page.getByRole('option', { name: 'Editor', exact: true }).click()
	await dialog.getByRole('button', { name: 'Change role', exact: true }).click()

	await expect(toastOf(page, `Role changed for "${promoted.name}".`)).toBeVisible()
	await expect(rowOf(page, promoted.email).getByRole('cell', { name: 'Editor', exact: true })).toBeVisible()
	await expect(menu).toBeFocused()
	expect((await listedAs(request, promoted.email))?.role).toBe('editor')
})

test('opens Change role from a click on the name and gives the focus back to the name once cancelled', async ({
	page,
	request,
}) => {
	const named = await madeAccount(request, 'named', 'member')
	await openSearched(page, named.email)
	const name = rowOf(page, named.email).locator('.dataviews-title-field')

	await name.click()
	const dialog = page.getByRole('dialog', { name: 'Change role' })
	await expect(dialog.getByText(`Choose the role of ${named.name}.`)).toBeVisible()
	await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()

	await expect(dialog).toHaveCount(0)
	await expect(name).toBeFocused()
	expect((await listedAs(request, named.email))?.role).toBe('member')
})

test('changes the role of two ticked accounts from the bulk bar and gives the focus back to the list', async ({
	page,
	request,
}) => {
	const first = await madeAccount(request, 'regrouped-first', 'member')
	const second = await madeAccount(request, 'regrouped-second', 'member')
	await openSearched(page, `${markOf()}-regrouped`)

	await page.getByRole('checkbox', { name: first.name, exact: true }).check()
	await page.getByRole('checkbox', { name: second.name, exact: true }).check()
	await page.getByRole('button', { name: 'Change role…', exact: true }).click()
	const dialog = page.getByRole('dialog', { name: 'Change role' })
	await expect(dialog.getByText('Choose the role of 2 users.')).toBeVisible()
	await expect(dialog.getByRole('combobox', { name: 'Role' })).toHaveText('Change role to…')
	await expect(dialog.getByRole('button', { name: 'Change role', exact: true })).toBeDisabled()
	await pickRole(page, 'Author')
	await dialog.getByRole('button', { name: 'Change role', exact: true }).click()

	await expect(toastOf(page, 'Changed roles.')).toBeVisible()
	for (const account of [first, second]) {
		await expect(rowOf(page, account.email).getByRole('cell', { name: 'Author', exact: true })).toBeVisible()
		expect((await listedAs(request, account.email))?.role).toBe('author')
	}
	await expect(page.getByRole('region', { name: 'Users' })).toBeFocused()
})

test('disables two ticked accounts at once, counts them in a toast and gives the focus back to the list', async ({
	page,
	request,
}) => {
	const first = await madeAccount(request, 'locked-first', 'member')
	const second = await madeAccount(request, 'locked-second', 'member')
	await openSearched(page, `${markOf()}-locked`)

	await page.getByRole('checkbox', { name: first.name, exact: true }).check()
	await page.getByRole('checkbox', { name: second.name, exact: true }).check()
	await expect(page.getByText('2 Items selected')).toBeVisible()
	await page.getByRole('button', { name: 'Disable', exact: true }).click()

	await expect(toastOf(page, '2 users disabled.')).toBeVisible()
	for (const account of [first, second]) {
		await expect(rowOf(page, account.email).getByText('Disabled', { exact: true })).toBeVisible()
		expect((await listedAs(request, account.email))?.disabled).toBe(true)
	}
	await expect(page.getByRole('region', { name: 'Users' })).toBeFocused()
})

test('says the reader may no longer edit users when it lost that right while Change role was open', async ({
	browser,
	request,
}) => {
	const lowered = await madeAccount(request, 'lowered', 'admin')
	const kept = await madeAccount(request, 'kept', 'author')
	const context = await browser.newContext({ baseURL, storageState: { cookies: [], origins: [] } })
	const page = await context.newPage()
	await page.goto('/admin/')
	await page.getByLabel('Email').fill(lowered.email)
	await page.getByLabel('Password').fill(credentials.password)
	await page.getByRole('button', { name: 'Log in' }).click()
	await expect(page.getByRole('heading', { name: 'Posts', level: 2 })).toBeVisible()
	await openSearched(page, kept.email)

	await rowOf(page, kept.email).getByRole('button', { name: 'Actions' }).click()
	await page.getByRole('menuitem', { name: 'Change role…', exact: true }).click()
	await pickRole(page, 'Editor')
	const demoted = await request.put(`/api/users/${lowered.id}/role`, { data: { role: 'editor' } })
	expect(demoted.ok()).toBe(true)
	await page.getByRole('dialog', { name: 'Change role' }).getByRole('button', { name: 'Change role', exact: true }).click()

	await expect(toastOf(page, 'Sorry, you are not allowed to edit users.')).toBeVisible({ timeout: 15_000 })
	await expect(page.getByText('Users could not be loaded.')).toBeVisible()
	expect((await listedAs(request, kept.email))?.role).toBe('author')
	await context.close()
})

test('refuses a role nobody declares', async ({ request }) => {
	const target = await madeAccount(request, 'undeclared', 'author')

	const answer = await request.put(`/api/users/${target.id}/role`, { data: { role: 'archivist' } })

	expect(answer.status()).toBe(422)
	const refusal = (await answer.json()) as { code: string, meta: { field: string } }
	expect(refusal.code).toBe('role_unknown')
	expect(refusal.meta.field).toBe('role')
	expect((await listedAs(request, target.email))?.role).toBe('author')
})

test('links the post count of an account to the posts list narrowed to the posts it wrote', async ({
	page,
	request,
}) => {
	const author = await listedAs(request, 'author@example.com')
	const answer = await request.get('/api/users/post-counts')
	expect(answer.ok()).toBe(true)
	const { counts } = (await answer.json()) as { counts: Record<string, number> }
	const written = counts[author?.id ?? '']
	expect(written).toBeGreaterThan(1)
	await openSearched(page, 'author@example.com')
	const count = rowOf(page, 'author@example.com').getByRole('link', { name: `${written} posts by this author` })

	await expect(count).toHaveCSS('color', await brandLinkColorBeside(count))
	await count.click()

	await expect(page).toHaveURL(/\/admin\/content\/post\?filters=/)
	await expect(page.getByRole('link', { name: 'Keyboard Shortcuts Worth Learning' })).toBeVisible()
	await expect(page.getByRole('link', { name: 'Welcome to Gophenberg' })).toBeHidden()
})

test('adds a user from Add User, starting on the default role the settings name and not on the narrowest one', async ({
	page,
	request,
}) => {
	const settings = await request.get('/api/settings')
	expect(settings.ok()).toBe(true)
	const { default_role: defaultRole } = (await settings.json()) as { default_role: string }
	expect(defaultRole).not.toBe(NARROWEST_ROLE)
	const email = `${markOf()}-added@example.com`
	await page.goto('/admin/users')

	await page.getByRole('main').getByRole('link', { name: 'Add User', exact: true }).click()
	await expect(page).toHaveURL(/\/admin\/users\/new$/)
	await expect(page.getByRole('combobox', { name: 'Role' })).toHaveText(ROLE_LABELS[defaultRole])
	await expect(page.getByLabel('Password', { exact: true })).toHaveValue(/^.{24}$/)
	await page.getByRole('textbox', { name: /^Email/ }).fill(email)
	await page.getByRole('textbox', { name: /^Name/ }).fill(`Maria Perez added ${markOf()}`)
	await page.getByRole('button', { name: 'Add User', exact: true }).click()

	await expect(page).toHaveURL(/\/admin\/users$/)
	await expect(toastOf(page, 'New user created.')).toBeVisible()
	expect((await listedAs(request, email))?.role).toBe(defaultRole)
})

test.describe('on a phone', () => {
	test.use({ viewport: PHONE, isMobile: true, hasTouch: true })

	test('lists one account per row, opens Change role from a tap and gives the focus back to the tapped row', async ({
		page,
		request,
	}) => {
		const tapped = await madeAccount(request, 'tapped', 'author')
		await openSearched(page, tapped.email)
		const row = page.getByRole('button', { name: tapped.name, exact: true })

		await expect(page.locator('.dataviews-view-list')).toBeVisible()
		await row.tap()
		const dialog = page.getByRole('dialog', { name: 'Change role' })
		await expect(dialog.getByText(`Choose the role of ${tapped.name}.`)).toBeVisible()
		await pickRole(page, 'Editor')
		await dialog.getByRole('button', { name: 'Change role', exact: true }).tap()

		await expect(toastOf(page, `Role changed for "${tapped.name}".`)).toBeVisible()
		await expect(row).toBeFocused()
		expect((await listedAs(request, tapped.email))?.role).toBe('editor')
	})
})
