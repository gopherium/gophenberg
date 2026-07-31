// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { useMutation } from '@tanstack/react-query'

import { useRefresh } from './actions'
import type { PostNotice, ReportNotice } from './actions'
import { restorePost } from './api'

/**
 * Renders the control taking back the trashing of a post.
 * @param props - The post to restore and the handler replacing the notice.
 * @returns The undo control.
 */
function Undo({ undoId, report }: { undoId: string, report: ReportNotice }) {
	const refresh = useRefresh()
	const undo = useMutation({
		mutationFn: () => restorePost(undoId),
		onSuccess: async () => {
			report(null)
			await refresh()
		},
		onError: () => report({ intent: 'error', message: 'Could not restore that post.' }),
	})
	return (
		<Notice.Actions>
			<Notice.ActionButton disabled={undo.isPending} onClick={() => undo.mutate()}>
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
			{notice.undoId !== undefined && <Undo undoId={notice.undoId} report={report} />}
		</Notice.Root>
	)
}
