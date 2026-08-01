// SPDX-License-Identifier: Apache-2.0

import { useSnackbar } from '@gophenberg/frontend-sdk'
import { parse, serialize, useStateWithHistory } from '@gophenberg/frontend-sdk/editor'
import type { Block } from '@gophenberg/frontend-sdk/editor'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { savePost } from './api'
import type { PostChanges, PostDetail, SaveOutcome } from './api'

export interface EditorBuffer {
	title: string
	blocks: Block[]
	status: string
	dirty: boolean
	saving: boolean
	hasUndo: boolean
	hasRedo: boolean
	setTitle: (title: string) => void
	onInput: (blocks: Block[]) => void
	onChange: (blocks: Block[]) => void
	undo: () => void
	redo: () => void
	save: () => void
	publish: () => void
}

/**
 * Returns the editing buffer held over a stored post.
 * @param postId - The post being edited.
 * @param stored - The post as the server last reported it.
 * @returns The buffer and the handlers changing it.
 */
export function useEditorBuffer(postId: string, stored: PostDetail): EditorBuffer {
	const client = useQueryClient()
	const snackbar = useSnackbar()
	const [title, setTitle] = useState(stored.title)
	const [status, setStatus] = useState(stored.status)
	const [saved, setSaved] = useState({ title: stored.title, content: stored.content })
	const history = useStateWithHistory<Block[]>(parse(stored.content))
	const blocks = history.value as Block[]
	const content = useMemo(() => serialize(blocks), [blocks])
	const write = useMutation({
		mutationFn: (changes: PostChanges) => savePost(postId, changes),
		onSuccess: async (outcome, changes) => {
			snackbar.show(reportOf(outcome, changes))
			if (outcome.kind === 'saved') {
				setSaved({ title: outcome.post.title, content: outcome.post.content })
				setStatus(outcome.post.status)
				await client.invalidateQueries({ queryKey: ['posts'] })
			}
		},
		onError: () => snackbar.show('Could not save that post.'),
	})
	return {
		title,
		blocks,
		status,
		dirty: title !== saved.title || content !== saved.content,
		saving: write.isPending,
		hasUndo: history.hasUndo,
		hasRedo: history.hasRedo,
		setTitle,
		onInput: (next: Block[]) => history.setValue(next, true),
		onChange: (next: Block[]) => history.setValue(next, false),
		undo: history.undo,
		redo: history.redo,
		save: () => write.mutate({ title, content }),
		publish: () => write.mutate({ status: 'published' }),
	}
}

/**
 * Returns the message an outcome is announced with.
 * @param outcome - The outcome the server produced.
 * @param changes - The changes the write asked for.
 * @returns The message to announce.
 */
function reportOf(outcome: SaveOutcome, changes: PostChanges): string {
	if (outcome.kind === 'conflict') {
		return 'This post changed elsewhere. Reload before saving again.'
	}
	if (outcome.kind === 'rejected') {
		return outcome.message
	}
	return changes.status === 'published' ? 'Post published.' : 'Draft saved.'
}
