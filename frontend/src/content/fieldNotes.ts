// SPDX-License-Identifier: Apache-2.0

import { groupErrorMessage, RejectedWriteError, StaleWriteError } from './groups'
import type { FieldDraft, FieldDrafts, FieldPart } from './fieldDraft'
import type { ContentField } from './types'

/** The server's words on a rejected save of a field, the setting they name, and what its control held when sent. */
export interface Rejection {
	words: string
	/** The key of the setting the server named, empty when it named none. */
	setting: string
	/** The text the control of that setting held when the save went. */
	sent: string
}

/** What an open field shows of its saves: someone saved it first, why its last save was rejected, the parts it lost. */
export interface FieldNote {
	/** Whether someone saved the field after its draft began. */
	stale: boolean
	rejected?: Rejection
	/** The parts where the saved version won over the draft as the draft moved onto it. */
	lost: FieldPart[]
}

/** The notes of a group's fields, each under the dotted path of its field. */
export type FieldNotes = Record<string, FieldNote>

/** One field the header Save could not store, by its dotted path, and what its save failed with. */
export interface FieldFailure {
	path: string
	error: unknown
}

/**
 * Returns what is held under each dotted path less the paths a test drops.
 * @param held - What is held, under each dotted path.
 * @param drop - Whether a path goes.
 * @returns What is left.
 */
export function withoutPaths<T>(held: Record<string, T>, drop: (path: string) => boolean): Record<string, T> {
	return Object.fromEntries(Object.entries(held).filter(([path]) => !drop(path)))
}

/**
 * Returns the field standing at a dotted path among the fields of a group.
 * @param fields - The fields at the group's top.
 * @param path - The dotted path of the field.
 * @returns The field, none while no field stands there.
 */
export function fieldAt(fields: ContentField[], path: string): ContentField | undefined {
	const [key, ...rest] = path.split('.')
	const field = fields.find((held) => held.key === key)
	return field === undefined || rest.length === 0 ? field : fieldAt(field.fields, rest.join('.'))
}

/**
 * Returns the key of the setting a rejected save names.
 * @param error - What the save failed with.
 * @returns The key, empty when the answer named none.
 */
function settingNamed(error: unknown): string {
	const named = error instanceof RejectedWriteError ? error.meta.setting : undefined
	return typeof named === 'string' ? named : ''
}

/**
 * Returns the note a failed save of a field leaves.
 * @param error - What the save failed with.
 * @param draft - The draft the save sent.
 * @param lost - The parts the field lost as its draft last moved onto a saved version.
 * @returns The note: someone saved the field first, or the server's words and the setting they name.
 */
function failedNote(error: unknown, draft: FieldDraft | undefined, lost: FieldPart[]): FieldNote {
	if (error instanceof StaleWriteError) {
		return { stale: true, lost }
	}
	const setting = settingNamed(error)
	const sent = draft?.typed[setting] ?? ''
	return { stale: false, lost, rejected: { words: groupErrorMessage(error), setting, sent } }
}

/**
 * Returns the notes as a header Save starts, each keeping only the parts its field lost.
 * @param notes - The notes held.
 * @returns The notes to hold while the save runs.
 */
export function notesStarted(notes: FieldNotes): FieldNotes {
	return Object.fromEntries(Object.entries(notes).map(([path, note]) => [path, { stale: false, lost: note.lost }]))
}

/**
 * Returns the notes once a header Save ended: none for a field it saved, why for each field it could not.
 * @param notes - The notes held.
 * @param saved - The dotted paths of the fields the run saved.
 * @param failures - The fields the run could not save and what each failed with.
 * @param drafts - The drafts the run sent.
 * @returns The notes to hold.
 */
export function notesRan(
	notes: FieldNotes,
	saved: string[],
	failures: FieldFailure[],
	drafts: FieldDrafts,
): FieldNotes {
	const kept = withoutPaths(notes, (path) => saved.includes(path))
	for (const { path, error } of failures) {
		kept[path] = failedNote(error, drafts[path], notes[path]?.lost ?? [])
	}
	return kept
}

/**
 * Returns the fields standing open once a header Save ended: each one it saved closed, each one it could not open.
 * @param opened - The dotted paths of the fields standing open.
 * @param saved - The dotted paths of the fields the run saved.
 * @param failures - The fields the run could not save.
 * @returns The paths to hold.
 */
export function openedRan(opened: ReadonlySet<string>, saved: string[], failures: FieldFailure[]): ReadonlySet<string> {
	const next = new Set([...opened].filter((path) => !saved.includes(path)))
	for (const { path } of failures) {
		next.add(path)
	}
	return next
}
