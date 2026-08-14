// SPDX-License-Identifier: Apache-2.0

import { Stack } from '@gophenberg/frontend-sdk'
import { DataForm } from '@gophenberg/frontend-sdk/dataviews'
import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import { useMemo } from 'react'

import { MediaField, mediaHeld } from './MediaField'
import { RelationPicker, targetsHeld } from './RelationPicker'
import type { ContentField } from './types'
import type { EditorBuffer } from './useEditorBuffer'

/** The values the panel edits, keyed by field key. */
type FieldValues = Record<string, unknown>

/** The DataViews type a declared kind is edited as, absent for the kinds a picker owns. */
const TYPES: Record<string, string> = {
	text: 'text',
	number: 'number',
	boolean: 'boolean',
	date: 'date',
}

/**
 * Returns the fields the panel edits, which are the ones a control covers.
 * @param declared - The fields the type declares.
 * @returns The declared fields this panel renders.
 */
export function editableFields(declared: ContentField[]): ContentField[] {
	return declared.filter((field) => TYPES[field.kind] !== undefined)
}

/**
 * Returns the fields a relation picker edits.
 * @param declared - The fields the type declares.
 * @returns The declared relation fields.
 */
export function relationFields(declared: ContentField[]): ContentField[] {
	return declared.filter((field) => field.kind === 'relation')
}

/**
 * Returns the fields the media library edits.
 * @param declared - The fields the type declares.
 * @returns The declared media fields.
 */
export function mediaFields(declared: ContentField[]): ContentField[] {
	return declared.filter((field) => field.kind === 'media')
}

/**
 * Returns the DataViews field descriptors for the declared fields.
 * @param declared - The fields the panel renders.
 * @returns The descriptors DataForm reads.
 */
export function fieldDescriptors(declared: ContentField[]): Field<FieldValues>[] {
	return declared.map((field) => ({
		id: field.key,
		label: field.label,
		type: TYPES[field.kind],
		isValid: { required: field.required },
		enableSorting: false,
	})) as Field<FieldValues>[]
}

/**
 * Returns the edits with every emptied control written as the value that clears its key.
 * @param edits - The values the control reported.
 * @returns The edits as the write path reads them.
 */
export function clearedEdits(edits: FieldValues): FieldValues {
	return Object.fromEntries(
		Object.entries(edits).map(([key, value]) => [
			key,
			value === undefined || value === '' ? null : value,
		]),
	)
}

/**
 * Renders the panel editing the values of a type's declared fields.
 * @param props - The buffer the panel drives and the fields the type declares.
 * @returns The panel element, or nothing when the type declares none.
 */
export function FieldsPanel({
	postId,
	declared,
	buffer,
}: {
	postId: string
	declared: ContentField[]
	buffer: EditorBuffer
}) {
	const rendered = useMemo(() => editableFields(declared), [declared])
	const related = useMemo(() => relationFields(declared), [declared])
	const pictured = useMemo(() => mediaFields(declared), [declared])
	const descriptors = useMemo(() => fieldDescriptors(rendered), [rendered])
	if (rendered.length === 0 && related.length === 0 && pictured.length === 0) {
		return null
	}
	return (
		<Stack direction="column" gap="md">
			{rendered.length > 0 && (
				<DataForm
					data={buffer.fields}
					fields={descriptors}
					form={{ fields: rendered.map((field) => field.key) }}
					onChange={(edits: FieldValues) =>
						buffer.setFields({ ...buffer.fields, ...clearedEdits(edits) })
					}
				/>
			)}
			{related.map((field) => (
				<RelationPicker
					key={field.key}
					field={field}
					postId={postId}
					targets={targetsHeld(buffer.fields[field.key])}
					onChange={(targets) => buffer.setFields({ ...buffer.fields, [field.key]: targets })}
				/>
			))}
			{pictured.map((field) => (
				<MediaField
					key={field.key}
					field={field}
					value={mediaHeld(buffer.fields[field.key])}
					onChange={(value) => buffer.setFields({ ...buffer.fields, [field.key]: value })}
				/>
			))}
		</Stack>
	)
}
