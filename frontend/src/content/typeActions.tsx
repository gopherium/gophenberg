// SPDX-License-Identifier: Apache-2.0

import { checkIcon, notAllowedIcon, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { RunConfirm, useBulkRun, useUnsavedText } from '@gopherium/godmin'
import type { BulkRun, ListRun, UnsavedText } from '@gopherium/godmin'
import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { useCallback, useMemo } from 'react'

import { oneWriteWords } from '../lists/itemWords'
import { originOf } from './typeFields'
import { AddressModal, DescribeModal, MakeDefaultModal } from './TypeModals'
import type { ContentType } from './types'
import { removalQuestion, removalWords, switchWords } from './typeWords'
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

/**
 * Returns the actions opening a form or a confirm about one type.
 * @param tools - What the actions need from the screen.
 * @returns Describe, Change address and Make default.
 */
function formActions(tools: ActionTools): Action<ContentType>[] {
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
			modalHeader: __('Make default', 'gophenberg'),
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => <MakeDefaultModal {...props} run={tools.run} />,
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
 * Renders the confirm deleting the picked types for good, closing only itself once the run ends.
 * @param props - The types, the handler closing the modal and what the action needs from the screen.
 * @returns The confirm.
 */
function DeleteTypesModal({ items, closeModal, tools }: RenderModalProps<ContentType> & { tools: ActionTools }) {
	return (
		<RunConfirm
			items={items}
			closeModal={closeModal}
			confirmLabel={__('Delete', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			run={(types) => tools.run(types, (type) => deleteInTurn(tools.client, type.key), removalWords())}
		>
			{removalQuestion(items)}
		</RunConfirm>
	)
}

/**
 * Returns the action deleting the picked types for good, behind a confirm with no header.
 * @param tools - What the action needs from the screen.
 * @returns Delete, taking several types.
 */
function deleteAction(tools: ActionTools): Action<ContentType> {
	return {
		id: 'delete',
		label: __('Delete…', 'gophenberg'),
		icon: trashIcon,
		supportsBulk: true,
		isEligible: demotable,
		hideModalHeader: true,
		modalFocusOnMount: 'firstContentElement',
		RenderModal: (props) => <DeleteTypesModal {...props} tools={tools} />,
	}
}

/**
 * Returns the actions offered on each row of the types list: the forms first, the one click switches, Delete last.
 * @param list - What the list does around a run.
 * @returns The row actions, the same for as long as the screen stays, so a confirm never starts over in a run.
 */
export function useTypeActions(list: ListRun): Action<ContentType>[] {
	const client = useQueryClient()
	const refresh = useCallback(() => refreshTypes(client), [client])
	const run = useBulkRun<ContentType>(list, refresh)
	const descriptions = useUnsavedText()
	const routeWords = useUnsavedText()
	return useMemo(() => {
		const tools = { client, run, descriptions, routeWords }
		return [...formActions(tools), ...switchActions(tools), deleteAction(tools)]
	}, [client, descriptions, routeWords, run])
}
