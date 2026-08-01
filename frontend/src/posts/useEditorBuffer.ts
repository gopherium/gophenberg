// SPDX-License-Identifier: Apache-2.0

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
	failure: string | null
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
	const [title, setTitle] = useState(stored.title)
	const [blocks, setBlocks] = useState<Block[]>(() => parse(stored.content))
	const [saved, setSaved] = useState({ title: stored.title, content: stored.content })
	const [failure, setFailure] = useState<string | null>(null)
	const content = useMemo(() => serialize(blocks), [blocks])
	const save = useMutation({
		mutationFn: () => savePost(postId, { title, content }),
		onSuccess: async (outcome) => {
			setFailure(failureOf(outcome))
			if (outcome.kind === 'saved') {
				setSaved({ title, content })
				await client.invalidateQueries({ queryKey: ['posts'] })
			}
		},
		onError: () => setFailure('Could not save that post.'),
	})
	return {
		title,
		blocks,
		dirty: title !== saved.title || content !== saved.content,
		saving: save.isPending,
		failure,
		setTitle,
		setBlocks,
		save: save.mutate,
	}
}

/**
 * Returns the message an outcome leaves on the screen.
 * @param outcome - The outcome the server produced.
 * @returns The message to report, or nothing when the post saved.
 */
function failureOf(outcome: SaveOutcome): string | null {
	if (outcome.kind === 'conflict') {
		return 'This post changed elsewhere. Reload before saving again.'
	}
	if (outcome.kind === 'rejected') {
		return outcome.message
	}
	return null
}
