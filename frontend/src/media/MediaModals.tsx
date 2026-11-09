// SPDX-License-Identifier: Apache-2.0

import { ValidatedInputControl, ValidatedTextareaControl } from '@gophenberg/frontend-sdk'
import { DataForm } from '@gophenberg/frontend-sdk/dataviews'
import type { DataFormControlProps, Field, Form } from '@gophenberg/frontend-sdk/dataviews'
import { ConfirmDialog, FormDialog, RenameDialog } from '@gopherium/godmin'
import type { BulkRun, BulkWords, DialogRun, Trigger } from '@gopherium/godmin'
import type { QueryClient } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { createContext, useContext, useMemo, useState } from 'react'

import { holdWrite, writesSettled } from '../content/postWrites'
import { deleteTitle, deleteWords, nameWords } from '../lists/itemWords'
import { deleteMedia, editMedia } from './api'
import type { MediaDescriptions, MediaItem } from './api'
import { mediaFields } from './fields'
import { fileDeleteDescription, saveFailure, titleOf } from './format'

/** What the actions of the media library need from the screen around them. */
export interface MediaTools {
	/** The query client the writes run on. */
	client: QueryClient
	/** Runs a call over files, the list reloading once the last call ended. */
	run: BulkRun<MediaItem>
	/** Runs the write of a dialog on one file and answers the dialog, the list reloading once the write ended. */
	write: DialogRun<MediaItem>
}

/** A dialog the screen holds open on one file, so a card moving to another row of the grid never takes it away. */
interface OpenedOnFile {
	/** The action the dialog belongs to. */
	action: 'edit' | 'rename'
	/** The file the dialog is open on. */
	item: MediaItem
	/** The trigger the dialog opened from, which takes the focus back once the dialog is done. */
	trigger: Trigger
}

/** The confirm the screen holds open on the picked files. */
interface OpenedDelete {
	/** The action the confirm belongs to. */
	action: 'delete'
	/** The files the confirm is open on. */
	items: MediaItem[]
	/** The trigger the confirm opened from, which takes the focus back once the confirm is done. */
	trigger: Trigger
}

/** A dialog the screen holds open on one file or on the picked files. */
export type OpenedDialog = OpenedOnFile | OpenedDelete

/** What a dialog the screen holds open on one file takes. */
interface MediaDialogProps {
	/** The file. */
	item: MediaItem
	/** What the dialog needs from the screen. */
	tools: MediaTools
	/** Takes the dialog away, once its card has left the page. */
	onClose: () => void
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
 * Returns the words an edit of a file reads as.
 * @returns The words for the edit that landed and for the one that failed, naming what kind of file it was.
 */
function editWords(): BulkWords<MediaItem> {
	return {
		done: () => __('Media file updated.', 'gophenberg'),
		failed: ([{ item, error }]) => saveFailure(item, (error as Error).message),
	}
}

/** Whether the edit of a file waits on its save, its boxes then read only. */
const Saving = createContext(false)

/**
 * Returns the props DataForm hands the box of a field, its help drawn under it as the details that hold a link.
 * @param props - The file, the field, the handler of a change and how the label shows.
 * @returns The props of the box.
 */
function boxProps({ data, field, onChange, hideLabelFromVision, markWhenOptional }: DataFormControlProps<MediaItem>) {
	const { label, placeholder, description, getValue, setValue, isValid } = field
	return {
		required: Boolean(isValid.required),
		markWhenOptional,
		label,
		placeholder,
		value: getValue({ item: data }),
		details: description,
		onValueChange: (value: string) => onChange(setValue({ item: data, value })),
		hideLabelFromVision,
		disabled: field.isDisabled({ item: data, field }),
	}
}

/**
 * Renders the one line box of a field, as DataForm draws it, read only while the save runs.
 * @param props - The file, the field, the handler of a change and how the label shows.
 * @returns The box.
 */
function HeldInput(props: DataFormControlProps<MediaItem>) {
	const saving = useContext(Saving)
	return <ValidatedInputControl {...boxProps(props)} readOnly={saving} />
}

/**
 * Returns the box of several lines of a field, as DataForm draws it, read only while the save runs.
 * @param rows - How many lines the box shows, the default of the box when absent.
 * @returns The box.
 */
function heldTextarea(rows: number | undefined) {
	/**
	 * Renders the box of several lines of a field, as DataForm draws it, read only while the save runs.
	 * @param props - The file, the field, the handler of a change and how the label shows.
	 * @returns The box.
	 */
	return function HeldTextarea(props: DataFormControlProps<MediaItem>) {
		const saving = useContext(Saving)
		return <ValidatedTextareaControl {...boxProps(props)} rows={rows} readOnly={saving} />
	}
}

/**
 * Returns a field of a file with its box swapped for one that turns read only while the save runs.
 * @param field - The field.
 * @returns The field, unchanged when a reader only reads it.
 */
function heldWhileSaving(field: Field<MediaItem>): Field<MediaItem> {
	if (field.readOnly === true || field.type !== 'text') {
		return field
	}
	const edit = field.Edit
	if (typeof edit === 'object' && edit.control === 'textarea') {
		return { ...field, Edit: heldTextarea(edit.rows) }
	}
	return { ...field, Edit: HeldInput }
}

/**
 * Returns the form editing a file and its send, which holds the dialog open until the write answers.
 * @param item - The file.
 * @param tools - What the dialog needs from the screen.
 * @returns The props of the form, whose boxes stay in place and turn read only while the write runs, and the send.
 */
function useEditMedia(item: MediaItem, tools: MediaTools) {
	const fields = useMemo(() => mediaFields().map(heldWhileSaving), [])
	const [typed, setTyped] = useState<MediaDescriptions>({})
	const save = () => {
		const unreached = __('The server could not be reached, so nothing was saved.', 'gophenberg')
		return tools.write([item], (file) => editInTurn(tools.client, file, typed, unreached), editWords())
	}
	const onChange = (changed: MediaDescriptions) => setTyped((now) => ({ ...now, ...changed }))
	const form = { data: { ...item, ...typed }, fields, form: EDIT_FORM, onChange }
	return { form, save }
}

/**
 * Renders Edit on a file, open until the write answers.
 * @param props - The file, what the dialog needs from the screen and what takes the dialog away.
 * @returns The dialog.
 */
function EditMediaDialog({ item, tools, onClose }: MediaDialogProps) {
	const edit = useEditMedia(item, tools)
	return (
		<FormDialog
			title={__('Edit media', 'gophenberg')}
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={edit.save}
			onClose={onClose}
		>
			{(busy) => (
				<Saving.Provider value={busy}>
					<DataForm {...edit.form} />
				</Saving.Provider>
			)}
		</FormDialog>
	)
}

/**
 * Renders the rename of a file, open until the write answers.
 * @param props - The file, what the dialog needs from the screen and what takes the dialog away.
 * @returns The dialog.
 */
function RenameMediaDialog({ item, tools, onClose }: MediaDialogProps) {
	const unreached = __('The name could not be updated.', 'gophenberg')
	return (
		<RenameDialog
			title={__('Rename', 'gophenberg')}
			name={item.title}
			fieldLabel={__('Name', 'gophenberg')}
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={(name) =>
				tools.write([item], (file) => editInTurn(tools.client, file, { title: name }, unreached), nameWords())
			}
			onClose={onClose}
		/>
	)
}

/** The dialogs the screen holds open on one file, by the action they belong to. */
const OPENED_DIALOGS = { edit: EditMediaDialog, rename: RenameMediaDialog }

/**
 * Renders the confirm deleting the picked files for good, busy until the run ends.
 * @param props - The files, what the confirm needs from the screen and what takes it away.
 * @returns The confirm.
 */
function DeleteMediaConfirm({ items, tools, onClose }: {
	items: MediaItem[]
	tools: MediaTools
	onClose: () => void
}) {
	return (
		<ConfirmDialog
			intent="irreversible"
			title={deleteTitle(items, titleOf)}
			description={fileDeleteDescription(items.length)}
			confirmLabel={__('Delete permanently', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onConfirm={() =>
				tools.run(items, (item) => inTurn(tools.client, item.id, () => deleteMedia(item.id)), deleteWords(titleOf))
			}
			onClose={onClose}
		/>
	)
}

/**
 * Renders the dialog the screen holds open on one file or on the picked files, none while no dialog is open.
 * @param props - The dialog, its files and the trigger it opened from, what it needs from the screen, what takes it
 *   away and what puts a lost focus back on the trigger it opened from.
 * @returns The dialog, giving the focus back to the trigger it opened from once it closed.
 */
export function OpenMediaDialog({
	opened,
	tools,
	onClose,
	onBack,
}: {
	opened?: OpenedDialog
	onBack: (id: string, trigger: Trigger) => void
} & Omit<MediaDialogProps, 'item'>) {
	if (opened === undefined) {
		return null
	}
	const [first] = opened.action === 'delete' ? opened.items : [opened.item]
	const close = () => {
		onClose()
		onBack(String(first.id), opened.trigger)
	}
	if (opened.action === 'delete') {
		return <DeleteMediaConfirm items={opened.items} tools={tools} onClose={close} />
	}
	const Dialog = OPENED_DIALOGS[opened.action]
	return <Dialog key={`${opened.action} ${first.id}`} item={first} tools={tools} onClose={close} />
}
