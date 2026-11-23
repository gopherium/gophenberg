// SPDX-License-Identifier: Apache-2.0

import {
	Badge,
	IconButton,
	Stack,
	Text,
	VisuallyHidden,
	downIcon,
	listIcon,
	upIcon,
} from '@gophenberg/frontend-sdk'
import { ErrorNotice, ListEmpty, RowControls, SectionTitle, useToaster } from '@gopherium/godmin'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
	groupErrorMessage,
	groupsQueryKey,
	reorderFieldsInGroup,
	reorderSubFields,
	StaleWriteError,
} from './groups'
import { AddField } from './AddField'
import { DeleteConfirm, DeleteField, deleteName } from './DeleteField'
import { FieldConditions } from './FieldConditions'
import { FieldSettings, ListField, RenameField, RequireField } from './FieldSettings'
import { CarryField } from './MoveField'
import { SubFields } from './SubFields'
import { pickedLabel } from './types'
import { useFieldOrder } from './useFieldOrder'
import { typesQueryKey } from './nav'
import type { FieldWrite, Reporter } from './fieldControls'
import type { FieldGroup } from './groups'
import type { ContentField, ContentType } from './types'

/** Where the fields of a group stand: the group, every group, the types, and the keys an earlier group serves. */
interface FieldsPlace {
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[]
	shadowed: Set<string>
}

/** The columns of the fields table: Label, Kind, API name and the actions. */
const COLUMNS = 4

/**
 * Returns the toast naming a field write that failed, its reason left to fill in.
 * @param write - The kind of write that failed.
 * @returns The toast before the reason is written in.
 */
function failedWrite(write: FieldWrite) {
	const sentences = {
		save: __('Could not save the field. %s', 'gophenberg'),
		add: __('Could not add the field. %s', 'gophenberg'),
		order: __('Could not order the fields. %s', 'gophenberg'),
		delete: __('Could not delete the field. %s', 'gophenberg'),
	}
	return sentences[write]
}

/**
 * Renders the fields a group holds, the controls managing them and the notice naming the last failed write.
 * @param props - The group, the other groups, the types and the keys an earlier group serves in its place.
 * @returns The fields element.
 */
export function GroupFields(props: FieldsPlace) {
	const client = useQueryClient()
	const toaster = useToaster()
	const [notice, setNotice] = useState('')

	/**
	 * Reports what a field write did in a toast and refreshes what the screen holds.
	 * @param said - The sentence naming what was done.
	 */
	async function done(said: string) {
		setNotice('')
		toaster.show(said)
		await Promise.all([
			client.invalidateQueries({ queryKey: groupsQueryKey }),
			client.invalidateQueries({ queryKey: typesQueryKey }),
		])
	}

	/**
	 * Reports why a field write was turned away, in a toast naming what failed and above the fields.
	 * @param cause - What the write failed with.
	 * @param write - The kind of write that failed, a field save when none is given.
	 */
	async function refused(cause: unknown, write: FieldWrite = 'save') {
		const said = groupErrorMessage(cause)
		setNotice(said)
		toaster.show(sprintf(failedWrite(write), said))
		if (cause instanceof StaleWriteError) {
			await client.invalidateQueries({ queryKey: groupsQueryKey })
		}
	}

	return (
		<Stack direction="column" gap="md">
			{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
			<FieldsBody
				held={props.held}
				groups={props.groups}
				types={props.types}
				shadowed={props.shadowed}
				onDone={done}
				onRefused={refused}
			/>
		</Stack>
	)
}

/**
 * Renders the fields a group holds and the control declaring another.
 * @param props - The group, the other groups, the types, the keys an earlier group serves in its place, and what to
 * report.
 * @returns The body element.
 */
function FieldsBody(props: Reporter & FieldsPlace) {
	return (
		<Stack direction="column" gap="lg">
			<FieldList
				held={props.held}
				groups={props.groups}
				types={props.types}
				shadowed={props.shadowed}
				onDone={props.onDone}
				onRefused={props.onRefused}
			/>
			<Stack direction="column" gap="sm">
				<SectionTitle>{__('Add a field', 'gophenberg')}</SectionTitle>
				<AddField
					group={props.held.id}
					types={props.types}
					groups={props.groups}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			</Stack>
		</Stack>
	)
}

/**
 * Renders the region the fields table scrolls in, holding the table or what shows while the group holds no field.
 * @param props - The group, the other groups, the types, the keys an earlier group serves, and what to report.
 * @returns The region element.
 */
function FieldList(props: Reporter & FieldsPlace) {
	const empty = props.held.fields.length === 0
	return (
		<div
			className="godmin-table-scroll"
			role="region"
			aria-label={__('Fields', 'gophenberg')}
			tabIndex={empty ? -1 : 0}
		>
			{empty ? (
				<NoFields />
			) : (
				<FieldTable
					held={props.held}
					groups={props.groups}
					types={props.types}
					shadowed={props.shadowed}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			)}
		</div>
	)
}

/**
 * Renders what the fields table shows while the group holds no field.
 * @returns The empty state.
 */
function NoFields() {
	return (
		<ListEmpty
			icon={listIcon}
			title={__('No fields yet', 'gophenberg')}
			description={__('Add a field to store more about the items this group shows on.', 'gophenberg')}
		/>
	)
}

/**
 * Renders the table of the fields a group holds, one row group per field, its arrows moving a field at once.
 * @param props - The group, the other groups, the types, the keys an earlier group serves, and what to report.
 * @returns The table element.
 */
function FieldTable(props: Reporter & FieldsPlace) {
	const client = useQueryClient()
	const order = useFieldOrder(props.held.fields, {
		store: (keys) => reorderFieldsInGroup(props.held.id, keys),
		onStored: () => props.onDone(__('Order stored.', 'gophenberg')),
		onFailed: async (cause) => {
			await props.onRefused(cause, 'order')
			await client.invalidateQueries({ queryKey: groupsQueryKey })
		},
	})

	return (
		<table className="godmin-table">
			<thead>
				<tr>
					<th scope="col">{__('Label', 'gophenberg')}</th>
					<th scope="col">{_x('Kind', 'field type', 'gophenberg')}</th>
					<th scope="col">{__('API name', 'gophenberg')}</th>
					<th scope="col" className="godmin-table__actions">
						<VisuallyHidden>{__('Actions', 'gophenberg')}</VisuallyHidden>
					</th>
				</tr>
			</thead>
			{order.rows.map((field, index) => (
				<FieldRows
					key={field.key}
					held={props.held}
					groups={props.groups}
					types={props.types}
					shadowed={props.shadowed}
					field={field}
					at={index}
					count={order.rows.length}
					onMove={(offset) => order.move(index, offset)}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			))}
		</table>
	)
}

/** One field of the table: the field, its place, how many fields the table holds, and the move it asks for. */
interface Placed {
	field: ContentField
	at: number
	count: number
	onMove: (offset: number) => void
}

/**
 * Renders the row group of one field: its row, then the list of the sub fields it holds.
 * @param props - Where the fields stand, the field, its place, the table's length, its move, and what to report.
 * @returns The row group element.
 */
function FieldRows(props: Reporter & FieldsPlace & Placed) {
	const { field } = props
	const [deleting, setDeleting] = useState(false)
	return (
		<tbody>
			<tr>
				<td>
					<LabelCell
						held={props.held}
						groups={props.groups}
						types={props.types}
						shadowed={props.shadowed}
						field={field}
						onDone={props.onDone}
						onRefused={props.onRefused}
					/>
				</td>
				<td>
					<Badge intent="stable">{pickedLabel(field)}</Badge>
				</td>
				<td>
					<code>{field.key}</code>
				</td>
				<td className="godmin-table__actions">
					<RowControls
						at={props.at}
						count={props.count}
						labels={{
							moveUp: sprintf(__('Move %(field)s up', 'gophenberg'), { field: field.label }),
							moveDown: sprintf(__('Move %(field)s down', 'gophenberg'), { field: field.label }),
							remove: deleteName(field),
						}}
						onMove={props.onMove}
						onRemove={() => setDeleting(true)}
					/>
					{deleting && (
						<DeleteConfirm
							group={props.held.id}
							field={field}
							onDone={props.onDone}
							onRefused={props.onRefused}
							onClose={() => setDeleting(false)}
						/>
					)}
				</td>
			</tr>
			{field.fields.length > 0 && (
				<tr>
					<td colSpan={COLUMNS}>
						<HeldFields
							group={props.held.id}
							groups={props.groups}
							types={props.types}
							declared={field.fields}
							at={field.key}
							onDone={props.onDone}
							onRefused={props.onRefused}
						/>
					</td>
				</tr>
			)}
		</tbody>
	)
}

/**
 * Renders the Label cell of a field: its label, Move and the buttons that open its dialogs, then its badges.
 * @param props - Where the fields stand, the field, and what to report.
 * @returns The cell's content.
 */
function LabelCell(props: Reporter & FieldsPlace & { field: ContentField }) {
	const { field } = props
	const group = props.held.id
	return (
		<Stack direction="column" gap="xs">
			<Text>{field.label}</Text>
			<Stack direction="row" gap="xs" align="center" wrap="wrap" className="gophenberg-fields__actions">
				<CarryField
					group={group}
					field={field}
					groups={props.groups}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
				<RequireField group={group} field={field} onDone={props.onDone} onRefused={props.onRefused} />
				<ListField group={group} field={field} onDone={props.onDone} onRefused={props.onRefused} />
				<FieldConditions
					group={group}
					field={field}
					siblings={props.held.fields}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
				<RenameField group={group} field={field} onDone={props.onDone} onRefused={props.onRefused} />
				<FieldSettings group={group} field={field} onDone={props.onDone} onRefused={props.onRefused} />
				<SubFields
					group={group}
					types={props.types}
					field={field}
					path={field.key}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			</Stack>
			<FieldBadges field={field} shadowed={props.shadowed.has(field.key)} />
		</Stack>
	)
}

/**
 * Renders the badges a field carries: Required, and Shadowed when an earlier group serves its key.
 * @param props - The field and whether an earlier group serves its key in the group's place.
 * @returns The badges, or nothing when the field carries none.
 */
function FieldBadges(props: { field: ContentField; shadowed: boolean }) {
	if (!props.field.required && !props.shadowed) {
		return null
	}
	return (
		<Stack direction="row" gap="xs" align="center" wrap="wrap">
			{props.field.required && <Badge>{__('Required', 'gophenberg')}</Badge>}
			{props.shadowed && <Badge intent="high">{__('Shadowed', 'gophenberg')}</Badge>}
		</Stack>
	)
}

/**
 * Renders the name of a sub field, its kind and the badges it carries.
 * @param props - The sub field.
 * @returns The name row.
 */
function FieldTitle(props: { field: ContentField }) {
	return (
		<Stack direction="row" gap="xs" align="center">
			<Text>{props.field.label}</Text>
			<Text variant="body-sm">{pickedLabel(props.field)}</Text>
			<FieldBadges field={props.field} shadowed={false} />
		</Stack>
	)
}

/**
 * Renders the sub fields a container declares, however deep they run.
 * @param props - The group, every group, the types, the sub fields, the path holding them, and what to report.
 * @returns The list element, or nothing when the container declares none.
 */
function HeldFields(
	props: Reporter & {
		group: number
		groups: FieldGroup[]
		types: ContentType[]
		declared: ContentField[]
		at: string
	},
) {
	const reorder = useMutation({
		mutationFn: (keys: string[]) => reorderSubFields(props.group, props.at, keys),
		onSuccess: () => props.onDone(__('Order stored.', 'gophenberg')),
		onError: (cause: unknown) => props.onRefused(cause, 'order'),
	})

	/**
	 * Returns the held keys with one field moved by an offset.
	 * @param index - The field's place in the held order.
	 * @param offset - How far the field moves.
	 * @returns The keys in the asked order.
	 */
	function moved(index: number, offset: number): string[] {
		const keys = props.declared.map((field) => field.key)
		const [taken] = keys.splice(index, 1)
		keys.splice(index + offset, 0, taken)
		return keys
	}

	if (props.declared.length === 0) {
		return null
	}
	return (
		<ul className="gophenberg-fields__list">
			{props.declared.map((field, index) => (
				<li key={field.key} aria-label={field.label}>
					<Stack direction="column" gap="xs">
						<Stack direction="row" gap="sm" align="center" justify="space-between">
							<FieldTitle field={field} />
							<Stack direction="row" gap="xs" align="center">
								<IconButton
									icon={upIcon}
									label={sprintf(__('Move %(field)s up', 'gophenberg'), { field: field.label })}
									size="compact"
									variant="minimal"
									tone="neutral"
									disabled={reorder.isPending || index === 0}
									onClick={() => reorder.mutate(moved(index, -1))}
								/>
								<IconButton
									icon={downIcon}
									label={sprintf(__('Move %(field)s down', 'gophenberg'), { field: field.label })}
									size="compact"
									variant="minimal"
									tone="neutral"
									disabled={reorder.isPending || index === props.declared.length - 1}
									onClick={() => reorder.mutate(moved(index, 1))}
								/>
							</Stack>
						</Stack>
						<Stack direction="row" gap="xs" align="center" wrap="wrap">
							<RequireField
								group={props.group}
								field={field}
								path={props.at + '.' + field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
							<ListField
								group={props.group}
								field={field}
								path={props.at + '.' + field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
							<FieldConditions
								group={props.group}
								field={field}
								siblings={props.declared}
								path={props.at + '.' + field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
							<RenameField
								group={props.group}
								field={field}
								path={props.at + '.' + field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
							<FieldSettings
								group={props.group}
								field={field}
								path={props.at + '.' + field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
							<CarryField
								group={props.group}
								field={field}
								path={props.at + '.' + field.key}
								groups={props.groups}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
							<SubFields
								group={props.group}
								types={props.types}
								field={field}
								path={props.at + '.' + field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
							<DeleteField
								group={props.group}
								field={field}
								path={props.at + '.' + field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
						</Stack>
					</Stack>
					<HeldFields
						group={props.group}
						groups={props.groups}
						types={props.types}
						declared={field.fields}
						at={props.at + '.' + field.key}
						onDone={props.onDone}
						onRefused={props.onRefused}
					/>
				</li>
			))}
		</ul>
	)
}
