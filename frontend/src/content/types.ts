// SPDX-License-Identifier: Apache-2.0

import { __, _x } from '@wordpress/i18n'
import { z } from 'zod'

import { errorText } from '../i18n/errors'
import type { Choice } from './select'

/** A field row as the API answers it, holding the sub fields a container declares. */
export interface FieldRow {
	key: string
	label: string
	kind: string
	relates_to?: string
	many: boolean
	required: boolean
	origin?: string
	settings?: Record<string, unknown>
	fields?: FieldRow[]
	updated_at: string
}

export const fieldSchema: z.ZodType<FieldRow> = z.lazy(() =>
	z.object({
		key: z.string(),
		label: z.string(),
		kind: z.string(),
		relates_to: z.string().optional(),
		many: z.boolean(),
		required: z.boolean(),
		origin: z.string().optional(),
		settings: z.record(z.string(), z.unknown()).optional(),
		fields: z.array(fieldSchema).optional(),
		updated_at: z.string(),
	}),
)

const typeSchema = z.object({
	key: z.string(),
	singular_label: z.string(),
	plural_label: z.string(),
	description: z.string(),
	route_word: z.string(),
	derived_route_word: z.string().optional(),
	hierarchical: z.boolean(),
	revisions: z.boolean(),
	revision_cap: z.number(),
	page_kind: z.string(),
	default: z.boolean(),
	active: z.boolean(),
	origin: z.string().optional(),
	fields: z.array(fieldSchema),
	created_at: z.string().optional(),
})

const typeListSchema = z.object({ items: z.array(typeSchema) })

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

/** One typed field a content type declares. */
export interface ContentField {
	key: string
	label: string
	kind: string
	relatesTo: string
	many: boolean
	required: boolean
	origin?: string
	settings: Record<string, unknown>
	fields: ContentField[]
	updatedAt: string
}

/** A content type as the admin reads it. */
export interface ContentType {
	key: string
	singularLabel: string
	pluralLabel: string
	description: string
	routeWord: string
	derivedRouteWord: string
	hierarchical: boolean
	revisions: boolean
	revisionCap: number
	pageKind: string
	isDefault: boolean
	active: boolean
	origin?: string
	fields: ContentField[]
	createdAt?: string
}

/** What registering a type needs. */
export interface NewType {
	key: string
	singularLabel: string
	pluralLabel: string
	description: string
	routeWord: string
}

/** The fields an edit may move, where an absent field is unchanged. */
export interface TypeEdit {
	singularLabel?: string
	pluralLabel?: string
	description?: string
	routeWord?: string
	hierarchical?: boolean
	isDefault?: boolean
	active?: boolean
}

/**
 * Returns the admin view of a stored type.
 * @param row - The type as the API answered it.
 * @returns The type the admin reads.
 */
function toType(row: z.infer<typeof typeSchema>): ContentType {
	return {
		key: row.key,
		singularLabel: row.singular_label,
		pluralLabel: row.plural_label,
		description: row.description,
		routeWord: row.route_word,
		derivedRouteWord: row.derived_route_word ?? '',
		hierarchical: row.hierarchical,
		revisions: row.revisions,
		revisionCap: row.revision_cap,
		pageKind: row.page_kind,
		isDefault: row.default,
		active: row.active,
		origin: row.origin,
		fields: row.fields.map(toField),
		createdAt: row.created_at,
	}
}

/**
 * Returns the admin view of one declared field.
 * @param row - The field as the API answered it.
 * @returns The field the admin reads.
 */
export function toField(row: z.infer<typeof fieldSchema>): ContentField {
	return {
		key: row.key,
		label: row.label,
		kind: row.kind,
		relatesTo: row.relates_to ?? '',
		many: row.many,
		required: row.required,
		origin: row.origin,
		settings: row.settings ?? {},
		fields: (row.fields ?? []).map(toField),
		updatedAt: row.updated_at,
	}
}

/**
 * Throws the reason the registry refused a write, or the status it failed with.
 * @param response - The answer the registry gave.
 */
async function refuse(response: Response): Promise<never> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	throw new Error(errorText(parsed.success ? parsed.data : { error: '' }))
}

/**
 * Sends a write to the registry and returns the answer it accepted.
 * @param url - The address of the write.
 * @param init - The method and body of the write.
 * @returns The answer, or throws the reason it was refused, or that the registry could not be reached.
 */
async function accepted(url: string, init: RequestInit): Promise<Response> {
	const response = await fetch(url, init).catch(() => null)
	if (response === null) {
		throw new Error(__('The registry could not be reached.', 'gophenberg'))
	}
	if (!response.ok) {
		await refuse(response)
	}
	return response
}

/**
 * Returns every registered content type, active or not.
 * @returns The registry in registration order.
 */
export async function listTypes(): Promise<ContentType[]> {
	const response = await fetch('/api/types')
	if (!response.ok) {
		throw new Error(`reading the content types failed with status ${response.status}`)
	}
	return typeListSchema.parse(await response.json()).items.map(toType)
}

/**
 * Registers a content type.
 * @param asked - The key, labels, description and route word to register.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored type.
 */
export async function createType(asked: NewType, signal?: AbortSignal): Promise<ContentType> {
	const response = await accepted('/api/types', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			key: asked.key,
			singular_label: asked.singularLabel,
			plural_label: asked.pluralLabel,
			description: asked.description,
			route_word: asked.routeWord,
		}),
		signal,
	})
	return toType(typeSchema.parse(await response.json()))
}

/**
 * Edits the fields the caller named.
 * @param key - The type to edit.
 * @param edit - The fields to move.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored type.
 */
export async function updateType(key: string, edit: TypeEdit, signal?: AbortSignal): Promise<ContentType> {
	const body: Record<string, unknown> = {}
	if (edit.singularLabel !== undefined) {
		body.singular_label = edit.singularLabel
	}
	if (edit.pluralLabel !== undefined) {
		body.plural_label = edit.pluralLabel
	}
	if (edit.description !== undefined) {
		body.description = edit.description
	}
	if (edit.routeWord !== undefined) {
		body.route_word = edit.routeWord
	}
	if (edit.hierarchical !== undefined) {
		body.hierarchical = edit.hierarchical
	}
	if (edit.isDefault !== undefined) {
		body.default = edit.isDefault
	}
	if (edit.active !== undefined) {
		body.active = edit.active
	}
	const response = await accepted(`/api/types/${encodeURIComponent(key)}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body),
		signal,
	})
	return toType(typeSchema.parse(await response.json()))
}

/**
 * Removes a content type that holds nothing.
 * @param key - The type to remove.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 */
export async function deleteType(key: string, signal?: AbortSignal): Promise<void> {
	await accepted(`/api/types/${encodeURIComponent(key)}`, { method: 'DELETE', signal })
}

/**
 * Returns the kinds a field may be declared as, in the order the admin offers them.
 * @returns The kinds, each under the label the admin shows.
 */
function fieldKinds(): Choice[] {
	return [
		{ label: __('Text', 'gophenberg'), value: 'text' },
		{ label: __('Text area', 'gophenberg'), value: 'textarea' },
		{ label: __('Email', 'gophenberg'), value: 'email' },
		{ label: __('Web address', 'gophenberg'), value: 'url' },
		{ label: __('Number', 'gophenberg'), value: 'number' },
		{ label: _x('Range', 'field type', 'gophenberg'), value: 'range' },
		{ label: __('Yes or no', 'gophenberg'), value: 'boolean' },
		{ label: _x('Date', 'field type', 'gophenberg'), value: 'date' },
		{ label: _x('Select', 'field type', 'gophenberg'), value: 'select' },
		{ label: __('Radio group', 'gophenberg'), value: 'radio' },
		{ label: __('Checkbox group', 'gophenberg'), value: 'checkboxes' },
		{ label: __('Button group', 'gophenberg'), value: 'buttons' },
		{ label: _x('Media', 'field type', 'gophenberg'), value: 'media' },
		{ label: _x('Gallery', 'field type', 'gophenberg'), value: 'gallery' },
		{ label: __('Relation', 'gophenberg'), value: 'relation' },
		{ label: __('Linked from', 'gophenberg'), value: 'backlinks' },
		{ label: _x('Link', 'field type', 'gophenberg'), value: 'link' },
		{ label: _x('Color', 'field type', 'gophenberg'), value: 'color' },
		{ label: _x('Section', 'field type', 'gophenberg'), value: 'section' },
		{ label: _x('Repeater', 'field type', 'gophenberg'), value: 'repeater' },
		{ label: _x('Flexible content', 'field type', 'gophenberg'), value: 'flexible' },
		{ label: _x('Layout', 'field type', 'gophenberg'), value: 'layout' },
	]
}

/**
 * Reports whether a field of this kind holds a value of its own that publishing can wait for.
 * @param kind - The kind the field holds.
 * @returns Whether the field can be required.
 */
export function requirable(kind: string): boolean {
	return kind !== 'backlinks' && kind !== 'layout'
}

/**
 * Reports whether a field of this kind may stand inside the container.
 * @param kind - The kind the field holds.
 * @param parent - The kind the container holds, absent at the top of a group.
 * @returns Whether the field may stand there.
 */
export function standsInside(kind: string, parent?: string): boolean {
	if (parent === 'flexible') {
		return kind === 'layout'
	}
	if (parent !== undefined) {
		return kind !== 'layout' && kind !== 'backlinks'
	}
	return kind !== 'layout'
}

/**
 * Returns the kinds a field may take inside the container it is declared under.
 * @param parent - The kind the container holds, absent at the top of a group.
 * @returns The kinds to offer.
 */
export function kindsInside(parent?: string): Choice[] {
	return fieldKinds().filter((held) => standsInside(held.value, parent))
}

/**
 * Reports whether a field of this kind declares fields inside it.
 * @param kind - The kind the field holds.
 * @returns Whether the kind holds sub fields.
 */
export function holdsFields(kind: string): boolean {
	return kind === 'section' || kind === 'repeater' || kind === 'flexible' || kind === 'layout'
}

/**
 * Reports whether a field of this kind holds rows its settings bound.
 * @param kind - The kind the field holds.
 * @returns Whether the kind takes a row count.
 */
export function holdsRows(kind: string): boolean {
	return kind === 'repeater' || kind === 'flexible' || kind === 'layout'
}

/** One choice a field offers: the value it stores and the label it shows. */
export interface ChoicePair {
	value: string
	label: string
}

/**
 * Returns the value and label pairs a field's choices setting holds.
 * @param settings - The settings the field carries.
 * @returns The pairs, empty when the field lists none.
 */
export function pairsOf(settings: Record<string, unknown>): ChoicePair[] {
	const listed = Array.isArray(settings.choices) ? settings.choices : []
	return listed.flatMap((pair) => {
		if (typeof pair !== 'object' || pair === null) {
			return []
		}
		const { value, label } = pair as Record<string, unknown>
		return typeof value === 'string' && typeof label === 'string' ? [{ value, label }] : []
	})
}

/** What a picker entry declares: the kind, whether it holds many, and the settings it starts with. */
export interface PickedKind {
	kind: string
	many: boolean
	settings?: Record<string, unknown>
}

/**
 * Returns the kind and the starting settings the picker entry declares.
 * @param value - The picker entry chosen.
 * @returns The declaration the entry stands for.
 */
export function pickedKind(value: string): PickedKind {
	const settled: Record<string, PickedKind> = {
		textarea: { kind: 'text', many: false, settings: { variant: 'textarea' } },
		email: { kind: 'text', many: false, settings: { variant: 'email' } },
		url: { kind: 'text', many: false, settings: { variant: 'url' } },
		range: { kind: 'number', many: false, settings: { presentation: 'range' } },
		select: { kind: 'choice', many: false, settings: { presentation: 'select' } },
		radio: { kind: 'choice', many: false, settings: { presentation: 'radio' } },
		checkboxes: {
			kind: 'choice',
			many: false,
			settings: { presentation: 'checkbox', multiple: true },
		},
		buttons: { kind: 'choice', many: false, settings: { presentation: 'buttons' } },
		gallery: { kind: 'media', many: true },
		color: { kind: 'text', many: false, settings: { variant: 'color' } },
	}
	return settled[value] ?? { kind: value, many: false }
}

/** What a stored field holds that tells which picker entry made it. */
type StoredKind = Pick<ContentField, 'kind' | 'many' | 'settings'>

/** The starting settings that tell one picker entry from another of the same kind. */
const TELLING_SETTINGS = ['variant', 'presentation']

/**
 * Returns the starting settings of a picker entry that tell it from another entry of the same kind.
 * @param picked - What the picker entry declares.
 * @returns The variant and the presentation the entry starts with, as name and value pairs.
 */
function tellingSettings(picked: PickedKind): [string, unknown][] {
	return Object.entries(picked.settings ?? {}).filter(([name]) => TELLING_SETTINGS.includes(name))
}

/**
 * Reports whether a stored field stands as a picker entry declares a field.
 * @param picked - What the picker entry declares.
 * @param field - The field as the registry stored it.
 * @returns Whether the field holds the entry's kind, the many flag it sets, its variant and its presentation.
 */
function standsAs(picked: PickedKind, field: StoredKind): boolean {
	return (
		picked.kind === field.kind
		&& (field.many || !picked.many)
		&& tellingSettings(picked).every(([name, value]) => field.settings[name] === value)
	)
}

/**
 * Returns how much of a field a picker entry sets beyond its kind.
 * @param picked - What the picker entry declares.
 * @returns The count of telling settings, one more when the entry holds many.
 */
function weightOf(picked: PickedKind): number {
	return tellingSettings(picked).length + Number(picked.many)
}

/**
 * Returns the label of the picker entry a stored field was made from, the reverse of pickedKind.
 * @param field - The field as the registry stored it.
 * @returns The label the picker showed, the stored kind when no entry makes such a field.
 */
export function pickedLabel(field: StoredKind): string {
	const [made] = fieldKinds()
		.map((entry) => ({ label: entry.label, picked: pickedKind(entry.value) }))
		.filter((entry) => standsAs(entry.picked, field))
		.sort((one, other) => weightOf(other.picked) - weightOf(one.picked))
	if (made !== undefined) {
		return made.label
	}
	return field.kind === 'choice' ? _x('Choice', 'field type', 'gophenberg') : field.kind
}

/**
 * Returns the key a label reduces to.
 * @param label - The name the operator typed.
 * @returns The slug the label reduces to.
 */
export function slugifyKey(label: string): string {
	return label
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
}

/** What declaring a field needs. */
export interface NewField {
	key: string
	label: string
	kind: string
	relatesTo?: string
	many?: boolean
	required?: boolean
	settings?: Record<string, unknown>
}

