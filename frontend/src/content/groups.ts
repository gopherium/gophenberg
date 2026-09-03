// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'
import { __ } from '@wordpress/i18n'

import { errorText } from '../i18n/errors'
import { fieldSchema, toField } from './types'
import type { ContentField, NewField } from './types'

const ruleSchema = z.object({
	source: z.string(),
	operator: z.string(),
	value: z.string(),
})

const groupSchema = z.object({
	id: z.number(),
	key: z.string(),
	title: z.string(),
	location: z.array(z.array(ruleSchema)),
	position: z.number(),
	active: z.boolean(),
	fields: z.array(fieldSchema),
})

const groupListSchema = z.object({ items: z.array(groupSchema) })

const sourceSchema = z.object({
	source: z.string(),
	operators: z.array(z.string()),
	values: z.array(z.object({ value: z.string(), label: z.string() })),
})

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

/** One condition of a group's location, read by its source. */
export interface LocationRule {
	source: string
	operator: string
	value: string
}

/** Where a group appears, as OR groups of AND rules. */
export type Location = LocationRule[][]

/** A field group as the admin reads it. */
export interface FieldGroup {
	id: number
	title: string
	location: Location
	position: number
	active: boolean
	fields: ContentField[]
}

/** One source a location rule may read, with the choices it offers. */
export interface RuleSource {
	source: string
	operators: string[]
	values: { value: string; label: string }[]
}

/** The parts of a group an edit may move, where an absent part is unchanged. */
export interface GroupEdit {
	title?: string
	location?: Location
	active?: boolean
}

/**
 * Returns the admin view of a stored group.
 * @param row - The group as the API answered it.
 * @returns The group the admin reads.
 */
function toGroup(row: z.infer<typeof groupSchema>): FieldGroup {
	return {
		id: row.id,
		title: row.title,
		location: row.location,
		position: row.position,
		active: row.active,
		fields: row.fields.map(toField),
	}
}

/** The failure a write reports when the stored field moved on before it landed. */
export class StaleWriteError extends Error {}

/**
 * Throws the reason a group write was refused, or the status it failed with.
 * @param response - The answer the server gave.
 */
async function refuse(response: Response): Promise<never> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	const said = errorText(parsed.success ? parsed.data : { error: '' })
	throw response.status === 409 ? new StaleWriteError(said) : new Error(said)
}

/**
 * Returns the sentence naming why a group write was turned away.
 * @param cause - What the write failed with.
 * @returns The sentence to show.
 */
export function groupErrorMessage(cause: unknown): string {
	return cause instanceof Error ? cause.message : __('The field groups could not be reached.', 'gophenberg')
}

/** The key the stored field groups are cached under. */
export const groupsQueryKey = ['field-groups']

/** The key the rule sources are cached under. */
export const ruleSourcesQueryKey = ['rule-sources']

/** The source a rule reads to match the content type. */
export const typeSource = 'content_type'

/** The value a rule carries to match every content type. */
export const anyType = '*'

/**
 * Returns every stored field group in position order.
 * @returns The groups with their rules and fields.
 */
export async function listGroups(): Promise<FieldGroup[]> {
	const response = await fetch('/api/groups')
	if (!response.ok) {
		throw new Error(`reading the field groups failed with status ${response.status}`)
	}
	return groupListSchema.parse(await response.json()).items.map(toGroup)
}

/**
 * Stores a new field group.
 * @param title - The name the group is listed under.
 * @param location - The rules deciding where its fields appear.
 * @returns The stored group.
 */
export async function createGroup(title: string, location: Location): Promise<FieldGroup> {
	const response = await fetch('/api/groups', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ title, location }),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toGroup(groupSchema.parse(await response.json()))
}

/**
 * Edits the parts of a group the caller names.
 * @param id - The group to edit.
 * @param edit - The parts to move.
 * @returns The stored group.
 */
export async function updateGroup(id: number, edit: GroupEdit): Promise<FieldGroup> {
	const body: Record<string, unknown> = {}
	if (edit.title !== undefined) {
		body.title = edit.title
	}
	if (edit.location !== undefined) {
		body.location = edit.location
	}
	if (edit.active !== undefined) {
		body.active = edit.active
	}
	const response = await fetch(`/api/groups/${id}`, {
		method: 'PATCH',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toGroup(groupSchema.parse(await response.json()))
}

/**
 * Removes a group with its fields and the values they held.
 * @param id - The group to remove.
 */
export async function deleteGroup(id: number): Promise<void> {
	const response = await fetch(`/api/groups/${id}`, { method: 'DELETE' })
	if (!response.ok) {
		await refuse(response)
	}
}

/**
 * Stores the order the groups are read in.
 * @param ids - Every stored group in the order to store.
 * @returns The reordered groups.
 */
export async function reorderGroups(ids: number[]): Promise<FieldGroup[]> {
	const response = await fetch('/api/groups/order', {
		method: 'PUT',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ order: ids }),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return groupListSchema.parse(await response.json()).items.map(toGroup)
}

/**
 * Returns the sources a location rule may read, with the choices each offers.
 * @returns The rule sources in registration order.
 */
export async function listRuleSources(): Promise<RuleSource[]> {
	const response = await fetch('/api/groups/params')
	if (!response.ok) {
		throw new Error(`reading the rule sources failed with status ${response.status}`)
	}
	return z.object({ items: z.array(sourceSchema) }).parse(await response.json()).items
}

/**
 * Declares a field inside a group.
 * @param id - The group declaring the field.
 * @param asked - The field to declare.
 * @returns The stored field.
 */
export async function createFieldInGroup(id: number, asked: NewField): Promise<ContentField> {
	const response = await fetch(`/api/groups/${id}/fields`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({
			key: asked.key,
			label: asked.label,
			kind: asked.kind,
			relates_to: asked.relatesTo,
			many: asked.many,
			required: asked.required,
			settings: asked.settings,
		}),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toField(fieldSchema.parse(await response.json()))
}

/**
 * Carries an edit to a field inside its group.
 * @param id - The group declaring the field.
 * @param key - The field to edit.
 * @param body - The parts to move.
 * @returns The stored field.
 */
async function patchFieldInGroup(
	id: number,
	key: string,
	body: Record<string, unknown>,
): Promise<ContentField> {
	const response = await fetch(`/api/groups/${id}/fields/${key}`, {
		method: 'PATCH',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toField(fieldSchema.parse(await response.json()))
}

/**
 * Carries a new label for a field inside its group.
 * @param id - The group declaring the field.
 * @param key - The field to relabel.
 * @param label - The label to carry.
 * @param updatedAt - The timestamp the editor read.
 * @returns The stored field.
 */
export async function renameFieldInGroup(
	id: number,
	key: string,
	label: string,
	updatedAt: string,
): Promise<ContentField> {
	return patchFieldInGroup(id, key, { label, updated_at: updatedAt })
}

/**
 * Stores the settings a field carries, replacing the ones it held.
 * @param id - The group declaring the field.
 * @param key - The field to change.
 * @param settings - The settings to store.
 * @param updatedAt - The timestamp the editor read.
 * @returns The stored field.
 */
export async function setFieldSettingsInGroup(
	id: number,
	key: string,
	settings: Record<string, unknown>,
	updatedAt: string,
): Promise<ContentField> {
	return patchFieldInGroup(id, key, { settings, updated_at: updatedAt })
}

/**
 * Stores whether a field must be filled before its item publishes.
 * @param id - The group declaring the field.
 * @param key - The field to change.
 * @param required - Whether the field gates publishing.
 * @param updatedAt - The timestamp the editor read.
 * @returns The stored field.
 */
export async function setFieldRequiredInGroup(
	id: number,
	key: string,
	required: boolean,
	updatedAt: string,
): Promise<ContentField> {
	return patchFieldInGroup(id, key, { required, updated_at: updatedAt })
}

/**
 * Stores the declaration order of the fields a container holds.
 * @param id - The group declaring the container.
 * @param path - The dotted path naming the container.
 * @param keys - Every field key the container holds, in the order to store.
 * @returns The reordered fields.
 */
export async function reorderSubFields(
	id: number,
	path: string,
	keys: string[],
): Promise<ContentField[]> {
	const response = await fetch(`/api/groups/${id}/inside/${path}/order`, {
		method: 'PUT',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ order: keys }),
	})
	if (!response.ok) {
		await refuse(response)
	}
	const listed = z.object({ items: z.array(fieldSchema) }).parse(await response.json())
	return listed.items.map(toField)
}

/**
 * Stores the declaration order of a group's fields.
 * @param id - The group whose fields are reordered.
 * @param keys - Every declared field key in the order to store.
 * @returns The fields in the stored order.
 */
export async function reorderFieldsInGroup(id: number, keys: string[]): Promise<ContentField[]> {
	const response = await fetch(`/api/groups/${id}/fields/order`, {
		method: 'PUT',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ order: keys }),
	})
	if (!response.ok) {
		await refuse(response)
	}
	const listed = z.object({ items: z.array(fieldSchema) }).parse(await response.json())
	return listed.items.map(toField)
}

/**
 * Removes a field from its group with every value it held.
 * @param id - The group declaring the field.
 * @param key - The field to remove.
 */
export async function deleteFieldInGroup(id: number, key: string): Promise<void> {
	const response = await fetch(`/api/groups/${id}/fields/${key}`, { method: 'DELETE' })
	if (!response.ok) {
		await refuse(response)
	}
}

/**
 * Declares a field inside the container the path addresses.
 * @param id - The group declaring the container.
 * @param path - The dotted keys addressing the container.
 * @param asked - The field to declare.
 * @returns The stored field.
 */
export async function createSubField(
	id: number,
	path: string,
	asked: NewField,
): Promise<ContentField> {
	const response = await fetch(`/api/groups/${id}/fields/${path}`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({
			key: asked.key,
			label: asked.label,
			kind: asked.kind,
			relates_to: asked.relatesTo,
			many: asked.many,
			required: asked.required,
			settings: asked.settings,
		}),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toField(fieldSchema.parse(await response.json()))
}

/**
 * Removes the field the path addresses inside its container, and the values every item held under it.
 * @param id - The group declaring the container.
 * @param path - The dotted keys addressing the field.
 */
export async function deleteSubField(id: number, path: string): Promise<void> {
	const response = await fetch(`/api/groups/${id}/inside/${path}`, { method: 'DELETE' })
	if (!response.ok) {
		await refuse(response)
	}
}

/**
 * Carries a field into another group, keeping the values it holds.
 * @param id - The group the field leaves.
 * @param key - The field to carry.
 * @param toGroup - The group the field lands in.
 * @returns The stored field.
 */
export async function moveField(id: number, key: string, toGroup: number): Promise<ContentField> {
	const response = await fetch(`/api/groups/${id}/fields/${key}/move`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ to_group: toGroup }),
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toField(fieldSchema.parse(await response.json()))
}
