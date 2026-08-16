// SPDX-License-Identifier: Apache-2.0

import { Button, Skeleton, Stack } from '@gophenberg/frontend-sdk'
import { __, _x } from '@wordpress/i18n'

import type { PostCounts } from './api'

const GHOST_CHIPS = 5

/**
 * Renders chip ghosts holding the filter row's place until the counts arrive.
 * @returns The ghost row element.
 */
export function StatusGhost() {
	return (
		<Stack direction="row" gap="xs" align="center" aria-hidden="true">
			{Array.from({ length: GHOST_CHIPS }, (_, chip) => (
				<Skeleton key={chip} className="gophenberg-status-ghost" />
			))}
		</Stack>
	)
}

interface StatusView {
	status: string
	label: string
}

const LEADING: StatusView[] = [
	{ status: '', label: _x('All', 'status filter', 'gophenberg') },
	{ status: 'published', label: _x('Published', 'post status', 'gophenberg') },
	{ status: 'draft', label: _x('Draft', 'post status', 'gophenberg') },
	{ status: 'pending', label: __('Pending', 'gophenberg') },
]

const TRASH: StatusView = { status: 'trash', label: _x('Trash', 'post status', 'gophenberg') }

const PRIVATE: StatusView = { status: 'private', label: __('Private', 'gophenberg') }

/**
 * Returns how many posts a status view covers.
 * @param counts - The number of posts holding each status.
 * @param status - The status the view filters by, empty for every status.
 * @returns The number the view reports.
 */
function countFor(counts: PostCounts, status: string): number {
	if (status !== '') {
		return counts[status] ?? 0
	}
	return Object.values(counts).reduce((total, count) => total + count, 0)
}

/**
 * Returns the status views offered for the given counts.
 * @param counts - The number of posts holding each status.
 * @returns The views in the order they are shown.
 */
function viewsFor(counts: PostCounts): StatusView[] {
	const optional = countFor(counts, 'private') > 0 ? [PRIVATE] : []
	return [...LEADING, ...optional, TRASH]
}

/**
 * Renders the row of status filters above the posts list.
 * @param props - The counts, the status in force and the handler for a change.
 * @returns The status views row.
 */
export function StatusViews({
	counts,
	current,
	onSelect,
}: {
	counts: PostCounts
	current: string
	onSelect: (status: string) => void
}) {
	return (
		<Stack
			direction="row"
			gap="xs"
			align="center"
			role="group"
			aria-label={__('Filter by status', 'gophenberg')}
			className="godmin-arrival"
		>
			{viewsFor(counts).map((view) => (
				<Button
					key={view.status}
					variant="minimal"
					tone="neutral"
					size="compact"
					aria-pressed={view.status === current}
					onClick={() => onSelect(view.status)}
				>
					{view.label} ({countFor(counts, view.status)})
				</Button>
			))}
		</Stack>
	)
}
