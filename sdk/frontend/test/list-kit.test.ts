// SPDX-License-Identifier: Apache-2.0

import * as godmin from '@gopherium/godmin'
import { expect, test } from 'vitest'

import {
	ConfirmBody,
	FormBody,
	ListEmpty,
	ListNoResults,
	RenameBody,
	RunConfirm,
	focusInDialog,
	focusedTrigger,
	markLatest,
	useBulkRun,
	useCloseWhileOpen,
	useFocusBack,
	useListRun,
	useToaster,
	useUnsavedText,
	viewNarrows,
} from '../index'
import type {
	BulkRun,
	BulkWords,
	ConfirmBodyProps,
	FormBodyProps,
	FormKind,
	ListRun,
	ListRunState,
	RenameBodyProps,
	RunConfirmProps,
	RunSettled,
	ToasterHandle,
	Trigger,
	UnsavedText,
} from '../index'

/** The words a plugin's bulk run reports its outcome in, typed through the SDK. */
const WORDS: BulkWords<{ id: string }> = {
	done: (count) => `Done: ${count}.`,
	failed: (failures, asked) => `Failed: ${failures.length} of ${asked}.`,
}

/**
 * Expects every piece the SDK hands out to be a function, the very one godmin ships in its place.
 * @param handed - The pieces the SDK hands out.
 * @param shipped - The pieces godmin ships, in the same order.
 */
function expectGodmins(handed: unknown[], shipped: unknown[]): void {
	expect(handed.map((piece) => typeof piece)).toEqual(shipped.map(() => 'function'))
	expect(handed).toEqual(shipped)
}

test('hands plugins the list kit the admin runs, the very functions godmin ships', () => {
	expectGodmins([RunConfirm, useBulkRun, useListRun], [godmin.RunConfirm, godmin.useBulkRun, godmin.useListRun])
})

test('hands plugins the dialog bodies the admin asks in, the very ones godmin ships', () => {
	expectGodmins([ConfirmBody, FormBody, RenameBody], [godmin.ConfirmBody, godmin.FormBody, godmin.RenameBody])
})

test('hands plugins the toaster the admin reports a failed write in, the very one godmin ships', () => {
	const handle: Pick<ToasterHandle, 'show'> | undefined = undefined

	expectGodmins([useToaster], [godmin.useToaster])
	expect(handle).toBeUndefined()
})

test('hands plugins the store a failed row write keeps its text in, the very one godmin ships', () => {
	const kept: UnsavedText = new Map([['7', 'Harbor at dawn']])

	expectGodmins([useUnsavedText, markLatest], [godmin.useUnsavedText, godmin.markLatest])
	expect(kept.get('7')).toBe('Harbor at dawn')
})

test('hands plugins the empty states the admin draws an empty list with, the very ones godmin ships', () => {
	expectGodmins(
		[ListEmpty, ListNoResults, viewNarrows],
		[godmin.ListEmpty, godmin.ListNoResults, godmin.viewNarrows],
	)
})

test('hands plugins the hooks a dialog closes and gives the focus back with, the very ones godmin ships', () => {
	const trigger: Trigger = 'actions'

	expectGodmins(
		[useCloseWhileOpen, useFocusBack, focusedTrigger, focusInDialog],
		[godmin.useCloseWhileOpen, godmin.useFocusBack, godmin.focusedTrigger, godmin.focusInDialog],
	)
	expect(trigger).toBe('actions')
})

test('hands plugins the types the list kit is written in', () => {
	const props: Pick<RunConfirmProps<{ id: string }>, 'confirmLabel' | 'cancelLabel'> = {
		confirmLabel: 'Trash',
		cancelLabel: 'Cancel',
	}
	const run: BulkRun<{ id: string }> | undefined = undefined
	const list: ListRun | undefined = undefined
	const state: ListRunState | undefined = undefined
	const settled: RunSettled = () => undefined

	expect([props.confirmLabel, run, list, state, settled(true), WORDS.done(2, undefined)]).toEqual([
		'Trash',
		undefined,
		undefined,
		undefined,
		undefined,
		'Done: 2.',
	])
})

test('hands plugins the types the dialog bodies are written in', () => {
	const kind: FormKind = 'copy'
	const confirm: Pick<ConfirmBodyProps, 'confirmLabel'> = { confirmLabel: 'Delete' }
	const form: Pick<FormBodyProps, 'submitLabel' | 'kind'> = { submitLabel: 'Duplicate', kind }
	const rename: Pick<RenameBodyProps, 'submitLabel'> = { submitLabel: 'Save' }

	expect([confirm.confirmLabel, form.submitLabel, form.kind, rename.submitLabel]).toEqual([
		'Delete',
		'Duplicate',
		'copy',
		'Save',
	])
})
