// SPDX-License-Identifier: Apache-2.0

import { checkIcon, notAllowedIcon, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action } from '@gophenberg/frontend-sdk/dataviews'
import { ConfirmDialog, focusedTrigger, useBulkRun, useUnsavedText } from '@gopherium/godmin'
import type { BulkRun, ListRun, Trigger, UnsavedText } from '@gopherium/godmin'
import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { useCallback, useMemo } from 'react'

import { irreversible, oneWriteWords } from '../lists/itemWords'
import { originOf } from './typeFields'
import { AddressModal, DescribeModal, MakeDefaultConfirm } from './TypeModals'
import type { ContentType } from './types'
import { removalTitle, removalWords, switchWords } from './typeWords'
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
	/** The descriptions failed saves left unsaved. */
	descriptions: UnsavedText
	/** The route words failed moves left unsaved. */
	routeWords: UnsavedText
}

/** A confirm the screen holds open on the picked types. */
export interface OpenedTypes {
	/** The action the confirm belongs to. */
	action: 'delete' | 'make-default'
	/** The types the confirm is open on. */
	items: ContentType[]
	/** The trigger the confirm opened from, which takes the focus back once the confirm is done. */
	trigger: Trigger
}

/**
 * Returns the actions opening a form or a confirm about one type.
 * @param tools - What the actions need from the screen.
 * @param open - Opens Make default on the type in a confirm the screen holds.
 * @returns Describe, Change address and Make default.
 */
function formActions(tools: ActionTools, open: (opened: OpenedTypes) => void): Action<ContentType>[] {
	return [
		{
			id: 'describe',
			label: __('Describe…', 'gophenberg'),
			isEligible: madeHere,
			modalHeader: __('Describe', 'gophenberg'),
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => <DescribeModal {...props} unsaved={tools.descriptions} run={tools.run} />,
		},
		{
			id: 'change-address',
			label: __('Change address…', 'gophenberg'),
			isEligible: madeHere,
			modalHeader: __('Change address', 'gophenberg'),
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => <AddressModal {...props} unsaved={tools.routeWords} run={tools.run} />,
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

/**
 * Renders the confirm the screen holds open on the picked types, none while no confirm is open.
 * @param props - The confirm, its types and the trigger it opened from, what it needs from the screen and what takes
 *   it away.
 * @returns Make default on one type, or the red confirm deleting the types for good, each busy until the run ends.
 */
export function OpenTypeDialog({ opened, tools, onClose }: {
	opened?: OpenedTypes
	tools: ActionTools
	onClose: () => void
}) {
	if (opened === undefined) {
		return null
	}
	if (opened.action === 'make-default') {
		return <MakeDefaultConfirm type={opened.items[0]} run={tools.run} onClose={onClose} />
	}
	return (
		<ConfirmDialog
			intent="irreversible"
			title={removalTitle(opened.items)}
			description={irreversible()}
			confirmLabel={__('Delete', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onConfirm={() => tools.run(opened.items, (type) => deleteInTurn(tools.client, type.key), removalWords())}
			onClose={onClose}
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
 * @returns The tools, whose run reloads the registry once the last call ended.
 */
export function useTypeTools(list: ListRun): ActionTools {
	const client = useQueryClient()
	const refresh = useCallback(() => refreshTypes(client), [client])
	const run = useBulkRun<ContentType>(list, refresh)
	const descriptions = useUnsavedText()
	const routeWords = useUnsavedText()
	return useMemo(() => ({ client, run, descriptions, routeWords }), [client, descriptions, routeWords, run])
}

/**
 * Returns the actions offered on each row of the types list: the forms first, the one click switches, Delete last.
 * @param tools - What the actions need from the screen.
 * @param open - Opens a confirm on the picked types that the screen holds.
 * @returns The row actions, the same for as long as the screen stays, so a form never starts over in a run.
 */
export function useTypeActions(tools: ActionTools, open: (opened: OpenedTypes) => void): Action<ContentType>[] {
	return useMemo(() => [...formActions(tools, open), ...switchActions(tools), deleteAction(open)], [open, tools])
}
