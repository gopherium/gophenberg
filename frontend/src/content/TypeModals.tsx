// SPDX-License-Identifier: Apache-2.0

import { InputControl } from '@gophenberg/frontend-sdk'
import { ConfirmDialog, FormDialog, useCloseWhileOpen, useToaster } from '@gopherium/godmin'
import type { BulkRun, DialogRun, ListRun } from '@gopherium/godmin'
import { useQueryClient } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import { oneWriteWords } from '../lists/itemWords'
import { typesQueryKey } from './nav'
import { pluralOf } from './typeText'
import { createType, slugifyKey } from './types'
import type { ContentType, NewType, TypeEdit } from './types'
import { editInTurn, refreshTypes } from './typeWrites'

/** What a row form the types screen holds open on one type takes. */
export interface TypeFormProps {
	/** The type the form is open on. */
	type: ContentType
	/** Runs the write over the type and answers the form, the list reloading once it ended. */
	write: DialogRun<ContentType>
	/** Takes the form away, once its card has left the page. */
	onClose: () => void
}

/**
 * Returns the value a row form edits and its send, which holds the form open until the edit answers.
 * @param props - The type and the write.
 * @param stored - The value the type holds now.
 * @param asEdit - Turns the value typed into the edit to send.
 * @param done - The toast once the edit landed.
 * @returns The value, its setter and the send.
 */
function useRowEdit(props: TypeFormProps, stored: string, asEdit: (value: string) => TypeEdit, done: string) {
	const { type, write } = props
	const client = useQueryClient()
	const [value, setValue] = useState(stored)
	const save = () => write([type], () => editInTurn(client, type.key, asEdit(value)), oneWriteWords(done))
	return { value, setValue, save }
}

/**
 * Renders the row form rewriting what a type's description says.
 * @param props - The type, the write and what takes the form away.
 * @returns The form dialog.
 */
export function DescribeDialog(props: TypeFormProps) {
	const edit = useRowEdit(
		props,
		props.type.description,
		(description) => ({ description }),
		__('Description updated.', 'gophenberg'),
	)
	return (
		<FormDialog
			title={__('Describe', 'gophenberg')}
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={edit.save}
			onClose={props.onClose}
		>
			{(busy) => (
				<InputControl
					label={__('Description', 'gophenberg')}
					description={__('A descriptive summary of the content type.', 'gophenberg')}
					autoComplete="off"
					readOnly={busy}
					value={edit.value}
					onValueChange={edit.setValue}
				/>
			)}
		</FormDialog>
	)
}

/**
 * Renders the row form moving a type to a new route word, the warning first, as WordPress puts help before a field.
 * @param props - The type, the write and what takes the form away.
 * @returns The form dialog.
 */
export function AddressDialog(props: TypeFormProps) {
	const edit = useRowEdit(
		props,
		props.type.routeWord,
		(routeWord) => ({ routeWord: routeWord.trim() }),
		__('Address updated.', 'gophenberg'),
	)
	return (
		<FormDialog
			title={__('Change address', 'gophenberg')}
			description={__(
				'Every address of this type moves. Links kept elsewhere to the old addresses stop working.',
				'gophenberg',
			)}
			submitLabel={__('Move every address', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			canSubmit={edit.value.trim() !== ''}
			onSubmit={edit.save}
			onClose={props.onClose}
		>
			{(busy) => (
				<InputControl
					label={__('Route word', 'gophenberg')}
					autoComplete="off"
					readOnly={busy}
					value={edit.value}
					onValueChange={edit.setValue}
				/>
			)}
		</FormDialog>
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
 * @param props - The type, the run and what takes the confirm away.
 * @returns The confirm, naming the type that held the root in the list on show when the confirm opened.
 */
export function MakeDefaultConfirm({ type, run, onClose }: {
	type: ContentType
	run: BulkRun<ContentType>
	onClose: () => void
}) {
	const client = useQueryClient()
	const [holder] = useState(() => client.getQueryData<ContentType[]>(typesQueryKey)?.find((held) => held.isDefault))
	const handOver = () => editInTurn(client, type.key, { isDefault: true })
	return (
		<ConfirmDialog
			title={sprintf(__('Make "%(type)s" the default?', 'gophenberg'), { type: pluralOf(type) })}
			description={rootQuestion(type, holder)}
			confirmLabel={__('Make default', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onConfirm={() => run([type], handOver, oneWriteWords(__('Default content type updated.', 'gophenberg')))}
			onClose={onClose}
		/>
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
