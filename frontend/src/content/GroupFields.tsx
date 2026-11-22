// SPDX-License-Identifier: Apache-2.0

import { Badge, IconButton, Stack, Text, downIcon, upIcon } from '@gophenberg/frontend-sdk'
import { ErrorNotice, SectionTitle, useToaster } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
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
import { DeleteField } from './DeleteField'
import { FieldConditions } from './FieldConditions'
import { FieldSettings, ListField, RenameField, RequireField } from './FieldSettings'
import { CarryField } from './MoveField'
import { SubFields } from './SubFields'
import { kindLabel } from './types'
import { typesQueryKey } from './nav'
import type { FieldWrite, Reporter } from './fieldControls'
import type { FieldGroup } from './groups'
import type { ContentField, ContentType } from './types'

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
export function GroupFields(props: {
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[]
	shadowed: Set<string>
}) {
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
function FieldsBody(
	props: Reporter & { held: FieldGroup; groups: FieldGroup[]; types: ContentType[]; shadowed: Set<string> },
) {
	const declared = props.held.fields
	const reorder = useMutation({
		mutationFn: (keys: string[]) => reorderFieldsInGroup(props.held.id, keys),
		onSuccess: () => props.onDone(__('Order stored.', 'gophenberg')),
		onError: (cause: unknown) => props.onRefused(cause, 'order'),
	})

	/**
	 * Returns the declared keys with one field moved by an offset.
	 * @param index - The field's place in the declared order.
	 * @param offset - How far the field moves.
	 * @returns The keys in the asked order.
	 */
	function moved(index: number, offset: number): string[] {
		const keys = declared.map((field) => field.key)
		const [taken] = keys.splice(index, 1)
		keys.splice(index + offset, 0, taken)
		return keys
	}

	return (
		<Stack direction="column" gap="md">
			{declared.length === 0 ? (
				<Text>{__('This group holds no fields yet.', 'gophenberg')}</Text>
			) : (
				<ul className="gophenberg-fields__list gophenberg-fields__list--top">
					{declared.map((field, index) => (
						<li key={field.key} aria-label={field.label}>
							<Stack direction="column" gap="xs">
								<Stack direction="row" gap="sm" align="center" justify="space-between">
									<FieldTitle field={field} shadowed={props.shadowed.has(field.key)} />
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
											disabled={reorder.isPending || index === declared.length - 1}
											onClick={() => reorder.mutate(moved(index, 1))}
										/>
									</Stack>
								</Stack>
								<Stack direction="row" gap="xs" align="center" wrap="wrap">
									<RequireField
										group={props.held.id}
										field={field}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
									<ListField
										group={props.held.id}
										field={field}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
									<FieldConditions
										group={props.held.id}
										field={field}
										siblings={declared}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
									<RenameField
										group={props.held.id}
										field={field}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
									<FieldSettings
										group={props.held.id}
										field={field}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
									<CarryField
										group={props.held.id}
										field={field}
										groups={props.groups}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
									<SubFields
										group={props.held.id}
										types={props.types}
										field={field}
										path={field.key}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
									<DeleteField
										group={props.held.id}
										field={field}
										onDone={props.onDone}
										onRefused={props.onRefused}
									/>
								</Stack>
							</Stack>
							<HeldFields
								group={props.held.id}
								groups={props.groups}
								types={props.types}
								declared={field.fields}
								at={field.key}
								onDone={props.onDone}
								onRefused={props.onRefused}
							/>
						</li>
					))}
				</ul>
			)}
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
 * Renders the name of a field, its kind and the badges it carries.
 * @param props - The field and whether an earlier group serves its key in the group's place.
 * @returns The name row.
 */
function FieldTitle(props: { field: ContentField; shadowed: boolean }) {
	return (
		<Stack direction="row" gap="xs" align="center">
			<Text>{props.field.label}</Text>
			<Text variant="body-sm">{kindLabel(props.field.kind)}</Text>
			{props.field.required && <Badge>{__('Required', 'gophenberg')}</Badge>}
			{props.shadowed && <Badge intent="high">{__('Shadowed', 'gophenberg')}</Badge>}
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
							<FieldTitle field={field} shadowed={false} />
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
