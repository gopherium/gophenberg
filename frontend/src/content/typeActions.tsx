// SPDX-License-Identifier: Apache-2.0

import { checkIcon, notAllowedIcon, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action } from '@gophenberg/frontend-sdk/dataviews'
import { ConfirmDialog, focusedTrigger, useBulkRun, useDialogRun } from '@gopherium/godmin'
import type { BulkRun, DialogRun, ListRun, Trigger } from '@gopherium/godmin'
import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { useCallback, useMemo } from 'react'

import { oneWriteWords } from '../lists/itemWords'
import { originOf } from './typeFields'
import { AddressDialog, DescribeDialog, MakeDefaultConfirm } from './TypeModals'
import type { ContentType } from './types'
import { removalDescription, removalTitle, removalWords, switchWords } from './typeWords'
import { deleteInTurn, editInTurn, refreshTypes } from './typeWrites'

/**
 * Reports whether the site made a type, so its shape is the site's to change.
 * @param type - The type.
 * @returns Whether no plugin declared it.
 */
function madeHere(type: ContentType): boolean {
	return originOf(type) === ''
}

/**
 * Reports whether a type may hand its place over or leave, being the site's and not the one at the root.
 * @param type - The type.
 * @returns Whether the site made it and it is not the default.
 */
function demotable(type: ContentType): boolean {
	return madeHere(type) && !type.isDefault
}

/** What the actions of the types list need from the screen around them. */
interface ActionTools {
	/** The query client the writes run on. */
	client: QueryClient
	/** Runs a call over types, the list reloading once the last call ended. */
	run: BulkRun<ContentType>
	/** Runs the write of a row form and answers the form, the list reloading once the write ended. */
	write: DialogRun<ContentType>
}

/** A dialog the screen holds open on the picked types. */
export interface OpenedTypes {
	/** The action the dialog belongs to. */
	action: 'delete' | 'make-default' | 'describe' | 'change-address'
	/** The types the dialog is open on. */
	items: ContentType[]
	/** The trigger the dialog opened from, which takes the focus back once the dialog is done. */
	trigger: Trigger
}

/**
 * Returns the actions opening a form or a confirm about one type.
 * @param open - Opens the form or the confirm on the type in a dialog the screen holds.
 * @returns Describe, Change address and Make default.
 */
function formActions(open: (opened: OpenedTypes) => void): Action<ContentType>[] {
	return [
		{
			id: 'describe',
			label: __('Describe…', 'gophenberg'),
			isEligible: madeHere,
			callback: (items) => open({ action: 'describe', items, trigger: focusedTrigger() }),
		},
		{
			id: 'change-address',
			label: __('Change address…', 'gophenberg'),
			isEligible: madeHere,
			callback: (items) => open({ action: 'change-address', items, trigger: focusedTrigger() }),
		},
		{
			id: 'make-default',
			label: __('Make default…', 'gophenberg'),
			isEligible: demotable,
			callback: (items) => open({ action: 'make-default', items, trigger: focusedTrigger() }),
		},
	]
}

/**
 * Returns the one click actions switching what a type does.
 * @param tools - What the actions need from the screen.
 * @returns The nesting switches, then Activate and Deactivate, both taking several types.
 */
function switchActions(tools: ActionTools): Action<ContentType>[] {
	const activeAs = (on: boolean) => (types: ContentType[]) =>
		tools.run(types, (type) => editInTurn(tools.client, type.key, { active: on }), switchWords(on))
	const nestingAs = (hierarchical: boolean) => (types: ContentType[]) =>
		tools.run(
			types,
			(type) => editInTurn(tools.client, type.key, { hierarchical }),
			oneWriteWords(__('Content type updated.', 'gophenberg')),
		)
	return [
		{
			id: 'nest',
			label: __('Let items nest', 'gophenberg'),
			isEligible: (type) => madeHere(type) && !type.hierarchical,
			callback: nestingAs(true),
		},
		{
			id: 'stop-nesting',
			label: __('Stop nesting', 'gophenberg'),
			isEligible: (type) => madeHere(type) && type.hierarchical,
			callback: nestingAs(false),
		},
		{
			id: 'activate',
			label: __('Activate', 'gophenberg'),
			icon: checkIcon,
			supportsBulk: true,
			isEligible: (type) => !type.active,
			callback: activeAs(true),
		},
		{
			id: 'deactivate',
			label: __('Deactivate', 'gophenberg'),
			icon: notAllowedIcon,
			supportsBulk: true,
			isEligible: (type) => type.active && !type.isDefault,
			callback: activeAs(false),
		},
	]
}

/** The row forms the screen holds open on one type, by the action they belong to. */
const ROW_FORMS = { describe: DescribeDialog, 'change-address': AddressDialog }

/**
 * Renders the dialog the screen holds open on the picked types, none while no dialog is open.
 * @param props - The dialog, its types and the trigger it opened from, what it needs from the screen, what takes it
 *   away and what puts a lost focus back on the trigger it opened from.
 * @returns Describe, Change address or Make default on one type, or the red confirm deleting the types for good, each
 *   busy until its write ends and giving the focus back to the trigger it opened from once it closed.
 */
export function OpenTypeDialog({ opened, tools, onClose, onBack }: {
	opened?: OpenedTypes
	tools: ActionTools
	onClose: () => void
	onBack: (id: string, trigger: Trigger) => void
}) {
	if (opened === undefined) {
		return null
	}
	const [type] = opened.items
	const close = () => {
		onClose()
		onBack(type.key, opened.trigger)
	}
	if (opened.action === 'make-default') {
		return <MakeDefaultConfirm type={type} run={tools.run} onClose={close} />
	}
	if (opened.action !== 'delete') {
		const Form = ROW_FORMS[opened.action]
		return <Form key={`${opened.action} ${type.key}`} type={type} write={tools.write} onClose={close} />
	}
	return (
		<ConfirmDialog
			intent="irreversible"
			title={removalTitle(opened.items)}
			description={removalDescription(opened.items.length)}
			confirmLabel={__('Delete', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onConfirm={() => tools.run(opened.items, (type) => deleteInTurn(tools.client, type.key), removalWords())}
			onClose={close}
		/>
	)
}

/**
 * Returns the action deleting the picked types for good, behind a confirm the screen holds.
 * @param open - Opens the confirm on the picked types.
 * @returns Delete, taking several types.
 */
function deleteAction(open: (opened: OpenedTypes) => void): Action<ContentType> {
	return {
		id: 'delete',
		label: __('Delete…', 'gophenberg'),
		icon: trashIcon,
		supportsBulk: true,
		isEligible: demotable,
		callback: (items) => open({ action: 'delete', items, trigger: focusedTrigger() }),
	}
}

/**
 * Returns what the actions and the dialogs of the types list need, the same for as long as the screen stays.
 * @param list - What the list does around a run.
 * @returns The tools, whose run and write reload the registry once the last call ended.
 */
export function useTypeTools(list: ListRun): ActionTools {
	const client = useQueryClient()
	const refresh = useCallback(() => refreshTypes(client), [client])
	const run = useBulkRun<ContentType>(list, refresh)
	const write = useDialogRun<ContentType>(list, refresh)
	return useMemo(() => ({ client, run, write }), [client, run, write])
}

/**
 * Returns the actions offered on each row of the types list: the forms first, the one click switches, Delete last.
 * @param tools - What the actions need from the screen.
 * @param open - Opens a form or a confirm on the picked types that the screen holds.
 * @returns The row actions, the same for as long as the screen stays.
 */
export function useTypeActions(tools: ActionTools, open: (opened: OpenedTypes) => void): Action<ContentType>[] {
	return useMemo(() => [...formActions(open), ...switchActions(tools), deleteAction(open)], [open, tools])
}
