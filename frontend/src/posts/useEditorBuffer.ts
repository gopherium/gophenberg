// SPDX-License-Identifier: Apache-2.0

import { useSnackbar } from '@gophenberg/frontend-sdk'
import { parse, serialize } from '@gophenberg/frontend-sdk/editor'
import type { Block } from '@gophenberg/frontend-sdk/editor'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { savePost } from './api'
import type { PostDetail, SaveOutcome } from './api'

export interface EditorBuffer {
	title: string
	blocks: Block[]
	dirty: boolean
	saving: boolean
	setTitle: (title: string) => void
	setBlocks: (blocks: Block[]) => void
	save: () => void
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
	const [blocks, setBlocks] = useState<Block[]>(() => parse(stored.content))
	const [saved, setSaved] = useState({ title: stored.title, content: stored.content })
	const content = useMemo(() => serialize(blocks), [blocks])
	const save = useMutation({
		mutationFn: () => savePost(postId, { title, content }),
		onSuccess: async (outcome) => {
			snackbar.show(reportOf(outcome))
			if (outcome.kind === 'saved') {
				setSaved({ title, content })
				await client.invalidateQueries({ queryKey: ['posts'] })
			}
		},
		onError: () => snackbar.show('Could not save that post.'),
	})
	return {
		title,
		blocks,
		dirty: title !== saved.title || content !== saved.content,
		saving: save.isPending,
		setTitle,
		setBlocks,
		save: save.mutate,
	}
}

/**
 * Returns the message an outcome is announced with.
 * @param outcome - The outcome the server produced.
 * @returns The message to announce.
 */
function reportOf(outcome: SaveOutcome): string {
	if (outcome.kind === 'conflict') {
		return 'This post changed elsewhere. Reload before saving again.'
	}
	if (outcome.kind === 'rejected') {
		return outcome.message
	}
	return 'Draft saved.'
}
