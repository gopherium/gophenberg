// SPDX-License-Identifier: Apache-2.0

import { DataForm } from '@gophenberg/frontend-sdk/dataviews'
import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import { useMemo } from 'react'

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
	declared,
	buffer,
}: {
	declared: ContentField[]
	buffer: EditorBuffer
}) {
	const rendered = useMemo(() => editableFields(declared), [declared])
	const descriptors = useMemo(() => fieldDescriptors(rendered), [rendered])
	if (rendered.length === 0) {
		return null
	}
	return (
		<DataForm
			data={buffer.fields}
			fields={descriptors}
			form={{ fields: rendered.map((field) => field.key) }}
			onChange={(edits: FieldValues) =>
				buffer.setFields({ ...buffer.fields, ...clearedEdits(edits) })
			}
		/>
	)
}
