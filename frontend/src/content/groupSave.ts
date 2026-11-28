// SPDX-License-Identifier: Apache-2.0

import { bulkNotes, useToaster } from '@gopherium/godmin'
import type { BulkFailure, BulkNotes, BulkOutcome } from '@gopherium/godmin'
import { formatList, formatNumber } from '@gopherium/gottext'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { __, _n, sprintf } from '@wordpress/i18n'

import { useWriteTimeout } from '../settings/useListSettings'
import { changedPaths, fieldEditOf } from './fieldDraft'
import type { FieldDrafts } from './fieldDraft'
import type { FieldFailure } from './fieldNotes'
import type { GroupDetailsState } from './GroupDetails'
import { changedParts, editOf } from './groupDraft'
import type { GroupDraft } from './groupDraft'
import {
	editFieldInGroup,
	fieldRead,
	groupErrorMessage,
	groupsQueryKey,
	groupsUnreached,
	StaleWriteError,
	staleFieldMessage,
	staleGroupMessage,
	updateGroup,
} from './groups'
import type { FieldGroup } from './groups'
import { typesQueryKey } from './nav'
import { ownValue } from './types'
import type { ContentField } from './types'

/** One change the header Save sends: a field by its dotted path, or the group's own parts under an empty path. */
interface SaveItem {
	/** The dotted path of the field, empty for the group's own parts. */
	path: string
	/** The name the words give the change. */
	label: string
}

/** What the header Save sends: the details of a group and the drafts of its fields. */
export interface SaveAsked {
	/** The group as last read. */
	held: FieldGroup
	/** Every group read, which a Linked from pick may stand in. */
	groups: FieldGroup[]
	/** The group the details draft stands on. */
	base: FieldGroup
	/** The draft of the group's details. */
	draft: GroupDraft
	/** The drafts of the group's fields, each under its dotted path. */
	drafts: FieldDrafts
}

/** What the header Save reached: each change it sent, how they went, the fields it saved and the group as stored. */
interface SaveRun {
	items: SaveItem[]
	outcome: BulkOutcome<SaveItem>
	/** The fields the run saved, each under its dotted path. */
	saved: Record<string, ContentField>
	/** The group as stored, none when the run sent no edit of the group or the edit failed. */
	stored?: FieldGroup
	/** The title the group carries once the run ended. */
	title: string
}

/** The changes a run sent so far, and the ones that failed. */
interface Tally {
	items: SaveItem[]
	failures: BulkFailure<SaveItem>[]
}

/**
 * Runs one field group write, stopping it once it ran past the time limit.
 * @param limit - The time limit in milliseconds the request gets, Infinity for none.
 * @param send - The request, handed the signal that stops it.
 * @returns The answer, or throws why the request failed, naming the field groups unreached once it ran past.
 */
export async function underLimit<T>(limit: number, send: (signal: AbortSignal) => Promise<T>): Promise<T> {
	const controller = new AbortController()
	const timer = Number.isFinite(limit) ? setTimeout(() => controller.abort(), limit) : undefined
	try {
		return await send(controller.signal)
	} catch (cause) {
		throw controller.signal.aborted ? new Error(groupsUnreached()) : cause
	} finally {
		clearTimeout(timer)
	}
}

/**
 * Sends one change of the run under the time limit and counts it, noting its failure against it.
 * @param tally - The changes the run sent so far.
 * @param item - The change.
 * @param limit - The time limit in milliseconds the request gets, Infinity for none.
 * @param send - The request, handed the signal that stops it.
 * @returns The answer, or nothing once the change failed.
 */
async function sent<T>(
	tally: Tally,
	item: SaveItem,
	limit: number,
	send: (signal: AbortSignal) => Promise<T>,
): Promise<T | undefined> {
	tally.items.push(item)
	try {
		return await underLimit(limit, send)
	} catch (error) {
		tally.failures.push({ item, error })
		return undefined
	}
}

/**
 * Sends each changed field in page order, each once the one before it answered.
 * @param asked - What the header Save sends.
 * @param limit - The time limit in milliseconds each request gets, Infinity for none.
 * @param tally - The changes the run sent so far.
 * @returns The fields saved, each under its dotted path.
 */
async function savedFields(asked: SaveAsked, limit: number, tally: Tally): Promise<Record<string, ContentField>> {
	const saved: Record<string, ContentField> = {}
	for (const path of changedPaths(asked.held.fields, asked.drafts)) {
		const draft = asked.drafts[path]
		const read = fieldRead({ group: asked.held.id, path }, draft.base)
		const field = await sent(tally, { path, label: draft.base.label }, limit, (signal) =>
			editFieldInGroup(read, fieldEditOf(draft), signal),
		)
		if (field !== undefined) {
			saved[path] = field
		}
	}
	return saved
}

/**
 * Returns the fields with each field the run saved at its dotted path, the sub fields as last read.
 * @param fields - The fields as last read.
 * @param saved - The fields the run saved, each under its dotted path.
 * @param parentPath - The dotted path of the field holding the fields, empty at a group's top.
 * @returns The fields.
 */
function fieldsWithSaved(
	fields: ContentField[],
	saved: Record<string, ContentField>,
	parentPath = '',
): ContentField[] {
	return fields.map((field) => {
		const path = parentPath === '' ? field.key : `${parentPath}.${field.key}`
		const newestField = ownValue(saved, path) ?? field
		return { ...newestField, fields: fieldsWithSaved(field.fields, saved, path) }
	})
}

/**
 * Returns the group with each field the run saved at its dotted path.
 * @param group - The group as last read.
 * @param saved - The fields the run saved, each under its dotted path.
 * @returns The group.
 */
function withSaved(group: FieldGroup, saved: Record<string, ContentField>): FieldGroup {
	return { ...group, fields: fieldsWithSaved(group.fields, saved) }
}

/**
 * Returns the groups with the fields a run saved placed in the group it saved them in.
 * @param groups - The groups cached, none before the first read.
 * @param groupId - The id of the group the run saved the fields in.
 * @param saved - The fields the run saved, each under its dotted path.
 * @returns The groups to cache.
 */
function groupsWithSaved(
	groups: FieldGroup[] | undefined,
	groupId: number,
	saved: Record<string, ContentField>,
): FieldGroup[] | undefined {
	return groups?.map((group) => (group.id === groupId ? withSaved(group, saved) : group))
}

/**
 * Sends the parts of the group its details changed, after the fields.
 * @param asked - What the header Save sends.
 * @param saved - The fields the run saved, each under its dotted path.
 * @param limit - The time limit in milliseconds the request gets, Infinity for none.
 * @param tally - The changes the run sent so far.
 * @returns The group as stored, or nothing when its details changed nothing or its edit failed.
 */
async function storedGroup(
	asked: SaveAsked,
	saved: Record<string, ContentField>,
	limit: number,
	tally: Tally,
): Promise<FieldGroup | undefined> {
	const parts = changedParts(asked.base, asked.draft)
	if (parts.length === 0) {
		return undefined
	}
	const edit = editOf(asked.draft, parts, withSaved(asked.held, saved), asked.groups)
	return sent(tally, { path: '', label: asked.base.title }, limit, (signal) => updateGroup(asked.base, edit, signal))
}

/**
 * Sends every change the header Save holds: each changed field in page order, one after another, then the group.
 * @param asked - What the header Save sends.
 * @param limit - The time limit in milliseconds each request gets, Infinity for none.
 * @returns What the run reached.
 */
export async function runSave(asked: SaveAsked, limit: number): Promise<SaveRun> {
	const tally: Tally = { items: [], failures: [] }
	const saved = await savedFields(asked, limit, tally)
	const stored = await storedGroup(asked, saved, limit, tally)
	const { items, failures } = tally
	return {
		items,
		outcome: { asked: items.length, done: items.length - failures.length, failures },
		saved,
		stored,
		title: stored?.title ?? asked.held.title,
	}
}

/**
 * Returns the reason one change of the header Save failed, in the reader's words.
 * @param failure - The change and what it failed with.
 * @returns The reason.
 */
function reasonOf(failure: BulkFailure<SaveItem>): string {
	const { item, error } = failure
	if (!(error instanceof StaleWriteError)) {
		return groupErrorMessage(error)
	}
	return item.path === '' ? staleGroupMessage(item.label) : staleFieldMessage(item.label)
}

/**
 * Returns the sentence counting the changes that saved, empty while none did.
 * @param done - How many changes saved.
 * @param asked - How many changes the run sent.
 * @returns The sentence.
 */
function savedCount(done: number, asked: number): string {
	if (done === 0) {
		return ''
	}
	return sprintf(
		_n('Saved %(done)s of %(asked)s change.', 'Saved %(done)s of %(asked)s changes.', asked, 'gophenberg'),
		{ done: formatNumber(done), asked: formatNumber(asked) },
	)
}

/**
 * Returns the notice naming the changes the header Save could not store, with the reason when only one failed.
 * @param failures - The changes that failed and what each failed with.
 * @param asked - How many changes the run sent.
 * @returns The notice.
 */
function failedWords(failures: BulkFailure<SaveItem>[], asked: number): string {
	const [first] = failures
	if (asked === 1 && first.item.path === '') {
		return sprintf(__('Could not save the field group. %s', 'gophenberg'), reasonOf(first))
	}
	const names = formatList(failures.map((failure) => failure.item.label))
	const lines = [
		savedCount(asked - failures.length, asked),
		sprintf(__('Could not save %(names)s.', 'gophenberg'), { names }),
		failures.length === 1 ? reasonOf(first) : '',
	]
	return lines.filter((line) => line !== '').join(' ')
}

/**
 * Returns the toast and the notice a run of the header Save reads as.
 * @param ran - What the run reached.
 * @returns The toast saying the group was updated once every change saved, and the notice naming what did not.
 */
export function saveNotes(ran: SaveRun): BulkNotes {
	return bulkNotes(ran.items, ran.outcome, {
		done: () => sprintf(__('%(group)s updated.', 'gophenberg'), { group: ran.title }),
		failed: failedWords,
	})
}

/** What the fields of a group's page hear of a run of the header Save. */
interface FieldsWatch {
	/** Runs as the header Save starts. */
	started: () => void
	/** Hands over the dotted paths of the fields the run saved, and the fields it could not save with why. */
	ran: (saved: string[], failures: FieldFailure[]) => void
}

/** What the header Save of a group's page works on. */
interface SavePage {
	held: FieldGroup
	groups: FieldGroup[]
	details: GroupDetailsState
	drafts: FieldDrafts
	/** What the fields hear of each run. */
	fields: FieldsWatch
}

/**
 * Returns the groups with one group's details as the server stored them and its fields as held.
 * @param listed - The groups held, none before the first read.
 * @param stored - The group the server answered with.
 * @returns The groups to hold.
 */
function withStored(listed: FieldGroup[] | undefined, stored: FieldGroup): FieldGroup[] | undefined {
	return listed?.map((one) => (one.id === stored.id ? { ...one, ...stored, fields: one.fields } : one))
}

/**
 * Returns the fields a run of the header Save could not store, each by its dotted path with what it failed with.
 * @param ran - What the run reached.
 * @returns The failed fields, the group's own parts left out.
 */
function fieldFailures(ran: SaveRun): FieldFailure[] {
	return ran.outcome.failures
		.filter((failure) => failure.item.path !== '')
		.map((failure) => ({ path: failure.item.path, error: failure.error }))
}

/**
 * Hands the details what the run did to the group: stored it, failed, or left it alone.
 * @param ran - What the run reached.
 * @param details - The details of the group.
 * @param client - The query client holding the groups.
 */
function settleDetails(ran: SaveRun, details: GroupDetailsState, client: QueryClient) {
	const { stored } = ran
	if (stored !== undefined) {
		client.setQueryData<FieldGroup[]>(groupsQueryKey, (listed) => withStored(listed, stored))
		details.saveLanded()
	}
	const failed = ran.outcome.failures.find((failure) => failure.item.path === '')
	if (failed !== undefined) {
		details.saveFailed(failed.error)
	}
}

/**
 * Returns the header Save of a group's page: whether any part changed, whether it runs, and what runs it.
 * @param page - The group, every group read, its details, the drafts of its fields, and what changes those drafts.
 * @returns Whether the details or a field at a path that still exists changed, whether the run is on its way, and
 * what starts it.
 */
export function useGroupSave(page: SavePage) {
	const client = useQueryClient()
	const toaster = useToaster()
	const limit = useWriteTimeout()
	const { held: group, details, drafts } = page
	const saveMutation = useMutation({
		mutationFn: () =>
			runSave({ held: group, groups: page.groups, base: details.base, draft: details.draft, drafts }, limit),
		onMutate: () => {
			details.saveStarted()
			page.fields.started()
		},
		onSuccess: async (saveRun) => {
			client.setQueryData<FieldGroup[]>(groupsQueryKey, (cached) => groupsWithSaved(cached, group.id, saveRun.saved))
			settleDetails(saveRun, details, client)
			page.fields.ran(Object.keys(saveRun.saved), fieldFailures(saveRun))
			const notes = saveNotes(saveRun)
			toaster.show((notes.notice ?? notes.toast) as string)
			if (saveRun.outcome.done > 0) {
				await Promise.all([
					client.invalidateQueries({ queryKey: groupsQueryKey }),
					client.invalidateQueries({ queryKey: typesQueryKey }),
				])
			}
		},
	})
	return {
		changed: details.changed || changedPaths(group.fields, drafts).length > 0,
		saving: saveMutation.isPending,
		save: () => saveMutation.mutate(),
	}
}
