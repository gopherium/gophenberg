// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { useMutation } from '@tanstack/react-query'

import { useRefresh } from './actions'
import type { PostNotice, ReportNotice } from './actions'
import { restorePost } from './api'

/**
 * Renders the control taking back a trashing.
 * @param props - The posts to restore and the handler replacing the notice.
 * @returns The undo control.
 */
function Undo({ undoIds, report }: { undoIds: string[], report: ReportNotice }) {
	const refresh = useRefresh()
	const undo = useMutation({
		mutationFn: () => Promise.all(undoIds.map((id) => restorePost(id))),
		onSuccess: async () => {
			report(null)
			await refresh()
		},
		onError: () =>
			report({
				intent: 'error',
				message:
					undoIds.length === 1
						? 'Could not restore that post.'
						: 'Could not restore those posts.',
			}),
	})
	return (
		<Notice.Actions>
			<Notice.ActionButton loading={undo.isPending} onClick={() => undo.mutate()}>
				Undo
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
