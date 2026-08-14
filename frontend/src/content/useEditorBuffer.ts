// SPDX-License-Identifier: Apache-2.0

import { useToaster } from '@gopherium/godmin'
import { parse, serialize, useStateWithHistory } from '@gophenberg/frontend-sdk/editor'
import type { Block } from '@gophenberg/frontend-sdk/editor'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { savePost } from './api'
import type { PostChanges, PostDetail, SaveOutcome } from './api'

export interface EditorBuffer {
	title: string
	blocks: Block[]
	content: string
	status: string
	savedStatus: string
	slug: string
	parentId: string | null
	excerpt: string
	fields: Record<string, unknown>
	dirty: boolean
	saving: boolean
	version: string
	hasUndo: boolean
	hasRedo: boolean
	setTitle: (title: string) => void
	setStatus: (status: string) => void
	setSlug: (slug: string) => void
	setParentId: (parentId: string | null) => void
	setExcerpt: (excerpt: string) => void
	setFields: (fields: Record<string, unknown>) => void
	restore: (kept: { title: string, content: string, excerpt: string, fields: Record<string, unknown> }) => void
	onInput: (blocks: Block[]) => void
	onChange: (blocks: Block[]) => void
	undo: () => void
	redo: () => void
	save: () => void
	publish: () => void
	adoptVersion: (version: string) => void
}

/**
 * Returns the editing buffer held over a stored post.
 * @param postId - The post being edited.
 * @param stored - The post as the server last reported it.
 * @returns The buffer and the handlers changing it.
 */
export function useEditorBuffer(postId: string, stored: PostDetail): EditorBuffer {
	const client = useQueryClient()
	const toaster = useToaster()
	const [title, setTitle] = useState(stored.title)
	const [status, setStatus] = useState(stored.status)
	const [slug, setSlug] = useState(stored.slug)
	const [parentId, setParentId] = useState(stored.parentId)
	const [excerpt, setExcerpt] = useState(stored.excerpt)
	const [fields, setFields] = useState(stored.fields)
	const [version, setVersion] = useState(stored.updatedAt)
	const [saved, setSaved] = useState({
		title: stored.title,
		content: stored.content,
		slug: stored.slug,
		parentId: stored.parentId,
		excerpt: stored.excerpt,
		status: stored.status,
		fields: stored.fields,
	})
	const history = useStateWithHistory<Block[]>(parse(stored.content))
	const blocks = history.value as Block[]
	const content = useMemo(() => serialize(blocks), [blocks])
	const write = useMutation({
		mutationFn: (changes: PostChanges) => savePost(postId, changes, version),
		onSuccess: async (outcome, changes) => {
			toaster.show(reportOf(outcome, changes))
			if (outcome.kind === 'saved') {
				adopt(outcome.post)
				client.setQueryData(['post', postId], outcome.post)
				await Promise.all([
					client.invalidateQueries({ queryKey: ['posts'] }),
					client.invalidateQueries({ queryKey: ['post-counts'] }),
				])
			}
		},
		onError: () => toaster.show('Could not save that post.'),
	})
	/**
	 * Takes the post the server wrote as the new settled buffer.
	 * @param written - The post the server reported after the write.
	 */
	function adopt(written: PostDetail) {
		setSaved({
			title: written.title,
			content: written.content,
			slug: written.slug,
			parentId: written.parentId,
			excerpt: written.excerpt,
			status: written.status,
			fields: written.fields,
		})
		setStatus(written.status)
		setSlug(written.slug)
		setParentId(written.parentId)
		setExcerpt(written.excerpt)
		setFields(written.fields)
		setVersion(written.updatedAt)
	}
	return {
		title,
		blocks,
		content,
		status,
		savedStatus: saved.status,
		slug,
		parentId,
		excerpt,
		fields,
		dirty:
			title !== saved.title ||
			content !== saved.content ||
			slug !== saved.slug ||
			parentId !== saved.parentId ||
			excerpt !== saved.excerpt ||
			status !== saved.status ||
			!sameFields(fields, saved.fields),
		saving: write.isPending,
		version,
		hasUndo: history.hasUndo,
		hasRedo: history.hasRedo,
		setTitle,
		setStatus,
		setSlug,
		setParentId,
		setExcerpt,
		setFields,
		restore: (kept: {
			title: string
			content: string
			excerpt: string
			fields: Record<string, unknown>
		}) => {
			setTitle(kept.title)
			setExcerpt(kept.excerpt)
			setFields(kept.fields)
			history.setValue(parse(kept.content), false)
		},
		onInput: (next: Block[]) => history.setValue(next, true),
		onChange: (next: Block[]) => history.setValue(next, false),
		undo: history.undo,
		redo: history.redo,
		save: () => write.mutate({ title, content, slug, excerpt, status, parent_id: parentId, fields }),
		publish: () =>
			write.mutate({
				title,
				content,
				slug,
				excerpt,
				status: 'published',
				parent_id: parentId,
				fields,
			}),
		adoptVersion: setVersion,
	}
}

/**
 * Reports whether two sets of field values hold the same things.
 * @param held - The values the buffer holds.
 * @param saved - The values the server last reported.
 * @returns True when nothing moved.
 */
function sameFields(held: Record<string, unknown>, saved: Record<string, unknown>): boolean {
	const keys = Object.keys(held)
	return keys.length === Object.keys(saved).length && keys.every((key) => held[key] === saved[key])
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
