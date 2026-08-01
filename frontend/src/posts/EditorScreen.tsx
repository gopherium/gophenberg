// SPDX-License-Identifier: Apache-2.0

import { InputControl, Notice, Stack, Text } from '@gophenberg/frontend-sdk'
import {
	BlockCanvas,
	BlockEditorProvider,
	BlockList,
	BlockToolbar,
	CANVAS_STYLES,
	ShortcutProvider,
	SlotFillProvider,
	registerCuratedBlocks,
} from '@gophenberg/frontend-sdk/editor'
import { useQuery } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'

import { fetchPost } from './api'
import type { PostDetail } from './api'
import { EDITOR_SETTINGS } from './editorSetup'
import { EditorHeader } from './EditorHeader'
import { useEditorBuffer } from './useEditorBuffer'

registerCuratedBlocks()

/**
 * Renders the post editor.
 * @returns The editor screen element.
 */
export function EditorScreen() {
	const { postId } = useParams({ from: '/posts/$postId/edit' })
	const post = useQuery({ queryKey: ['post', postId], queryFn: () => fetchPost(postId) })
	if (post.isError) {
		return (
			<Notice.Root intent="error" role="alert">
				<Notice.Description>Could not load that post.</Notice.Description>
			</Notice.Root>
		)
	}
	if (post.data === undefined) {
		return <Text>Loading the post.</Text>
	}
	return <Editor postId={postId} stored={post.data} />
}

/**
 * Renders the editor over a post that has loaded.
 * @param props - The post to edit and its id.
 * @returns The editor element.
 */
function Editor({ postId, stored }: { postId: string, stored: PostDetail }) {
	const buffer = useEditorBuffer(postId, stored)
	return (
		<SlotFillProvider>
			<ShortcutProvider>
				<Stack direction="column" gap="sm">
					<EditorHeader buffer={buffer} />
					<InputControl
						label="Title"
						value={buffer.title}
						onValueChange={buffer.setTitle}
					/>
					<BlockEditorProvider
						value={buffer.blocks}
						onInput={buffer.onInput}
						onChange={buffer.onChange}
						settings={EDITOR_SETTINGS}
					>
						<BlockToolbar />
						<BlockCanvas height="70vh" styles={CANVAS_STYLES}>
							<BlockList />
						</BlockCanvas>
					</BlockEditorProvider>
				</Stack>
			</ShortcutProvider>
		</SlotFillProvider>
	)
}
