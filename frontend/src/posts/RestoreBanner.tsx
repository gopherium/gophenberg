// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { fetchAutosave } from './api'
import type { Autosave, PostDetail } from './api'

const MESSAGE = 'The browser kept a newer version of this post.'

/**
 * Returns the autosave worth offering over a stored post.
 * @param kept - The autosave the server holds, or nothing.
 * @param stored - The post as the server last reported it.
 * @returns The autosave to offer, or nothing.
 */
function offerable(kept: Autosave | null | undefined, stored: PostDetail): Autosave | null {
	if (kept === null || kept === undefined) {
		return null
	}
	return kept.savedAt > stored.updatedAt ? kept : null
}

/**
 * Renders the offer to take back words the browser kept.
 * @param props - The post being edited and the handler taking the words.
 * @returns The offer, or nothing when there is none worth making.
 */
export function RestoreBanner({
	postId,
	stored,
	onRestore,
}: {
	postId: string
	stored: PostDetail
	onRestore: (kept: Autosave) => void
}) {
	const [taken, setTaken] = useState(false)
	const kept = useQuery({
		queryKey: ['post-autosave', postId],
		queryFn: () => fetchAutosave(postId),
	})
	const offer = offerable(kept.data, stored)
	if (offer === null || taken) {
		return null
	}
	return (
		<Notice.Root intent="warning" spokenMessage={MESSAGE}>
			<Notice.Description>{MESSAGE}</Notice.Description>
			<Notice.Actions>
				<Notice.ActionButton
					onClick={() => {
						setTaken(true)
						onRestore(offer)
					}}
				>
					Restore
				</Notice.ActionButton>
			</Notice.Actions>
		</Notice.Root>
	)
}
