// SPDX-License-Identifier: Apache-2.0

import { __, _x } from '@wordpress/i18n'
import { z } from 'zod'

import { errorText } from '../i18n/errors'
import type { Choice } from './select'

export const fieldSchema = z.object({
	key: z.string(),
	label: z.string(),
	kind: z.string(),
	relates_to: z.string().optional(),
	many: z.boolean(),
	required: z.boolean(),
	settings: z.record(z.string(), z.unknown()).optional(),
})

const typeSchema = z.object({
	key: z.string(),
	singular_label: z.string(),
	plural_label: z.string(),
	route_word: z.string(),
	hierarchical: z.boolean(),
	revisions: z.boolean(),
	revision_cap: z.number(),
	page_kind: z.string(),
	default: z.boolean(),
	active: z.boolean(),
	fields: z.array(fieldSchema).optional(),
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
	settings: Record<string, unknown>
}

/** A content type as the admin reads it. */
export interface ContentType {
	key: string
	singularLabel: string
	pluralLabel: string
	routeWord: string
	hierarchical: boolean
	revisions: boolean
	revisionCap: number
	pageKind: string
	isDefault: boolean
	active: boolean
	fields: ContentField[]
}

/** What registering a type needs. */
export interface NewType {
	key: string
	singularLabel: string
	pluralLabel: string
	routeWord: string
}

/** The fields an edit may move, where an absent field is unchanged. */
export interface TypeEdit {
	singularLabel?: string
	pluralLabel?: string
	routeWord?: string
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
		routeWord: row.route_word,
		hierarchical: row.hierarchical,
		revisions: row.revisions,
		revisionCap: row.revision_cap,
		pageKind: row.page_kind,
		isDefault: row.default,
		active: row.active,
		fields: (row.fields ?? []).map(toField),
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
		settings: row.settings ?? {},
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
 * @param asked - The key, labels and route word to register.
 * @returns The stored type.
 */
export async function createType(asked: NewType): Promise<ContentType> {
	const response = await fetch('/api/types', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			key: asked.key,
			singular_label: asked.singularLabel,
			plural_label: asked.pluralLabel,
			route_word: asked.routeWord,
		}),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toType(typeSchema.parse(await response.json()))
}

/**
 * Edits the fields the caller named.
 * @param key - The type to edit.
 * @param edit - The fields to move.
 * @returns The stored type.
 */
export async function updateType(key: string, edit: TypeEdit): Promise<ContentType> {
	const body: Record<string, unknown> = {}
	if (edit.singularLabel !== undefined) {
		body.singular_label = edit.singularLabel
	}
	if (edit.pluralLabel !== undefined) {
		body.plural_label = edit.pluralLabel
	}
	if (edit.routeWord !== undefined) {
		body.route_word = edit.routeWord
	}
	if (edit.isDefault !== undefined) {
		body.default = edit.isDefault
	}
	if (edit.active !== undefined) {
		body.active = edit.active
	}
	const response = await fetch(`/api/types/${encodeURIComponent(key)}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toType(typeSchema.parse(await response.json()))
}

/**
 * Removes a content type that holds nothing.
 * @param key - The type to remove.
 */
export async function deleteType(key: string): Promise<void> {
	const response = await fetch(`/api/types/${encodeURIComponent(key)}`, { method: 'DELETE' })
	if (!response.ok) {
		await refuse(response)
	}
}

/**
 * Returns the kinds a field may be declared as, in the order the admin offers them.
 * @returns The kinds, each under the label the admin shows.
 */
export function fieldKinds(): Choice[] {
	return [
		{ label: __('Text', 'gophenberg'), value: 'text' },
		{ label: __('Number', 'gophenberg'), value: 'number' },
		{ label: __('Yes or no', 'gophenberg'), value: 'boolean' },
		{ label: _x('Date', 'field type', 'gophenberg'), value: 'date' },
		{ label: _x('Media', 'field type', 'gophenberg'), value: 'media' },
		{ label: __('Relation', 'gophenberg'), value: 'relation' },
	]
}

/**
 * Returns the label a declared field's kind is shown under.
 * @param kind - The kind as the registry stored it.
 * @returns The label to show, the stored kind when the admin offers no name for it.
 */
export function kindLabel(kind: string): string {
	return fieldKinds().find((held) => held.value === kind)?.label ?? kind
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
}

