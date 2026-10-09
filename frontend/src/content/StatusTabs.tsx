// SPDX-License-Identifier: Apache-2.0

import { PageTab, PageTabs } from '@gopherium/godmin'
import { Link } from '@tanstack/react-router'
import { __, _x } from '@wordpress/i18n'

/** The statuses a tab narrows a list to, in the order the tabs show them after All. */
const TAB_STATUSES = ['published', 'draft', 'pending', 'private', 'trash'] as const

/** A status a tab narrows a list to. */
export type TabStatus = (typeof TAB_STATUSES)[number]

/**
 * Keeps the status an address names when a tab narrows the list to it, so a stale one never reaches the server.
 * @param raw - The search the router parsed from the address.
 * @returns The status, undefined for the All tab so it overwrites any other value the address holds.
 */
export function statusSearch(raw: Record<string, unknown>): { status?: TabStatus } {
	return { status: TAB_STATUSES.find((held) => held === raw.status) }
}

/**
 * Returns the address search a tab leads to, the page and the search dropped and every other choice kept.
 * @param held - The search the address holds.
 * @param status - The status the tab narrows the list to, none for All.
 * @returns The search of the tab.
 */
function tabSearch(held: Record<string, unknown>, status: TabStatus | undefined): Record<string, unknown> {
	const kept = Object.fromEntries(Object.entries(held).filter(([key]) => !['page', 'search', 'status'].includes(key)))
	return status === undefined ? kept : { ...kept, status }
}

/**
 * Returns the tabs of a list, each label read fresh so the loaded catalogue answers.
 * @returns The status each tab narrows the list to, none for All, beside its label.
 */
function tabs(): { status: TabStatus | undefined, label: string }[] {
	return [
		{ status: undefined, label: _x('All', 'status filter', 'gophenberg') },
		{ status: 'published', label: _x('Published', 'post status', 'gophenberg') },
		{ status: 'draft', label: _x('Draft', 'post status', 'gophenberg') },
		{ status: 'pending', label: __('Pending', 'gophenberg') },
		{ status: 'private', label: __('Private', 'gophenberg') },
		{ status: 'trash', label: _x('Trash', 'post status', 'gophenberg') },
	]
}

/**
 * Renders the tabs narrowing a list by status, marking the one on screen.
 * @param props - The status the list shows, none for All.
 * @returns The tab navigation.
 */
export function StatusTabs({ current }: { current: TabStatus | undefined }) {
	return (
		<PageTabs label={__('Filter by status', 'gophenberg')}>
			{tabs().map((tab) => (
				<PageTab
					key={tab.label}
					current={tab.status === current}
					render={
						<Link
							to="."
							search={(held: Record<string, unknown>) => tabSearch(held, tab.status)}
							activeOptions={{ exact: true }}
						/>
					}
				>
					{tab.label}
				</PageTab>
			))}
		</PageTabs>
	)
}
