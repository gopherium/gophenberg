// SPDX-License-Identifier: Apache-2.0

import {
	Button,
	IconButton,
	SelectControl,
	Stack,
	Text,
	chevronLeft,
	cloudUpload,
	drawerLeft,
	drawerRight,
	listView,
	redo,
	undo,
} from '@gophenberg/frontend-sdk'
import { Inserter } from '@gophenberg/frontend-sdk/editor'
import { __, _x, isRTL, sprintf } from '@wordpress/i18n'
import { Link } from '@tanstack/react-router'
import type { Ref } from 'react'

import { DocumentBar } from './DocumentBar'
import { PREVIEW_WIDTHS, previewItem } from './editorSetup'
import type { PreviewWidth } from './editorSetup'
import { pluralOf } from './typeText'
import type { EditorBuffer } from './useEditorBuffer'
import { useContentType } from './useContentType'
import type { EditorViews } from './useEditorViews'
import type { EditorWidth } from './useEditorWidth'

/**
 * Returns the word the header shows for the state of the buffer.
 * @param dirty - Whether the buffer holds unsaved changes.
 * @param saving - Whether a write is in flight.
 * @returns The state to show.
 */
function stateOf(dirty: boolean, saving: boolean): string {
	if (saving) {
		return __('Saving', 'gophenberg')
	}
	return dirty ? __('Unsaved changes', 'gophenberg') : __('Saved', 'gophenberg')
}

/** The class WordPress's components pulse a saving control with, at rest under reduced motion. */
const SAVING_PULSE = 'components-animate__loading'

/**
 * Returns the name the save control carries below the editor breakpoint, as WordPress names its save button.
 * @param dirty - Whether the buffer holds unsaved changes.
 * @param saving - Whether a write is in flight.
 * @returns The name to give the control.
 */
function saveStateName(dirty: boolean, saving: boolean): string {
	if (saving) {
		return __('Saving', 'gophenberg')
	}
	return dirty ? __('Save draft', 'gophenberg') : __('Saved', 'gophenberg')
}

/**
 * Returns the preview width a select change asks for.
 * @param item - The item the select reported, or nothing.
 * @param current - The width the canvas renders at.
 * @returns The width to render at.
 */
export function chosenPreview(
	item: { value: string | null } | null,
	current: PreviewWidth,
): PreviewWidth {
	if (item === null || item.value === null) {
		return current
	}
	return item.value as PreviewWidth
}

/**
 * Returns the primary write control the buffer's state calls for.
 * @param buffer - The buffer the header drives.
 * @returns The label of the control and the write it runs.
 */
function primaryControl(buffer: EditorBuffer): { label: string, write: () => void } {
	if (buffer.savedStatus === 'published') {
		return { label: __('Update', 'gophenberg'), write: buffer.save }
	}
	return { label: __('Publish', 'gophenberg'), write: buffer.publish }
}

/**
 * Renders the undo and redo buttons.
 * @param props - The buffer whose history they walk.
 * @returns The two buttons.
 */
function HistoryButtons({ buffer }: { buffer: EditorBuffer }) {
	return (
		<>
			<IconButton
				label={__('Undo', 'gophenberg')}
				icon={undo}
				variant="minimal"
				tone="neutral"
				disabled={!buffer.hasUndo}
				onClick={buffer.undo}
			/>
			<IconButton
				label={__('Redo', 'gophenberg')}
				icon={redo}
				variant="minimal"
				tone="neutral"
				disabled={!buffer.hasRedo}
				onClick={buffer.redo}
			/>
		</>
	)
}

/**
 * Renders the word naming the state of the buffer and the select choosing the preview width.
 * @param props - The buffer whose state it names and the views holding the preview width.
 * @returns The word and the select.
 */
function StateAndPreview({ buffer, views }: { buffer: EditorBuffer, views: EditorViews }) {
	return (
		<>
			<Text>{stateOf(buffer.dirty, buffer.saving)}</Text>
			<SelectControl
				label={__('Preview', 'gophenberg')}
				hideLabelFromVision
				size="compact"
				items={PREVIEW_WIDTHS}
				value={previewItem(views.preview)}
				onValueChange={(item) => views.setPreview(chosenPreview(item, views.preview))}
			/>
		</>
	)
}

/**
 * Renders the control that saves a draft, named after the action when wide and after the state it shows below.
 * @param props - The buffer it saves and the window it sits in.
 * @returns The save control, in words or, below 600 pixels, as an icon.
 */
function SaveDraft({ buffer, width }: { buffer: EditorBuffer, width: EditorWidth }) {
	const disabled = !buffer.dirty || buffer.saving
	if (!width.narrow) {
		return (
			<Button variant="outline" disabled={disabled} loading={buffer.saving} onClick={buffer.save}>
				{__('Save draft', 'gophenberg')}
			</Button>
		)
	}
	const name = saveStateName(buffer.dirty, buffer.saving)
	const pulse = buffer.saving ? SAVING_PULSE : undefined
	if (width.iconSave) {
		return (
			<IconButton
				label={name}
				icon={cloudUpload}
				variant="minimal"
				disabled={disabled}
				className={pulse}
				onClick={buffer.save}
			/>
		)
	}
	return (
		<Button variant="outline" disabled={disabled} className={pulse} onClick={buffer.save}>
			{name}
		</Button>
	)
}

/**
 * Renders the button that shows and hides the settings sidebar.
 * @param props - The views it toggles and the reference the sidebar hands focus back through.
 * @returns The settings button.
 */
function SettingsToggle({ views, settingsRef }: { views: EditorViews, settingsRef: Ref<HTMLButtonElement> }) {
	return (
		<IconButton
			ref={settingsRef}
			label={_x('Settings', 'panel button label', 'gophenberg')}
			icon={isRTL() ? drawerLeft : drawerRight}
			variant="minimal"
			tone="neutral"
			aria-pressed={views.sidebarOpen}
			aria-expanded={views.sidebarOpen}
			aria-controls={views.sidebarId}
			onClick={views.toggleSidebar}
		/>
	)
}

/**
 * Renders the editor header with its navigation, history and write controls.
 * @param props - The buffer and views it drives, the type of post, the window and the settings reference.
 * @returns The header element.
 */
export function EditorHeader(
	{ buffer, type, views, width, settingsRef }: {
		buffer: EditorBuffer,
		type: string,
		views: EditorViews,
		width: EditorWidth,
		settingsRef: Ref<HTMLButtonElement>,
	},
) {
	const primary = primaryControl(buffer)
	const listed = useContentType()
	const draftable = buffer.savedStatus !== 'published'
	return (
		<div className="gophenberg-editor__header">
			<Stack direction="row" gap="xs" align="center" className="gophenberg-editor__tools">
				<IconButton
					label={sprintf(__('Back to %(type)s', 'gophenberg'), { type: pluralOf(listed).toLowerCase() })}
					icon={chevronLeft}
					variant="minimal"
					tone="neutral"
					render={<Link to="/content/$typeKey" params={{ typeKey: type }} />}
				/>
				<Inserter position="bottom right" toggleProps={{ label: __('Add block', 'gophenberg') }} />
				{width.narrow ? null : <HistoryButtons buffer={buffer} />}
				<IconButton
					label={__('List view', 'gophenberg')}
					icon={listView}
					variant="minimal"
					tone="neutral"
					aria-pressed={views.listOpen}
					onClick={views.toggleList}
				/>
			</Stack>
			{width.noPill ? null : <DocumentBar title={buffer.title} type={type} short={width.shortPill} />}
			<Stack direction="row" gap="sm" align="center" className="gophenberg-editor__actions">
				{width.narrow ? null : <StateAndPreview buffer={buffer} views={views} />}
				{draftable && <SaveDraft buffer={buffer} width={width} />}
				<SettingsToggle views={views} settingsRef={settingsRef} />
				<Button loading={buffer.saving} onClick={primary.write}>
					{primary.label}
				</Button>
			</Stack>
		</div>
	)
}
