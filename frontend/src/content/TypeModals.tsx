// SPDX-License-Identifier: Apache-2.0

import { InputControl, Text } from '@gophenberg/frontend-sdk'
import type { RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { FormBody, RunConfirm, markLatest, useCloseWhileOpen, useToaster } from '@gopherium/godmin'
import type { BulkRun, ListRun, UnsavedText } from '@gopherium/godmin'
import { FormDialog } from '@gopherium/godmin/dialogs'
import { useQueryClient } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import { oneWriteWords } from '../lists/itemWords'
import { typesQueryKey } from './nav'
import { pluralOf } from './typeText'
import { createType, slugifyKey } from './types'
import type { ContentType, NewType, TypeEdit } from './types'
import { editInTurn, refreshTypes } from './typeWrites'

/** What a row form of the types list takes beside the props DataViews hands a modal. */
export interface TypeFormProps extends RenderModalProps<ContentType> {
	/** The text failed writes left unsaved, offered again when the form opens for the same type. */
	unsaved: UnsavedText
	/** Runs the write over the type, the list reloading once it ended. */
	run: BulkRun<ContentType>
}

/**
 * Returns the value a row form edits and its send, which closes the form at once, as WordPress closes a row edit.
 * @param props - The type, the handler closing the modal, the unsaved text and the run.
 * @param stored - The value the type holds now.
 * @param asEdit - Turns the value typed into the edit to send.
 * @param done - The toast once the edit landed.
 * @returns The value, its setter and the send.
 */
function useRowEdit(props: TypeFormProps, stored: string, asEdit: (value: string) => TypeEdit, done: string) {
	const { items, closeModal, unsaved, run } = props
	const [type] = items
	const client = useQueryClient()
	const [value, setValue] = useState(() => unsaved.get(type.key) ?? stored)
	const write = (stillLatest: () => boolean) =>
		editInTurn(client, type.key, asEdit(value)).catch((failure: unknown) => {
			if (stillLatest()) {
				unsaved.set(type.key, value)
			}
			throw failure
		})
	const save = () => {
		closeModal?.()
		unsaved.delete(type.key)
		const stillLatest = markLatest(unsaved, type.key)
		void run(items, () => write(stillLatest), oneWriteWords(done))
	}
	return { value, setValue, save }
}

/**
 * Renders the row form rewriting what a type's description says.
 * @param props - The type, the handler closing the modal and the unsaved descriptions.
 * @returns The form.
 */
export function DescribeModal(props: TypeFormProps) {
	const edit = useRowEdit(
		props,
		props.items[0].description,
		(description) => ({ description }),
		__('Description updated.', 'gophenberg'),
	)
	return (
		<FormBody
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={edit.save}
			onCancel={props.closeModal}
		>
			<InputControl
				label={__('Description', 'gophenberg')}
				description={__('A descriptive summary of the content type.', 'gophenberg')}
				autoComplete="off"
				value={edit.value}
				onValueChange={edit.setValue}
			/>
		</FormBody>
	)
}

/**
 * Renders the row form moving a type to a new route word, the warning first, as WordPress puts help before a field.
 * @param props - The type, the handler closing the modal and the unsaved route words.
 * @returns The form.
 */
export function AddressModal(props: TypeFormProps) {
	const edit = useRowEdit(
		props,
		props.items[0].routeWord,
		(routeWord) => ({ routeWord: routeWord.trim() }),
		__('Address updated.', 'gophenberg'),
	)
	return (
		<FormBody
			submitLabel={__('Move every address', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			canSubmit={edit.value.trim() !== ''}
			onSubmit={edit.save}
			onCancel={props.closeModal}
		>
			<Text>
				{__('Every address of this type moves. Links kept elsewhere to the old addresses stop working.', 'gophenberg')}
			</Text>
			<InputControl
				label={__('Route word', 'gophenberg')}
				autoComplete="off"
				value={edit.value}
				onValueChange={edit.setValue}
			/>
		</FormBody>
	)
}

/**
 * Returns the question asked before a type takes the root, naming where the type holding it moves.
 * @param type - The type taking the root.
 * @param holder - The type holding the root now, none when no type does.
 * @returns The question, one paragraph as WordPress words its homepage question.
 */
function rootQuestion(type: ContentType, holder: ContentType | undefined): string {
	const answers = sprintf(__('%(type)s will answer at the root.', 'gophenberg'), { type: pluralOf(type) })
	if (holder === undefined) {
		return answers
	}
	const moves = sprintf(__('%(name)s moves to /%(word)s. Every address of both types changes.', 'gophenberg'), {
		name: pluralOf(holder),
		word: holder.derivedRouteWord,
	})
	return `${answers} ${moves}`
}

/**
 * Renders the confirm handing the root to a type, busy until the write ends, as WordPress sets a homepage.
 * @param props - The type, the handler closing the modal and the run.
 * @returns The confirm, naming the type that held the root in the list on show when the confirm opened.
 */
export function MakeDefaultModal({
	items,
	closeModal,
	run,
}: RenderModalProps<ContentType> & { run: BulkRun<ContentType> }) {
	const client = useQueryClient()
	const [holder] = useState(() => client.getQueryData<ContentType[]>(typesQueryKey)?.find((held) => held.isDefault))
	const [type] = items
	const handOver = () => editInTurn(client, type.key, { isDefault: true })
	return (
		<RunConfirm
			items={items}
			closeModal={closeModal}
			confirmLabel={__('Make default', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			run={() => run(items, handOver, oneWriteWords(__('Default content type updated.', 'gophenberg')))}
		>
			{rootQuestion(type, holder)}
		</RunConfirm>
	)
}

/** The fields of Add Content Type. */
type TypeForm = Pick<NewType, 'singularLabel' | 'pluralLabel' | 'description'>

/** The fields of Add Content Type with nothing typed. */
const EMPTY_FORM: TypeForm = { singularLabel: '', pluralLabel: '', description: '' }

/**
 * Renders the dialog registering a new type as WordPress asks for a new pattern, kept open after a failure.
 * @param props - What takes the dialog away and what the list does around a reload.
 * @returns The dialog.
 */
export function AddTypeDialog({ onClose, list }: { onClose: () => void, list: ListRun }) {
	const client = useQueryClient()
	const toaster = useToaster()
	const closeWhileOpen = useCloseWhileOpen(onClose)
	const [form, setForm] = useState(EMPTY_FORM)
	const fill = (field: keyof TypeForm) => (value: string) => setForm((now) => ({ ...now, [field]: value }))
	const add = () =>
		createType({ ...form, key: slugifyKey(form.singularLabel), routeWord: slugifyKey(form.pluralLabel) }).then(
			async (created) => {
				toaster.show(sprintf(_x('"%s" successfully created.', 'content type', 'gophenberg'), pluralOf(created)))
				closeWhileOpen()
				const ended = list.onRefresh()
				await refreshTypes(client).then(ended, ended)
			},
			(failure: Error) => toaster.show(failure.message),
		)
	return (
		<FormDialog
			title={__('Add Content Type', 'gophenberg')}
			submitLabel={__('Add', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			canSubmit={form.singularLabel.trim() !== '' && form.pluralLabel.trim() !== ''}
			onSubmit={add}
			onClose={onClose}
		>
			<InputControl
				label={__('Singular name', 'gophenberg')}
				autoComplete="off"
				value={form.singularLabel}
				onValueChange={fill('singularLabel')}
			/>
			<InputControl
				label={__('Plural name', 'gophenberg')}
				description={sprintf(__('This type will answer under /%(word)s.', 'gophenberg'), {
					word: slugifyKey(form.pluralLabel) || __('address', 'gophenberg'),
				})}
				autoComplete="off"
				value={form.pluralLabel}
				onValueChange={fill('pluralLabel')}
			/>
			<InputControl
				label={__('Description', 'gophenberg')}
				description={__('A descriptive summary of the content type.', 'gophenberg')}
				autoComplete="off"
				value={form.description}
				onValueChange={fill('description')}
			/>
		</FormDialog>
	)
}
