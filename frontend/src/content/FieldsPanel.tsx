// SPDX-License-Identifier: Apache-2.0

import {
	Button,
	CheckboxControl,
	InputControl,
	RadioControl,
	RangeControl,
	Stack,
	Text,
} from '@gophenberg/frontend-sdk'
import { DataForm } from '@gophenberg/frontend-sdk/dataviews'
import type {
	DataFormControlProps,
	Field,
	FieldValidity,
	FormValidity,
} from '@gophenberg/frontend-sdk/dataviews'
import { __, sprintf } from '@wordpress/i18n'
import { useId, useMemo, useState } from 'react'
import type { ComponentType } from 'react'

import { errorTemplates } from '../i18n/errorTemplates'
import { GalleryField, MediaField, galleryHeld, mediaHeld } from './MediaField'
import { RelationPicker, targetsHeld } from './RelationPicker'
import { pairsOf } from './types'
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

/** The DataViews type a text variant is edited as. */
const VARIANTS: Record<string, string> = {
	email: 'email',
	url: 'url',
}

/**
 * Returns the DataViews type the field is edited as, or nothing when a picker owns it.
 * @param field - The declared field to place.
 * @returns The type, or undefined.
 */
function fieldType(field: ContentField): string | undefined {
	if (field.kind === 'choice') {
		return field.settings.multiple === true ? 'array' : 'text'
	}
	if (field.kind === 'text') {
		const variant = worded(field.settings.variant)
		return variant !== undefined && VARIANTS[variant] !== undefined ? VARIANTS[variant] : 'text'
	}
	return TYPES[field.kind]
}

/**
 * Returns the fields the panel edits, which are the ones a control covers.
 * @param declared - The fields the type declares.
 * @returns The declared fields this panel renders.
 */
export function editableFields(declared: ContentField[]): ContentField[] {
	return declared.filter((field) => fieldType(field) !== undefined)
}

/**
 * Returns the sentence a value outside what its field takes earns, or nothing when it stands.
 * @param field - The declared field the value sits under.
 * @param value - The buffered value.
 * @returns The sentence, or undefined.
 */
function boundBroken(field: ContentField, value: unknown): string | undefined {
	if (field.kind === 'number') {
		return numberBroken(field, value)
	}
	if (field.kind === 'choice') {
		return choiceBroken(field, value)
	}
	return undefined
}

/**
 * Returns the sentence a number outside its bounds earns, or nothing when the value stands.
 * @param field - The declared field the value sits under.
 * @param value - The buffered value.
 * @returns The sentence, or undefined.
 */
function numberBroken(field: ContentField, value: unknown): string | undefined {
	if (typeof value !== 'number') {
		return undefined
	}
	const low = field.settings.min
	if (typeof low === 'number' && value < low) {
		return sprintf(errorTemplates().field_min, { field: field.label, limit: low } as never)
	}
	const high = field.settings.max
	if (typeof high === 'number' && value > high) {
		return sprintf(errorTemplates().field_max, { field: field.label, limit: high } as never)
	}
	return undefined
}

/**
 * Returns the sentence an answer the field does not list earns, or nothing when it stands.
 * @param field - The declared field the value sits under.
 * @param value - The buffered value.
 * @returns The sentence, or undefined.
 */
function choiceBroken(field: ContentField, value: unknown): string | undefined {
	const pairs = pairsOf(field.settings)
	if (field.settings.allow_custom === true || pairs.length === 0) {
		return undefined
	}
	const held = Array.isArray(value) ? value : [value]
	const strays = held.filter(
		(one) => typeof one === 'string' && one !== '' && !pairs.some((pair) => pair.value === one),
	)
	if (strays.length === 0) {
		return undefined
	}
	return sprintf(errorTemplates().field_choice, { field: field.label } as never)
}

/**
 * Returns the bound complaints the buffered values earn, keyed by field key.
 * @param declared - The fields the type declares.
 * @param values - The buffered values under edit.
 * @returns The complaints DataForm shows, or nothing when every value sits inside its bounds.
 */
export function fieldValidity(declared: ContentField[], values: FieldValues): FormValidity {
	const spoken: Record<string, FieldValidity> = {}
	for (const field of declared) {
		const message = boundBroken(field, values[field.key])
		if (message !== undefined) {
			spoken[field.key] = { custom: { type: 'invalid', message } }
		}
	}
	return Object.keys(spoken).length > 0 ? spoken : undefined
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
		type: fieldType(field),
		description: worded(field.settings.instructions),
		placeholder: worded(field.settings.placeholder),
		isValid: { required: field.required, ...boundsOf(field) },
		elements: choiceElements(field),
		Edit: editControl(field),
		enableSorting: false,
	})) as Field<FieldValues>[]
}

/**
 * Returns the entries a choice field offers its control.
 * @param field - The declared field to read.
 * @returns The entries, or nothing when the field offers none.
 */
function choiceElements(field: ContentField) {
	if (field.kind !== 'choice') {
		return undefined
	}
	const pairs = pairsOf(field.settings)
	if (pairs.length === 0) {
		return undefined
	}
	if (field.settings.allow_null === true) {
		return [{ value: '', label: __('None', 'gophenberg') }, ...pairs]
	}
	return pairs
}

/**
 * Returns the control a field's presentation asks for, or nothing for the type's own.
 * @param field - The declared field to place.
 * @returns The control name, the range component, or undefined.
 */
function editControl(field: ContentField): string | ComponentType<DataFormControlProps<FieldValues>> | undefined {
	if (field.kind === 'choice') {
		return choiceControl(field)
	}
	if (field.kind === 'text' && worded(field.settings.variant) === 'textarea') {
		return 'textarea'
	}
	if (field.kind === 'number' && worded(field.settings.presentation) === 'range') {
		return rangeEdit(field)
	}
	return undefined
}

/**
 * Returns the control a choice field's presentation asks for, or nothing for the type's own.
 * @param field - The declared choice field to place.
 * @returns The control name, the checkboxes component, or undefined.
 */
function choiceControl(
	field: ContentField,
): string | ComponentType<DataFormControlProps<FieldValues>> | undefined {
	const presentation = worded(field.settings.presentation)
	if (field.settings.multiple === true) {
		return presentation === 'checkbox' ? checkboxesEdit(field) : undefined
	}
	if (presentation === 'radio' || presentation === 'checkbox') {
		return field.settings.allow_custom === true ? radioEdit(field) : 'radio'
	}
	if (presentation === 'buttons') {
		return 'toggleGroup'
	}
	return undefined
}

/**
 * Returns the radios a group taking custom answers is edited with.
 * @param field - The declared field carrying the answers.
 * @returns The radios component.
 */
function radioEdit(field: ContentField): ComponentType<DataFormControlProps<FieldValues>> {
	const offered = choiceElements(field) ?? []
	/**
	 * Renders the listed answers beside a box taking one the field does not list.
	 * @param props - The item, the field, and what to call with a change.
	 * @returns The radios and the other box.
	 */
	return function RadioEdit({ data, field: described, onChange }: DataFormControlProps<FieldValues>) {
		const held = described.getValue({ item: data })
		const word = typeof held === 'string' ? held : ''
		const listed = offered.some((one) => one.value === word)
		/**
		 * Carries the answer the author settled on.
		 * @param next - The answer to store.
		 */
		function carry(next: string) {
			onChange(described.setValue({ item: data, value: next }))
		}
		return (
			<Stack direction="column" gap="xs">
				<RadioControl
					label={described.label}
					help={described.description}
					options={offered}
					selected={listed ? word : undefined}
					onChange={carry}
				/>
				<InputControl
					label={__('Other', 'gophenberg')}
					autoComplete="off"
					value={listed ? '' : word}
					onValueChange={carry}
				/>
			</Stack>
		)
	}
}

/**
 * Returns the checkboxes a checkbox group is edited with, or nothing when it lists no answers.
 * @param field - The declared field carrying the answers.
 * @returns The checkboxes component, or undefined.
 */
function checkboxesEdit(
	field: ContentField,
): ComponentType<DataFormControlProps<FieldValues>> | undefined {
	const pairs = pairsOf(field.settings)
	if (pairs.length === 0) {
		return undefined
	}
	/**
	 * Renders one checkbox per answer a checkbox group offers.
	 * @param props - The item, the field, and what to call with a change.
	 * @returns The checkbox list element.
	 */
	const taking = field.settings.allow_custom === true
	return function CheckboxesEdit({
		data,
		field: described,
		onChange,
		validity,
	}: DataFormControlProps<FieldValues>) {
		const named = useId()
		const held = wordsHeld(described.getValue({ item: data }))
		const strays = held.filter((one) => !pairs.some((pair) => pair.value === one))
		const offered = [...pairs, ...strays.map((one) => ({ value: one, label: one }))]
		/**
		 * Carries the answers the author settled on.
		 * @param next - The answers to store.
		 */
		function carry(next: string[]) {
			onChange(described.setValue({ item: data, value: next }))
		}
		return (
			<Stack direction="column" gap="xs" role="group" aria-labelledby={named}>
				<Text variant="body-sm" id={named}>
					{described.label}
				</Text>
				{described.description !== undefined && (
					<Text variant="body-sm">{described.description}</Text>
				)}
				{offered.map((pair) => (
					<CheckboxControl
						__nextHasNoMarginBottom
						key={pair.value}
						label={pair.label}
						checked={held.includes(pair.value)}
						onChange={(next) =>
							carry(next ? [...held, pair.value] : held.filter((one) => one !== pair.value))
						}
					/>
				))}
				{taking && <OtherAdder onAdd={(word) => carry(held.includes(word) ? held : [...held, word])} />}
				<Complaint validity={validity} />
			</Stack>
		)
	}
}

/**
 * Renders the sentence a turned away value earns, or nothing while the value stands.
 * @param props - The validity the control was handed.
 * @returns The sentence element, or nothing.
 */
function Complaint(props: { validity: FieldValidity | undefined }) {
	if (props.validity?.custom?.type !== 'invalid') {
		return null
	}
	return (
		<Text variant="body-sm" role="alert">
			{props.validity.custom.message}
		</Text>
	)
}

/**
 * Renders the box adding an answer a field does not list.
 * @param props - What to call with the answer typed.
 * @returns The box and the button committing it.
 */
function OtherAdder(props: { onAdd: (word: string) => void }) {
	const [typed, setTyped] = useState('')
	return (
		<Stack direction="row" gap="sm">
			<InputControl
				label={__('Other', 'gophenberg')}
				autoComplete="off"
				value={typed}
				onValueChange={setTyped}
			/>
			<Button
				variant="outline"
				size="compact"
				onClick={() => {
					if (typed === '') {
						return
					}
					props.onAdd(typed)
					setTyped('')
				}}
			>
				{__('Add', 'gophenberg')}
			</Button>
		</Stack>
	)
}

/**
 * Returns the string members a value holds.
 * @param value - The buffered value.
 * @returns The strings, empty for anything else.
 */
function wordsHeld(value: unknown): string[] {
	return Array.isArray(value) ? value.filter((one): one is string => typeof one === 'string') : []
}

/**
 * Returns the slider a bounded range is edited with, or nothing when a bound is missing.
 * @param field - The declared field carrying the bounds.
 * @returns The slider component, or undefined.
 */
function rangeEdit(field: ContentField): ComponentType<DataFormControlProps<FieldValues>> | undefined {
	const low = counted(field.settings.min)
	const high = counted(field.settings.max)
	if (low === undefined || high === undefined) {
		return undefined
	}
	const step = counted(field.settings.step)
	/**
	 * Renders the slider a bounded range is edited with.
	 * @param props - The item, the field, and what to call with a change.
	 * @returns The slider element.
	 */
	return function RangeEdit({ data, field: described, onChange }: DataFormControlProps<FieldValues>) {
		const held = described.getValue({ item: data })
		return (
			<RangeControl
				__next40pxDefaultSize
				__nextHasNoMarginBottom
				label={described.label}
				help={described.description}
				min={low}
				max={high}
				step={step}
				value={typeof held === 'number' ? held : undefined}
				onChange={(next) => onChange(described.setValue({ item: data, value: next }))}
			/>
		)
	}
}

/**
 * Returns the setting as text the control shows, or nothing when it holds none.
 * @param held - The setting to read.
 * @returns The text, or undefined.
 */
function worded(held: unknown): string | undefined {
	return typeof held === 'string' && held !== '' ? held : undefined
}

/**
 * Returns the setting as a number a rule takes, or nothing when it holds none.
 * @param held - The setting to read.
 * @returns The number, or undefined.
 */
function counted(held: unknown): number | undefined {
	return typeof held === 'number' ? held : undefined
}

/**
 * Returns the validation rules a field's bounds set on its control.
 * @param field - The declared field to read.
 * @returns The rules the control validates against.
 */
function boundsOf(field: ContentField): Record<string, number> {
	const rules: Record<string, number> = {}
	if (field.kind !== 'text') {
		return rules
	}
	const longest = counted(field.settings.maxlength)
	if (longest !== undefined) {
		rules.maxLength = longest
	}
	return rules
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
	const complaints = useMemo(() => fieldValidity(rendered, buffer.fields), [rendered, buffer.fields])
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
					validity={complaints}
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
			{pictured.map((field) =>
				field.many ? (
					<GalleryField
						key={field.key}
						field={field}
						value={galleryHeld(buffer.fields[field.key])}
						onChange={(value) => buffer.setFields({ ...buffer.fields, [field.key]: value })}
					/>
				) : (
					<MediaField
						key={field.key}
						field={field}
						value={mediaHeld(buffer.fields[field.key])}
						onChange={(value) => buffer.setFields({ ...buffer.fields, [field.key]: value })}
					/>
				),
			)}
		</Stack>
	)
}
