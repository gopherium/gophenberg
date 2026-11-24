// SPDX-License-Identifier: Apache-2.0

import { settled } from './groupDraft'
import { pairedSettings, settingsOffered, storedSettings, typedSettings } from './settingControls'
import { pairsOf } from './types'
import type { ContentField } from './types'

/** An open field's label, required flag and settings as the reader leaves them, with the field as first read. */
export interface FieldDraft {
	/** The field as read when the draft started, whose stamp a save sends. */
	base: ContentField
	label: string
	required: boolean
	/** The text each setting control holds. */
	typed: Record<string, string>
	/** Every setting the field carries, the ones no control types in among them. */
	settings: Record<string, unknown>
}

/** The drafts of a group's fields, each under the dotted path of its field. */
export type FieldDrafts = Record<string, FieldDraft>

/** One part of a field a draft may change, a setting named by its key. */
export type FieldPart = 'label' | 'required' | `setting:${string}`

/** What a field save sends: the label, the required flag and the whole settings. */
export interface FieldEdit {
	label: string
	required: boolean
	settings: Record<string, unknown>
}

/** A draft moved onto the version saved elsewhere, with the parts where the saved value won over the draft. */
export interface RebasedField {
	draft: FieldDraft
	lost: FieldPart[]
}

/**
 * Returns the part a setting goes under.
 * @param name - The key of the setting.
 * @returns The part name.
 */
export function settingPart(name: string): FieldPart {
	return `setting:${name}`
}

/**
 * Returns the draft an open field starts from, the field as read with its whole settings.
 * @param field - The field as read.
 * @returns The draft.
 */
export function fieldDraftOf(field: ContentField): FieldDraft {
	return {
		base: field,
		label: field.label,
		required: field.required,
		typed: typedSettings(settingsOffered(field), field.settings),
		settings: field.settings,
	}
}

/**
 * Returns what a save of a draft sends, every setting the field carries written back.
 * @param draft - The draft held.
 * @returns The label, the required flag and the whole settings to send.
 */
export function fieldEditOf(draft: FieldDraft): FieldEdit {
	const stored = storedSettings(settingsOffered(draft.base), draft.typed, draft.settings)
	return {
		label: draft.label,
		required: draft.required,
		settings: pairedSettings(draft.base.kind, stored, pairsOf(draft.settings)),
	}
}

/**
 * Reports whether two values of a setting hold the same.
 * @param one - The first value.
 * @param other - The second value.
 * @returns Whether they match.
 */
function sameSetting(one: unknown, other: unknown): boolean {
	return JSON.stringify(one) === JSON.stringify(other)
}

/**
 * Returns the keys of every setting the sides hold that no control types in.
 * @param typed - The keys of the settings the controls type in.
 * @param sides - The settings to read the keys of.
 * @returns The keys, each once.
 */
function heldKeys(typed: string[], sides: Record<string, unknown>[]): string[] {
	const keys = new Set(sides.flatMap((side) => Object.keys(side)))
	return [...keys].filter((key) => !typed.includes(key))
}

/**
 * Returns the settings a draft holds otherwise than the field it started from.
 * @param draft - The draft held.
 * @returns The changed settings, the ones the controls type in first.
 */
function changedSettings(draft: FieldDraft): FieldPart[] {
	const offered = settingsOffered(draft.base)
	const was = typedSettings(offered, draft.base.settings)
	const names = offered.map((setting) => setting.name)
	const typed = names.filter((name) => draft.typed[name] !== was[name])
	const held = heldKeys(names, [draft.base.settings, draft.settings]).filter(
		(key) => !sameSetting(draft.settings[key], draft.base.settings[key]),
	)
	return [...typed, ...held].map(settingPart)
}

/**
 * Returns the parts a draft holds that differ from the field it started from.
 * @param draft - The draft held.
 * @returns The changed parts, none once every change is undone.
 */
export function changedFieldParts(draft: FieldDraft): FieldPart[] {
	const parts: FieldPart[] = []
	if (draft.label !== draft.base.label) {
		parts.push('label')
	}
	if (draft.required !== draft.base.required) {
		parts.push('required')
	}
	return [...parts, ...changedSettings(draft)]
}

/**
 * Returns the text the controls of a draft hold once it moves onto the saved version, setting by setting.
 * @param draft - The draft held.
 * @param saved - The field as saved elsewhere.
 * @param lost - The parts named so far, where a setting the saved value won over goes.
 * @returns The text each control holds.
 */
function rebasedTyped(draft: FieldDraft, saved: ContentField, lost: FieldPart[]): Record<string, string> {
	const offered = settingsOffered(saved)
	const base = typedSettings(offered, draft.base.settings)
	const theirs = typedSettings(offered, saved.settings)
	const typed: Record<string, string> = {}
	for (const { name } of offered) {
		const sides = { base: base[name], mine: draft.typed[name], theirs: theirs[name] }
		typed[name] = settled(settingPart(name), sides, (one, other) => one === other, lost)
	}
	return typed
}

/**
 * Returns the settings of a draft moved onto the saved version, each one no control types in settled on its own.
 * @param draft - The draft held.
 * @param saved - The field as saved elsewhere.
 * @param lost - The parts named so far, where a setting the saved value won over goes.
 * @returns The settings the draft holds.
 */
function rebasedHeld(draft: FieldDraft, saved: ContentField, lost: FieldPart[]): Record<string, unknown> {
	const settings: Record<string, unknown> = { ...saved.settings }
	const typed = settingsOffered(saved).map((setting) => setting.name)
	for (const key of heldKeys(typed, [draft.base.settings, draft.settings, saved.settings])) {
		const sides = { base: draft.base.settings[key], mine: draft.settings[key], theirs: saved.settings[key] }
		const kept = settled(settingPart(key), sides, sameSetting, lost)
		if (kept === undefined) {
			delete settings[key]
		} else {
			settings[key] = kept
		}
	}
	return settings
}

/**
 * Returns the draft moved onto the version saved elsewhere, with the parts where the saved value won.
 * @param draft - The draft held.
 * @param saved - The field as saved elsewhere.
 * @returns The draft standing on the saved version, and the parts where the saved value won.
 */
export function rebasedField(draft: FieldDraft, saved: ContentField): RebasedField {
	const lost: FieldPart[] = []
	const plain = (one: unknown, other: unknown) => one === other
	const moved: FieldDraft = {
		base: saved,
		label: settled('label', { base: draft.base.label, mine: draft.label, theirs: saved.label }, plain, lost),
		required: settled(
			'required',
			{ base: draft.base.required, mine: draft.required, theirs: saved.required },
			plain,
			lost,
		),
		typed: rebasedTyped(draft, saved, lost),
		settings: rebasedHeld(draft, saved, lost),
	}
	return { draft: moved, lost }
}

/**
 * Returns the dotted paths of the fields holding a changed draft in page order, leaving out a path no field stands at.
 * @param fields - The fields as last read.
 * @param drafts - The drafts held.
 * @param above - The dotted path holding the fields, empty at a group's top.
 * @returns The paths.
 */
export function changedPaths(fields: ContentField[], drafts: FieldDrafts, above = ''): string[] {
	return fields.flatMap((field) => {
		const path = above === '' ? field.key : `${above}.${field.key}`
		const draft = drafts[path]
		const own = draft !== undefined && changedFieldParts(draft).length > 0 ? [path] : []
		return [...own, ...changedPaths(field.fields, drafts, path)]
	})
}

/**
 * Reports whether a dotted path names a field or a field inside it.
 * @param held - The dotted path to place.
 * @param path - The dotted path of the field.
 * @returns Whether the path stands at or under the field.
 */
export function atOrUnder(held: string, path: string): boolean {
	return held === path || held.startsWith(`${path}.`)
}

/**
 * Reports whether a field, or a field inside it, holds a changed draft.
 * @param changed - The dotted paths of the fields holding a changed draft.
 * @param path - The dotted path of the field.
 * @returns Whether a changed draft stands at or under the field.
 */
export function holdsDraft(changed: string[], path: string): boolean {
	return changed.some((held) => atOrUnder(held, path))
}
