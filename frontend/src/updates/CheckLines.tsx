// SPDX-License-Identifier: Apache-2.0

import { Link, Stack, Text, useToaster } from '@gophenberg/frontend-sdk'
import { ErrorNotice } from '@gopherium/godmin'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { __, sprintf } from '@wordpress/i18n'

import { checkForUpdates, updatesQueryKey } from './api'
import type { CheckResult, UpdatesOverview } from './api'
import { dateAndTime, timeUntil } from './words'

/**
 * Returns the sentence saying when the site last checked, and the words of the control that checks again.
 * @param checkedAt - When the last good check ran, or null before the first.
 * @returns The sentence and the words of the control.
 */
function lastCheckedWords(checkedAt: string | null): { sentence: string, action: string } {
	if (checkedAt === null) {
		return {
			sentence: __('Gophenberg has not checked for updates yet.', 'gophenberg'),
			action: __('Check now.', 'gophenberg'),
		}
	}
	return {
		sentence: sprintf(__('Last checked on %(date)s at %(time)s.', 'gophenberg'), dateAndTime(checkedAt)),
		action: __('Check again.', 'gophenberg'),
	}
}

/**
 * Returns the sentence saying when the next check runs, or that the site does not check on its own.
 * @param nextCheck - When the check job runs next, or null while the checks are off.
 * @returns The sentence.
 */
function nextCheckWords(nextCheck: string | null): string {
	if (nextCheck === null) {
		return __('This site does not check for updates on its own.', 'gophenberg')
	}
	return sprintf(__('The next check runs in %(time)s.', 'gophenberg'), { time: timeUntil(nextCheck, Date.now()) })
}

/**
 * Returns the line naming when the last check failed and why, or nothing while it did not.
 * @param overview - The overview of the updates.
 * @returns The line element, or null.
 */
function FailedCheck({ overview }: { overview: UpdatesOverview }) {
	if (overview.failed_at === null) {
		return null
	}
	const sentence = sprintf(__('The check on %(date)s at %(time)s failed: %(error)s', 'gophenberg'), {
		...dateAndTime(overview.failed_at),
		error: overview.error,
	})
	return <Text render={<p />}>{sentence}</Text>
}

/**
 * Reports whether a check that ran found nothing the overview did not already offer.
 * @param result - What the check found.
 * @param before - The overview the screen held when the check started.
 * @returns True when the check succeeded and found no newer update.
 */
function foundNothingNew(result: CheckResult, before: UpdatesOverview): boolean {
	return result.error === '' && result.offer?.version === before.offer?.version
}

/**
 * Returns the check of the update site that the screen runs on request, saying when it found nothing new.
 * @param before - The overview the screen holds.
 * @returns The mutation running the check.
 */
function useCheck(before: UpdatesOverview) {
	const client = useQueryClient()
	const toaster = useToaster()
	return useMutation({
		mutationFn: checkForUpdates,
		onSuccess: async (result) => {
			if (foundNothingNew(result, before)) {
				toaster.show(__('No new updates found.', 'gophenberg'))
			}
			await client.invalidateQueries({ queryKey: updatesQueryKey })
		},
	})
}

/**
 * Renders when the site last checked for updates with the control that checks again, and when it checks next.
 * @param props - The overview of the updates.
 * @returns The lines element.
 */
export function CheckLines({ overview }: { overview: UpdatesOverview }) {
	const check = useCheck(overview)
	const last = lastCheckedWords(overview.checked_at)
	return (
		<Stack direction="column" gap="sm">
			<Text render={<p />} className="gophenberg-updates__last-check">
				{last.sentence}{' '}
				<Link
					render={<button type="button" className="gophenberg-updates__check" disabled={check.isPending} />}
					onClick={() => check.mutate()}
				>
					{last.action}
				</Link>
			</Text>
			<Text render={<p />}>{nextCheckWords(overview.next_check)}</Text>
			<FailedCheck overview={overview} />
			{check.isError ? <ErrorNotice>{check.error.message}</ErrorNotice> : null}
		</Stack>
	)
}
