// SPDX-License-Identifier: Apache-2.0

import { LoadingScreen } from '@gopherium/godmin'
import { InputControl, Notice } from '@gophenberg/frontend-sdk'
import {
	BlockBreadcrumb,
	BlockCanvas,
	BlockEditorProvider,
	BlockList,
	CANVAS_STYLES,
	ListView,
	ShortcutProvider,
	SlotFillProvider,
	registerCuratedBlocks,
} from '@gophenberg/frontend-sdk/editor'
import { useQuery } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'

import './editor.css'
import { fetchPost } from './api'
import type { PostDetail } from './api'
import { EDITOR_SETTINGS, canvasClass } from './editorSetup'
import { EditorHeader } from './EditorHeader'
import { EditorSidebar } from './EditorSidebar'
import { RestoreBanner } from './RestoreBanner'
import { useAutosave } from './useAutosave'
import { useEditorBuffer } from './useEditorBuffer'
import { useEditorViews } from './useEditorViews'

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
		return <LoadingScreen label="Loading the post." />
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
	const views = useEditorViews()
	useAutosave(postId, buffer)
	return (
		<SlotFillProvider>
			<ShortcutProvider>
				<BlockEditorProvider
					value={buffer.blocks}
					onInput={buffer.onInput}
					onChange={buffer.onChange}
					settings={EDITOR_SETTINGS}
				>
					<div className="gophenberg-editor">
						<EditorHeader buffer={buffer} type={stored.type} views={views} />
						<div className="gophenberg-editor__main">
							{views.listOpen ? (
								<div className="gophenberg-editor__outline">
									<ListView />
								</div>
							) : null}
							<div className="gophenberg-editor__content">
								<RestoreBanner
									postId={postId}
									stored={stored}
									onRestore={buffer.restore}
								/>
								<div className="gophenberg-editor__title">
									<InputControl
										label="Title"
										placeholder="Add title"
										value={buffer.title}
										onValueChange={buffer.setTitle}
									/>
								</div>
								<div className={canvasClass(views.preview)}>
									<BlockCanvas height="100%" styles={CANVAS_STYLES}>
										<BlockList />
									</BlockCanvas>
								</div>
							</div>
							<div className="gophenberg-editor__sidebar">
								<EditorSidebar postId={postId} buffer={buffer} />
							</div>
						</div>
						<div className="gophenberg-editor__foot">
							<BlockBreadcrumb rootLabelText="Post" />
						</div>
					</div>
				</BlockEditorProvider>
			</ShortcutProvider>
		</SlotFillProvider>
	)
}
