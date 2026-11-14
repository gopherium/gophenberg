// SPDX-License-Identifier: Apache-2.0

import { LoadingScreen, useToaster } from '@gopherium/godmin'
import { InputControl, Notice, sessionMayChange } from '@gophenberg/frontend-sdk'
import {
	BlockBreadcrumb,
	BlockCanvas,
	BlockEditorProvider,
	BlockList,
	BlockToolbar,
	CANVAS_STYLES,
	ListView,
	ShortcutProvider,
	SlotFillProvider,
	installApiFetchGuard,
	registerCuratedBlocks,
	registerMediaCategories,
	registerMediaLibrary,
	useBlockNotices,
} from '@gophenberg/frontend-sdk/editor'
import { useSession } from '@gopherium/react-auth'
import { __ } from '@wordpress/i18n'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useParams } from '@tanstack/react-router'
import { useMemo, useRef } from 'react'

import './editor.css'

import { Layout } from '../Layout'
import { MEDIA_CATEGORIES } from '../media/inserterCategories'
import { MediaLibraryPicker } from '../media/MediaLibraryPicker'
import { PickerViewProvider } from '../media/pickerView'
import { fetchPost } from './api'
import type { PostDetail } from './api'
import { PostReadView } from './PostReadView'
import { writesSettled } from './postWrites'
import { CANVAS_ROOT, canvasClass, editorClass, editorSettings, sidebarClass } from './editorSetup'
import { EditorHeader } from './EditorHeader'
import { EditorSidebar } from './EditorSidebar'
import { RestoreBanner } from './RestoreBanner'
import { singularOf } from './typeText'
import { useAutosave } from './useAutosave'
import { useContentType } from './useContentType'
import { useEditorBuffer } from './useEditorBuffer'
import { useEditorViews } from './useEditorViews'
import { useEditorWidth } from './useEditorWidth'

registerCuratedBlocks()
installApiFetchGuard()
registerMediaLibrary(MediaLibraryPicker)
registerMediaCategories(MEDIA_CATEGORIES)

/**
 * Renders the post editor.
 * @returns The editor screen element.
 */
export function EditorScreen() {
	const { postId } = useParams({ from: '/content/$typeKey/$postId/edit' })
	const session = useSession().data
	const client = useQueryClient()
	const post = useQuery({
		queryKey: ['post', postId],
		queryFn: () => writesSettled(client, postId).then(() => fetchPost(postId)),
	})
	if (post.isError) {
		return (
			<Layout>
				<Notice.Root intent="error" role="alert">
					<Notice.Description>{__('Could not load that post.', 'gophenberg')}</Notice.Description>
				</Notice.Root>
			</Layout>
		)
	}
	if (post.data === undefined) {
		return <LoadingScreen label={__('Loading the post.', 'gophenberg')} />
	}
	const mine = sessionMayChange(session, post.data.authorId)
	if (!mine || post.data.status === 'trash') {
		return (
			<Layout>
				<PostReadView key={postId} stored={post.data} mine={mine} />
			</Layout>
		)
	}
	return <Editor key={postId} postId={postId} stored={post.data} />
}

/**
 * Renders the editor over a post that has loaded.
 * @param props - The post to edit and its id.
 * @returns The editor element.
 */
function Editor({ postId, stored }: { postId: string, stored: PostDetail }) {
	const listed = useContentType()
	const buffer = useEditorBuffer(postId, stored, listed.fields)
	const width = useEditorWidth()
	const views = useEditorViews(width.narrow)
	const settingsRef = useRef<HTMLButtonElement>(null)
	const settings = useMemo(() => editorSettings(width.narrow), [width.narrow])
	const toaster = useToaster()
	useBlockNotices(toaster.show)
	const parkNow = useAutosave(postId, buffer)
	const closeSettings = () => {
		const toggle = settingsRef.current as HTMLButtonElement
		views.closeSidebar()
		toggle.focus()
	}
	return (
		<PickerViewProvider>
			<SlotFillProvider>
				<ShortcutProvider>
					<BlockEditorProvider
						value={buffer.blocks}
						onInput={buffer.onInput}
						onChange={buffer.onChange}
						settings={settings}
					>
						<div className={editorClass(width.narrow)}>
							<EditorHeader
								buffer={buffer}
								type={stored.type}
								views={views}
								width={width}
								settingsRef={settingsRef}
							/>
							<div className="gophenberg-editor__main">
								{views.listOpen ? (
									<div className="gophenberg-editor__outline">
										<ListView focusOnMount />
									</div>
								) : null}
								<div className="gophenberg-editor__content">
									<RestoreBanner
										postId={postId}
										stored={stored}
										onRestore={buffer.restore}
									/>
									{width.narrow ? <BlockToolbar hideDragHandle showPlaceholder /> : null}
									<div className="gophenberg-editor__title">
										<InputControl
											label={__('Title', 'gophenberg')}
											placeholder={__('Add title', 'gophenberg')}
											value={buffer.title}
											onValueChange={buffer.setTitle}
										/>
									</div>
									<div className={canvasClass(views.preview)}>
										<BlockCanvas height="100%" styles={CANVAS_STYLES}>
											<BlockList {...CANVAS_ROOT} />
										</BlockCanvas>
									</div>
								</div>
								<div className={sidebarClass(views.sidebarMoved)} id={views.sidebarId} hidden={!views.sidebarOpen}>
									<EditorSidebar
										postId={postId}
										buffer={buffer}
										parkNow={parkNow}
										onClose={closeSettings}
									/>
								</div>
							</div>
							{width.narrow ? null : (
								<div className="gophenberg-editor__foot">
									<BlockBreadcrumb rootLabelText={singularOf(listed)} />
								</div>
							)}
						</div>
					</BlockEditorProvider>
				</ShortcutProvider>
			</SlotFillProvider>
		</PickerViewProvider>
	)
}
