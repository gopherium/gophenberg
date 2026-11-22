// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'
import type { Locator, Page } from '@playwright/test'

const PHONE = { width: 390, height: 844 }

/** The feed as its manifest describes it, the one plugin the release binary holds. */
const FEED = {
	name: 'Feed',
	description: 'Serves your published posts as an RSS feed.',
	version: '0.0.1',
}

/**
 * Returns the tab of the list's status filter carrying the label, matched whole so no other word answers.
 * @param page - The page holding the list.
 * @param label - The label of the tab.
 * @returns The tab locator.
 */
function statusTab(page: Page, label: string): Locator {
	return page.getByRole('navigation', { name: 'Filter by status' }).getByRole('link', { name: label, exact: true })
}

/**
 * Returns the row of the plugins table showing a name in its title.
 * @param page - The page holding the list.
 * @param name - The name the row shows.
 * @returns The row locator.
 */
function rowOf(page: Page, name: string): Locator {
	return page.getByRole('row').filter({ has: page.locator('.dataviews-title-field', { hasText: name }) })
}

/**
 * Opens the Plugins screen from its row in the menu.
 * @param page - The page to drive.
 */
async function openFromMenu(page: Page) {
	await page.goto('/admin/')
	await page.getByRole('region', { name: 'Navigation' }).getByRole('link', { name: 'Plugins', exact: true }).click()
	await expect(page).toHaveURL(/\/admin\/plugins$/)
	await expect(page.getByRole('heading', { name: 'Plugins', level: 2 })).toBeVisible()
}

test('opens Plugins from the menu and lists the feed as active, with its description and version', async ({
	page,
}) => {
	await openFromMenu(page)
	const feed = rowOf(page, FEED.name)

	await expect(page.getByText('Every plugin this site holds, and whether it runs.')).toBeVisible()
	await expect(page.getByRole('columnheader')).toContainText(['Plugin', 'Description', 'Status', 'Version'])
	await expect(feed.getByRole('cell', { name: FEED.description, exact: true })).toBeVisible()
	await expect(feed.getByText('Active', { exact: true })).toBeVisible()
	await expect(feed.getByRole('cell', { name: FEED.version, exact: true })).toBeVisible()
})

test('says the Paused tab holds no plugin while the feed runs', async ({ page }) => {
	await openFromMenu(page)

	await statusTab(page, 'Paused').click()

	await expect(statusTab(page, 'Paused')).toHaveAttribute('aria-current', 'page')
	await expect(page).toHaveURL(/[?&]status=paused/)
	await expect(page.getByText('No paused plugins', { exact: true })).toBeVisible()
	await expect(rowOf(page, FEED.name)).toHaveCount(0)
})

test.describe('on a phone', () => {
	test.use({ viewport: PHONE, isMobile: true, hasTouch: true })

	test('lists the feed with its name, its description and its badge, and leaves its version out', async ({
		page,
	}) => {
		await page.goto('/admin/plugins')
		const list = page.locator('.dataviews-view-list')
		const feed = list.getByRole('row').filter({ has: page.locator('.dataviews-title-field', { hasText: FEED.name }) })

		await expect(list).toBeVisible()
		await expect(feed.getByText(FEED.description, { exact: true })).toBeVisible()
		await expect(feed.getByText('Active', { exact: true })).toBeVisible()
		await expect(feed.getByText(FEED.version)).toHaveCount(0)
	})
})
