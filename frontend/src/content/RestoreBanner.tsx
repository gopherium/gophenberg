// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { fetchAutosave } from './api'
import type { Autosave, PostDetail } from './api'

const MESSAGE = 'The browser kept an unsaved version of this post.'

/**
 * Reports whether the post already holds the kept words.
 * @param kept - The autosave the server holds.
 * @param stored - The post as the server last reported it.
 * @returns Whether the words match.
 */
function holdsKeptWords(kept: Autosave, stored: PostDetail): boolean {
	return (
		kept.title === stored.title &&
		kept.content === stored.content &&
		kept.excerpt === stored.excerpt &&
		holdsKeptValues(kept.fields, stored.fields)
	)
}

/**
 * Reports whether the kept values are the ones the post already holds.
 * @param kept - The values the browser parked.
 * @param stored - The values the post holds.
 * @returns True when nothing was kept back.
 */
function holdsKeptValues(kept: Record<string, unknown>, stored: Record<string, unknown>): boolean {
	const keys = Object.keys(kept)
	return keys.length === Object.keys(stored).length && keys.every((key) => kept[key] === stored[key])
}

/**
 * Returns the autosave worth offering over a stored post.
 * @param kept - The autosave the server holds, or nothing.
 * @param stored - The post as the server last reported it.
 * @returns The autosave to offer, or nothing.
 */
function offerable(kept: Autosave | null | undefined, stored: PostDetail): Autosave | null {
	if (kept === null || kept === undefined || holdsKeptWords(kept, stored)) {
		return null
	}
	return Date.parse(kept.savedAt) > Date.parse(stored.updatedAt) ? kept : null
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
