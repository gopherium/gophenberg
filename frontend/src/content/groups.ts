// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'
import { __, sprintf } from '@wordpress/i18n'
import type { DialogAnswer } from '@gopherium/godmin'

import { unlessTimedOut } from '../dialogAnswer'
import { errorText } from '../i18n/errors'
import { fieldSchema, toField } from './types'
import type { ContentField, NewField } from './types'
import type { Choice } from './select'

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
	origin: z.string().optional(),
	fields: z.array(fieldSchema),
	created_at: z.string(),
	updated_at: z.string(),
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
	key: string
	title: string
	location: Location
	position: number
	active: boolean
	origin?: string
	fields: ContentField[]
	createdAt: string
	updatedAt: string
}

/** The source a backlinks field reads, with the groups and relations on offer. */
export interface BacklinkSources {
	groups: Choice[]
	fields: Choice[]
	group: Choice
	field: Choice
}

/** The choice standing for a source nothing was picked for. */
const noSource: Choice = { label: '', value: '' }

/**
 * Reports whether the field points at content of another type.
 * @param field - The field to weigh.
 * @returns Whether it is a relation.
 */
function relates(field: ContentField): boolean {
	return field.kind === 'relation'
}

/**
 * Returns the groups and relations a backlinks field may read, and the pair it stands on.
 * @param groups - The groups the admin declared.
 * @param group - The group key picked, empty for the first on offer.
 * @param field - The relation key picked, empty for the first on offer.
 * @returns The choices both pickers offer and the pair held.
 */
export function backlinkSources(groups: FieldGroup[], group: string, field: string): BacklinkSources {
	const relating = groups.filter((held) => held.fields.some(relates))
	const offered = relating.map((held) => ({ label: held.title, value: held.key }))
	const held = relating.find((one) => one.key === group) ?? relating[0]
	const fields = (held?.fields ?? []).filter(relates).map((one) => ({ label: one.label, value: one.key }))
	return {
		groups: offered,
		fields,
		group: offered.find((one) => one.value === group) ?? offered[0] ?? noSource,
		field: fields.find((one) => one.value === field) ?? fields[0] ?? noSource,
	}
}

/**
 * Returns the settings naming the relation a backlinks field reads.
 * @param sources - The pair the pickers stand on.
 * @returns The settings to send.
 */
export function backlinkSettings(sources: BacklinkSources): Record<string, unknown> {
	return { source_group: sources.group.value, source_field: [sources.field.value] }
}

/** The group and the relation a backlinks field reads, by their keys. */
export interface BacklinkPick {
	group: string
	field: string
}

/**
 * Returns the pair a backlinks field reads as its pickers start on it, or nothing for a relation inside a container.
 * @param field - The backlinks field.
 * @returns The group key and the relation key, or null when the path runs through a container.
 */
export function pickOf(field: ContentField): BacklinkPick | null {
	const group = field.settings.source_group
	const path = Array.isArray(field.settings.source_field) ? field.settings.source_field : []
	if (path.length > 1) {
		return null
	}
	const [first] = path
	return {
		group: typeof group === 'string' ? group : '',
		field: typeof first === 'string' ? first : '',
	}
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
	backlinks?: Record<string, Record<string, unknown>>
}

/** The group an edit is for, with the timestamp the editor read it at. */
type GroupRead = Pick<FieldGroup, 'id' | 'updatedAt'>

/**
 * Returns the admin view of a stored group.
 * @param row - The group as the API answered it.
 * @returns The group the admin reads.
 */
function toGroup(row: z.infer<typeof groupSchema>): FieldGroup {
	return {
		id: row.id,
		key: row.key,
		title: row.title,
		location: row.location,
		position: row.position,
		active: row.active,
		origin: row.origin,
		fields: row.fields.map(toField),
		createdAt: row.created_at,
		updatedAt: row.updated_at,
	}
}

/** A write the server rejected, with the code and the data its answer named. */
export class RejectedWriteError extends Error {
	/** The code the server named, empty when it named none. */
	readonly code: string
	/** The data the answer carried, such as the setting it names. */
	readonly meta: Record<string, unknown>

	/**
	 * Holds the words for the reader with the code and the data the server named.
	 * @param message - The words for the reader.
	 * @param rejected - The code and the data the answer carried.
	 */
	constructor(message: string, rejected: Pick<z.infer<typeof errorSchema>, 'code' | 'meta'>) {
		super(message)
		this.code = rejected.code ?? ''
		this.meta = rejected.meta ?? {}
	}
}

/** The failure a write reports when the stored field moved on before it landed. */
export class StaleWriteError extends RejectedWriteError {}

/**
 * Returns the sentence saying someone saved the group after it was read.
 * @param title - The title of the group.
 * @returns The sentence to show.
 */
export function staleGroupMessage(title: string): string {
	return sprintf(__('Someone saved %(group)s after you opened it.', 'gophenberg'), { group: title })
}

/**
 * Returns the sentence saying someone saved a field after its draft began.
 * @param label - The label of the field.
 * @returns The sentence to show.
 */
export function staleFieldMessage(label: string): string {
	return sprintf(__('Someone saved %(field)s after you opened it.', 'gophenberg'), { field: label })
}

/**
 * Returns what a form over a group's fields answers once its write settles.
 * @param write - The write the form sent.
 * @param said - The sentence naming what the write did.
 * @param reporter - What reports a landed write, and a stale one, where the form was opened from.
 * @param signal - The signal the form handed its write, which hands a failure back once the time limit ran out.
 * @returns Nothing once the write landed or went stale, so the form closes, or the words any other failure shows.
 */
export function formAnswer(
	write: Promise<unknown>,
	said: string,
	reporter: { onDone: (said: string) => Promise<void>; onRefused: (cause: unknown) => Promise<void> },
	signal: AbortSignal,
): Promise<DialogAnswer | undefined> {
	return write.then(
		() => {
			void reporter.onDone(said)
			return undefined
		},
		(cause: unknown) =>
			unlessTimedOut(signal, cause, async () => {
				if (!(cause instanceof StaleWriteError)) {
					return { error: groupErrorMessage(cause) }
				}
				await reporter.onRefused(cause)
				return undefined
			}),
	)
}

/**
 * Throws the reason a group write was rejected, with the code and the data the server named.
 * @param response - The answer the server gave.
 */
async function refuse(response: Response): Promise<never> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	const rejected = parsed.success ? parsed.data : { error: '' }
	const said = errorText(rejected)
	throw response.status === 409 ? new StaleWriteError(said, rejected) : new RejectedWriteError(said, rejected)
}

/**
 * Returns the sentence saying no answer came from the field groups.
 * @returns The sentence to show.
 */
export function groupsUnreached(): string {
	return __('The field groups could not be reached.', 'gophenberg')
}

/**
 * Returns the sentence naming why a group write was turned away.
 * @param cause - What the write failed with.
 * @returns The sentence to show.
 */
export function groupErrorMessage(cause: unknown): string {
	return cause instanceof Error ? cause.message : groupsUnreached()
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
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored group.
 */
export async function createGroup(title: string, location: Location, signal?: AbortSignal): Promise<FieldGroup> {
	const response = await fetch('/api/groups', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ title, location }),
		signal,
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toGroup(groupSchema.parse(await response.json()))
}

/**
 * Edits the parts of a group the caller names, while the group still holds the stamp it was read at.
 * @param read - The group to edit, with the timestamp the editor read.
 * @param edit - The parts to move.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored group.
 */
export async function updateGroup(read: GroupRead, edit: GroupEdit, signal?: AbortSignal): Promise<FieldGroup> {
	const body: Record<string, unknown> = { updated_at: read.updatedAt }
	if (edit.title !== undefined) {
		body.title = edit.title
	}
	if (edit.location !== undefined) {
		body.location = edit.location
	}
	if (edit.active !== undefined) {
		body.active = edit.active
	}
	if (edit.backlinks !== undefined) {
		body.backlinks = edit.backlinks
	}
	const response = await fetch(`/api/groups/${read.id}`, {
		method: 'PATCH',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
		signal,
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toGroup(groupSchema.parse(await response.json()))
}

/**
 * Removes a group with its fields and the values they held.
 * @param id - The group to remove.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 */
export async function deleteGroup(id: number, signal?: AbortSignal): Promise<void> {
	const response = await fetch(`/api/groups/${id}`, { method: 'DELETE', signal })
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
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored field.
 */
export async function createFieldInGroup(id: number, asked: NewField, signal?: AbortSignal): Promise<ContentField> {
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
		signal,
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toField(fieldSchema.parse(await response.json()))
}

/** The field an edit is for, in the group declaring it, with the timestamp the editor read it at. */
interface FieldRead {
	/** The group declaring the field. */
	group: number
	/** The field, or the dotted path of a field inside a container. */
	key: string
	/** The timestamp the editor read. */
	updatedAt: string
}

/**
 * Returns the field an edit is for, by its dotted path, as the editor read it.
 * @param place - The group declaring the field, and the dotted path of the field, its own key at the group's top.
 * @param field - The field as the editor read it.
 * @returns The field to edit.
 */
export function fieldRead(place: { group: number; path: string }, field: ContentField): FieldRead {
	return { group: place.group, key: place.path, updatedAt: field.updatedAt }
}

/** The parts of a field an edit may move, where an absent part is unchanged and settings replace the ones held. */
interface FieldPatch {
	label?: string
	required?: boolean
	settings?: Record<string, unknown>
}

/**
 * Carries an edit to a field inside its group in one request, while the field still holds the stamp it was read at.
 * @param read - The field to edit, with the timestamp the editor read.
 * @param parts - The label, the required flag and the settings to move, each left out to keep it.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored field.
 */
export async function editFieldInGroup(
	read: FieldRead,
	parts: FieldPatch,
	signal?: AbortSignal,
): Promise<ContentField> {
	const response = await fetch(`/api/groups/${read.group}/fields/${read.key}`, {
		method: 'PATCH',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({
			label: parts.label,
			required: parts.required,
			settings: parts.settings,
			updated_at: read.updatedAt,
		}),
		signal,
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toField(fieldSchema.parse(await response.json()))
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
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 */
export async function deleteFieldInGroup(id: number, key: string, signal?: AbortSignal): Promise<void> {
	const response = await fetch(`/api/groups/${id}/fields/${key}`, { method: 'DELETE', signal })
	if (!response.ok) {
		await refuse(response)
	}
}

/**
 * Declares a field inside the container the path addresses.
 * @param id - The group declaring the container.
 * @param path - The dotted keys addressing the container.
 * @param asked - The field to declare.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored field.
 */
export async function createSubField(
	id: number,
	path: string,
	asked: NewField,
	signal?: AbortSignal,
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
		signal,
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
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 */
export async function deleteSubField(id: number, path: string, signal?: AbortSignal): Promise<void> {
	const response = await fetch(`/api/groups/${id}/inside/${path}`, { method: 'DELETE', signal })
	if (!response.ok) {
		await refuse(response)
	}
}

/** Where a field carried to another place lands. */
interface FieldLanding {
	/** The group the field lands in. */
	group: number
	/** The dotted path of the container it lands in, empty or none for the group's top. */
	parent?: string
}

/**
 * Carries a field to the top of a group, or inside one of its containers.
 * @param id - The group the field leaves.
 * @param path - The dotted path of the field to carry.
 * @param to - The group and the container the field lands in.
 * @param signal - The signal that stops the request, none for a write that waits for its answer.
 * @returns The stored field.
 */
export async function moveField(
	id: number,
	path: string,
	to: FieldLanding,
	signal?: AbortSignal,
): Promise<ContentField> {
	const { group, parent = '' } = to
	const body: Record<string, unknown> = { to_group: group }
	if (parent !== '') {
		body.to_parent = parent
	}
	const response = await fetch(`/api/groups/${id}/fields/${path}/move`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
		signal,
	})
	if (!response.ok) {
		await refuse(response)
	}
	return toField(fieldSchema.parse(await response.json()))
}
