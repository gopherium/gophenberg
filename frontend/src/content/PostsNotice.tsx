// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { __, _n } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'

import { useRefresh } from './actions'
import type { PostNotice, ReportNotice } from './actions'
import { restorePost } from './api'

/**
 * Returns the notice reporting how many restores the server refused.
 * @param refused - How many posts were not restored.
 * @returns The notice to show.
 */
function refusal(refused: number): PostNotice {
	return {
		intent: 'error',
		message: _n('Could not restore that post.', 'Could not restore those posts.', refused, 'gophenberg'),
	}
}

/**
 * Renders the control taking back a trashing.
 * @param props - The posts to restore and the handler replacing the notice.
 * @returns The undo control.
 */
function Undo({ undoIds, report }: { undoIds: string[], report: ReportNotice }) {
	const refresh = useRefresh()
	const undo = useMutation({
		mutationFn: () => Promise.allSettled(undoIds.map((id) => restorePost(id))),
		onSuccess: async (outcomes) => {
			const refused = outcomes.filter((outcome) => outcome.status === 'rejected')
			report(refused.length === 0 ? null : refusal(refused.length))
			await refresh()
		},
	})
	return (
		<Notice.Actions>
			<Notice.ActionButton loading={undo.isPending} onClick={() => undo.mutate()}>
				{__('Undo', 'gophenberg')}
			</Notice.ActionButton>
		</Notice.Actions>
	)
}

/**
 * Renders the message left by the last action taken on a post.
 * @param props - The message to show and the handler replacing it.
 * @returns The notice.
 */
export function PostsNotice({ notice, report }: { notice: PostNotice, report: ReportNotice }) {
	return (
		<Notice.Root intent={notice.intent} spokenMessage={notice.message}>
			<Notice.Description>{notice.message}</Notice.Description>
			{notice.undoIds !== undefined && <Undo undoIds={notice.undoIds} report={report} />}
		</Notice.Root>
	)
}
