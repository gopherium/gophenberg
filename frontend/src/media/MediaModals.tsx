// SPDX-License-Identifier: Apache-2.0

import { InputControl } from '@gophenberg/frontend-sdk'
import { DataForm } from '@gophenberg/frontend-sdk/dataviews'
import type { Form, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { RunConfirm, markLatest } from '@gopherium/godmin'
import type { BulkRun, BulkWords, Trigger, UnsavedText } from '@gopherium/godmin'
import { FormDialog } from '@gopherium/godmin/dialogs'
import type { QueryClient } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { useMemo, useState } from 'react'

import { holdWrite, writesSettled } from '../content/postWrites'
import { useRenameSend } from '../lists/RenameModal'
import { deleteQuestion, deleteWords } from '../lists/itemWords'
import { deleteMedia, editMedia } from './api'
import type { MediaDescriptions, MediaItem } from './api'
import { mediaFields } from './fields'
import { FieldsPane } from './FieldsPane'
import { saveFailure, titleOf } from './format'

/** What the actions of the media library need from the screen around them. */
export interface MediaTools {
	/** The query client the writes run on. */
	client: QueryClient
	/** Runs a call over files, the list reloading once the last call ended. */
	run: BulkRun<MediaItem>
	/** The fields failed edits left unsaved. */
	edits: UnsavedText<MediaDescriptions>
	/** The names failed renames left unsaved, the store the latest write of each file is marked on. */
	renames: UnsavedText
}

/** What a row dialog of the media library takes beside the props DataViews hands a modal. */
type MediaModalProps = RenderModalProps<MediaItem> & { tools: MediaTools }

/** A dialog the screen holds open on one file, so a card moving to another row of the grid never takes it away. */
export interface OpenedDialog {
	/** The action the dialog belongs to. */
	action: 'edit' | 'rename'
	/** The file the dialog is open on. */
	item: MediaItem
	/** The trigger the dialog opened from, which takes the focus back once the dialog is done. */
	trigger: Trigger
}

/** What a dialog the screen holds open on one file takes. */
interface MediaDialogProps {
	/** The file. */
	item: MediaItem
	/** What the dialog needs from the screen. */
	tools: MediaTools
	/** Takes the dialog away. */
	onClose: () => void
	/** Puts a focus the list lost while it read the library again back on the trigger the dialog opened from. */
	onSettled: () => void
}

/** The layout of a field a reader writes in, its label on top, as WordPress lays out the attachment form. */
const TOP_LABEL = { type: 'regular', labelPosition: 'top' } as const

/** The attachment form of WordPress: the four fields a reader writes in, then the rows a reader only reads. */
const EDIT_FORM: Form = {
	layout: { type: 'panel' },
	fields: [
		{ id: 'title', layout: TOP_LABEL },
		{ id: 'altText', layout: TOP_LABEL },
		{ id: 'caption', layout: TOP_LABEL },
		{ id: 'description', layout: TOP_LABEL },
		'date',
		'author',
		'filename',
		'mimeType',
		'filesize',
		'dimensions',
	],
}

/**
 * Sends a write to a file once every earlier write to it has answered, holding it until it answers in turn.
 * @param client - The query client the writes belong to.
 * @param id - The file the write goes to.
 * @param write - Sends the write.
 * @returns The write.
 */
function inTurn<T>(client: QueryClient, id: number, write: () => Promise<T>): Promise<T> {
	const slot = `media ${id}`
	return holdWrite(client, slot, writesSettled(client, slot).then(write))
}

/**
 * Edits a file once every earlier write to it has answered, so its version is read after them.
 * @param client - The query client the writes belong to.
 * @param item - The file to edit.
 * @param edits - The descriptions the reader changed.
 * @param unreached - The words to throw when no answer came.
 * @returns The stored file.
 */
function editInTurn(client: QueryClient, item: MediaItem, edits: MediaDescriptions, unreached: string) {
	return inTurn(client, item.id, () => editMedia(item.id, edits, unreached))
}

/**
 * Drops the title a failed edit of a file kept, leaving the other fields it kept.
 * @param edits - The fields failed edits left unsaved.
 * @param id - The file a rename now goes to.
 */
function dropKeptTitle(edits: UnsavedText<MediaDescriptions>, id: string) {
	const kept = edits.get(id)
	if (kept !== undefined) {
		const others = { ...kept }
		delete others.title
		edits.set(id, others)
	}
}

/**
 * Returns the words an edit of a file reads as.
 * @returns The words for the edit that landed and for the one that failed, naming what kind of file it was.
 */
function editWords(): BulkWords<MediaItem> {
	return {
		done: () => __('Media file updated.', 'gophenberg'),
		failed: ([{ item, error }]) => saveFailure(item, (error as Error).message),
	}
}

/**
 * Returns the form editing a file and its send, which closes the dialog at once, as WordPress closes a row edit.
 * @param item - The file.
 * @param tools - What the dialog needs from the screen.
 * @param close - Takes the dialog away.
 * @param settled - Puts a focus the list lost while it read the library again back on the trigger Edit opened from.
 * @returns The props of the form and the send.
 */
function useEditMedia(item: MediaItem, tools: MediaTools, close: () => void, settled: () => void) {
	const id = String(item.id)
	const fields = useMemo(() => mediaFields(), [])
	const [typed, setTyped] = useState<MediaDescriptions>(() => tools.edits.get(id) ?? {})
	const save = () => {
		close()
		tools.edits.delete(id)
		if (typed.title !== undefined) {
			tools.renames.delete(id)
		}
		const stillLatest = markLatest(tools.renames, id)
		const unreached = __('The server could not be reached, so nothing was saved.', 'gophenberg')
		const write = () =>
			editInTurn(tools.client, item, typed, unreached).catch((failure: unknown) => {
				if (stillLatest()) {
					tools.edits.set(id, typed)
				}
				throw failure
			})
		void tools.run([item], write, editWords()).then(settled)
	}
	const onChange = (changed: MediaDescriptions) => setTyped((now) => ({ ...now, ...changed }))
	return { form: { data: { ...item, ...typed }, fields, form: EDIT_FORM, onChange }, save }
}

/**
 * Renders Edit on a file, closed the moment Save is pressed, its fields scrolling in a pane above Cancel and Save as
 * WordPress's media editor scrolls them.
 * @param props - The file, what the dialog needs from the screen, what takes the dialog away and what gives the focus
 *   back once the write read the library again.
 * @returns The dialog.
 */
function EditMediaDialog({ item, tools, onClose, onSettled }: MediaDialogProps) {
	const edit = useEditMedia(item, tools, onClose, onSettled)
	return (
		<FormDialog
			title={__('Edit media', 'gophenberg')}
			size="medium"
			kind="edit"
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={edit.save}
			onClose={onClose}
		>
			<FieldsPane>
				<DataForm {...edit.form} />
			</FieldsPane>
		</FormDialog>
	)
}

/**
 * Renders the rename of a file, closed the moment Save is pressed, its write marked as the latest one to the file
 * across Edit and Rename.
 * @param props - The file, what the dialog needs from the screen and what takes the dialog away.
 * @returns The dialog.
 */
function RenameMediaDialog({ item, tools, onClose, onSettled }: MediaDialogProps) {
	const [name, setName] = useState(() => tools.renames.get(String(item.id)) ?? item.title)
	const unreached = __('The name could not be updated.', 'gophenberg')
	const send = useRenameSend(item, onClose, {
		idOf: (file) => String(file.id),
		rename: (file, typed) => {
			dropKeptTitle(tools.edits, String(file.id))
			return editInTurn(tools.client, file, { title: typed }, unreached)
		},
		run: tools.run,
		unsaved: tools.renames,
	})
	return (
		<FormDialog
			title={__('Rename', 'gophenberg')}
			size="medium"
			kind="edit"
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			canSubmit={name.trim() !== ''}
			onSubmit={() => void send(name).then(onSettled)}
			onClose={onClose}
		>
			<InputControl label={__('Name', 'gophenberg')} value={name} onValueChange={setName} />
		</FormDialog>
	)
}

/** The dialogs the screen holds open on one file, by the action they belong to. */
const OPENED_DIALOGS = { edit: EditMediaDialog, rename: RenameMediaDialog }

/**
 * Renders the dialog the screen holds open on one file, none while no dialog is open.
 * @param props - The dialog, its file and the trigger it opened from, what it needs from the screen, what takes it away
 *   and what puts a lost focus back on the trigger a dialog on a file opened from.
 * @returns The dialog, giving the focus back to the trigger it opened from once it closed and once its write read the
 *   library again.
 */
export function OpenMediaDialog({
	opened,
	tools,
	onClose,
	onBack,
}: {
	opened?: OpenedDialog
	onBack: (id: string, trigger: Trigger) => void
} & Omit<MediaDialogProps, 'item' | 'onSettled'>) {
	if (opened === undefined) {
		return null
	}
	const Dialog = OPENED_DIALOGS[opened.action]
	const back = () => onBack(String(opened.item.id), opened.trigger)
	const close = () => {
		onClose()
		back()
	}
	return (
		<Dialog
			key={`${opened.action} ${opened.item.id}`}
			item={opened.item}
			tools={tools}
			onClose={close}
			onSettled={back}
		/>
	)
}

/**
 * Renders the confirm deleting the picked files for good, closing only itself once the run ends.
 * @param props - The files, the handler closing the modal and what the action needs from the screen.
 * @returns The confirm.
 */
export function DeleteMediaModal({ items, closeModal, tools }: MediaModalProps) {
	return (
		<RunConfirm
			items={items}
			closeModal={closeModal}
			confirmLabel={__('Delete permanently', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			run={(picked) =>
				tools.run(picked, (item) => inTurn(tools.client, item.id, () => deleteMedia(item.id)), deleteWords(titleOf))
			}
		>
			{deleteQuestion(items, titleOf)}
		</RunConfirm>
	)
}
