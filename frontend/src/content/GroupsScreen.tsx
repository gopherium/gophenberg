// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Dialog, IconButton, InputControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, _x, sprintf } from '@wordpress/i18n'
import { ErrorNotice, LoadingRows, Page, useToaster } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { createGroup, deleteGroup, groupsQueryKey, listGroups, reorderGroups, updateGroup } from './groups'
import { typesQueryKey } from './nav'
import { listTypes } from './types'
import type { FieldGroup, GroupEdit, Location } from './groups'
import type { ContentType } from './types'

/** The value a rule carries to match every content type. */
const ANY_TYPE = '*'

/** The source the built in content type rule reads. */
const TYPE_SOURCE = 'content_type'

const upIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="20" height="20">
		<path d="M12 8l6 6H6z" />
	</svg>
)

const downIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="20" height="20">
		<path d="M12 16l-6-6h12z" />
	</svg>
)

/** A content type as the placement sentence reads it. */
interface NamedType {
	key: string
	pluralLabel: string
}

/**
 * Returns the sentence naming where a group's rules place it.
 * @param location - The rules the group carries.
 * @param types - The registered types the rules may name.
 * @returns The sentence the list column shows.
 */
export function placementOf(location: Location, types: NamedType[]): string {
	if (location.length === 0) {
		return __('Nowhere', 'gophenberg')
	}
	return location.map((group) => group.map((rule) => ruleSentence(rule, types)).join(__(' and ', 'gophenberg')))
		.join(__(' or ', 'gophenberg'))
}

/**
 * Returns the phrase one rule reads as.
 * @param rule - The rule to phrase.
 * @param types - The registered types the rule may name.
 * @returns The phrase for that rule.
 */
function ruleSentence(rule: { source: string; operator: string; value: string }, types: NamedType[]): string {
	if (rule.source !== TYPE_SOURCE) {
		return sprintf(__('%(source)s is %(value)s', 'gophenberg'), { source: rule.source, value: rule.value })
	}
	const named = rule.value === ANY_TYPE
		? __('Every content type', 'gophenberg')
		: (types.find((held) => held.key === rule.value)?.pluralLabel ?? rule.value)
	if (rule.operator === '!=') {
		return sprintf(__('Not %(type)s', 'gophenberg'), { type: named })
	}
	return named
}

/**
 * Returns the sentence naming why a group write was turned away.
 * @param cause - What the write failed with.
 * @returns The sentence to show.
 */
export function groupErrorMessage(cause: unknown): string {
	return cause instanceof Error ? cause.message : __('The field groups could not be reached.', 'gophenberg')
}

/**
 * Renders the field groups screen.
 * @returns The field groups screen element.
 */
export function GroupsScreen() {
	const client = useQueryClient()
	const toaster = useToaster()
	const [notice, setNotice] = useState('')
	const groups = useQuery({ queryKey: groupsQueryKey, queryFn: listGroups })
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })

	/**
	 * Reports what a group write did, and refreshes what the admin holds.
	 * @param said - The sentence naming what was done.
	 */
	async function done(said: string) {
		setNotice('')
		toaster.show(said)
		await client.invalidateQueries({ queryKey: groupsQueryKey })
		await client.invalidateQueries({ queryKey: typesQueryKey })
	}

	/**
	 * Reports why a group write was turned away.
	 * @param cause - What the write failed with.
	 */
	function refused(cause: unknown) {
		setNotice(groupErrorMessage(cause))
	}

	return (
		<Page
			title={__('Field Groups', 'gophenberg')}
			subtitle={__('Bundles of fields, shown where their rules place them.', 'gophenberg')}
			actions={<AddGroup types={types.data ?? []} onDone={done} onRefused={refused} />}
		>
			<Stack direction="column" gap="md">
				{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
				<GroupsBody
					groups={groups.data ?? []}
					types={types.data ?? []}
					loading={groups.isPending}
					failed={groups.isError}
					onDone={done}
					onRefused={refused}
				/>
			</Stack>
		</Page>
	)
}

/** What a row reports back when a write finishes. */
interface Reporter {
	onDone: (said: string) => void
	onRefused: (cause: unknown) => void
}

/**
 * Renders the groups in whichever state they are in.
 * @param props - The groups, the types, whether the read is loading or failed, and the reporter.
 * @returns The groups body element.
 */
function GroupsBody(
	props: Reporter & { groups: FieldGroup[]; types: ContentType[]; loading: boolean; failed: boolean },
) {
	if (props.failed) {
		return <ErrorNotice>{__('The field groups could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	if (props.loading) {
		return <LoadingRows label={__('Loading field groups.', 'gophenberg')} />
	}
	return (
		<div
			className="godmin-table-scroll godmin-arrival"
			role="region"
			aria-label={__('Field Groups', 'gophenberg')}
			tabIndex={0}
		>
			<table className="godmin-table">
				<thead>
					<tr>
						<th scope="col">{__('Group', 'gophenberg')}</th>
						<th scope="col">{__('Appears on', 'gophenberg')}</th>
						<th scope="col">{__('Fields', 'gophenberg')}</th>
						<th scope="col">{_x('Status', 'field group', 'gophenberg')}</th>
						<th scope="col">{__('Actions', 'gophenberg')}</th>
					</tr>
				</thead>
				<tbody>
					{props.groups.map((held, at) => (
						<GroupRow
							key={held.id}
							held={held}
							types={props.types}
							order={props.groups.map((listed) => listed.id)}
							at={at}
							onDone={props.onDone}
							onRefused={props.onRefused}
						/>
					))}
				</tbody>
			</table>
		</div>
	)
}

/**
 * Renders one field group and what may be done to it.
 * @param props - The group, the types, the order it sits in, and the reporter.
 * @returns The row element.
 */
function GroupRow(
	props: Reporter & { held: FieldGroup; types: ContentType[]; order: number[]; at: number },
) {
	const { held } = props
	const edit = useMutation({
		mutationFn: (asked: GroupEdit) => updateGroup(held.id, asked),
		onSuccess: () => props.onDone(sprintf(__('%(group)s updated.', 'gophenberg'), { group: held.title })),
		onError: props.onRefused,
	})
	const move = useMutation({
		mutationFn: (ids: number[]) => reorderGroups(ids),
		onSuccess: () => props.onDone(__('The groups were reordered.', 'gophenberg')),
		onError: props.onRefused,
	})
	return (
		<tr>
			<td>{held.title}</td>
			<td>{placementOf(held.location, props.types)}</td>
			<td>{String(held.fields.length)}</td>
			<td>{!held.active && <Badge intent="draft">{__('Inactive', 'gophenberg')}</Badge>}</td>
			<td>
				<Stack direction="row" gap="xs">
					<MoveGroup held={held} order={props.order} at={props.at} pending={move.isPending} onMove={move.mutate} />
					<Button variant="outline" onClick={() => edit.mutate({ active: !held.active })}>
						{held.active ? __('Deactivate', 'gophenberg') : __('Activate', 'gophenberg')}
					</Button>
					<RemoveGroup held={held} onDone={props.onDone} onRefused={props.onRefused} />
				</Stack>
			</td>
		</tr>
	)
}

/**
 * Renders the controls moving a group through the order.
 * @param props - The group, the order, its place in it, and what to do.
 * @returns The move controls.
 */
function MoveGroup(props: {
	held: FieldGroup
	order: number[]
	at: number
	pending: boolean
	onMove: (ids: number[]) => void
}) {
	/**
	 * Returns the order with the group carried one place in the given direction.
	 * @param by - How far to carry the group.
	 * @returns The order to store.
	 */
	function carried(by: number): number[] {
		const moved = [...props.order]
		const landing = props.at + by
		;[moved[props.at], moved[landing]] = [moved[landing], moved[props.at]]
		return moved
	}
	return (
		<>
			<IconButton
				label={sprintf(__('Move %(group)s up', 'gophenberg'), { group: props.held.title })}
				icon={upIcon}
				variant="outline"
				disabled={props.pending || props.at === 0}
				onClick={() => props.onMove(carried(-1))}
			/>
			<IconButton
				label={sprintf(__('Move %(group)s down', 'gophenberg'), { group: props.held.title })}
				icon={downIcon}
				variant="outline"
				disabled={props.pending || props.at === props.order.length - 1}
				onClick={() => props.onMove(carried(1))}
			/>
		</>
	)
}

/**
 * Renders the confirmation removing a group takes.
 * @param props - The group and the reporter.
 * @returns The control and its dialog.
 */
function RemoveGroup(props: Reporter & { held: FieldGroup }) {
	const [open, setOpen] = useState(false)
	const remove = useMutation({
		mutationFn: () => deleteGroup(props.held.id),
		onSuccess: () => {
			setOpen(false)
			props.onDone(sprintf(__('%(group)s removed.', 'gophenberg'), { group: props.held.title }))
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				{__('Delete', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Delete %(group)s', 'gophenberg'), { group: props.held.title })}
						</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Text>
							{__(
								'The fields this group holds go with it, and so do the values every item stored in them.',
								'gophenberg',
							)}
						</Text>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Keep it', 'gophenberg')}
						</Button>
						<Button loading={remove.isPending} onClick={() => remove.mutate()}>
							{__('Delete the group', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the control storing a new field group.
 * @param props - The registered types and the reporter.
 * @returns The control and its dialog.
 */
function AddGroup(props: Reporter & { types: ContentType[] }) {
	const [open, setOpen] = useState(false)
	const [title, setTitle] = useState('')
	const seeded = props.types[0]?.key ?? ''
	const add = useMutation({
		mutationFn: () =>
			createGroup(title, [[{ source: TYPE_SOURCE, operator: '==', value: seeded }]]),
		onSuccess: () => {
			setOpen(false)
			setTitle('')
			props.onDone(sprintf(__('%(group)s created.', 'gophenberg'), { group: title }))
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button onClick={() => setOpen(true)}>{__('Add New Group', 'gophenberg')}</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{__('Create a field group', 'gophenberg')}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<InputControl
								label={__('Title', 'gophenberg')}
								autoComplete="off"
								value={title}
								onValueChange={setTitle}
							/>
							<Text variant="body-sm">
								{sprintf(
									__('It starts on %(type)s, and the rules may be edited afterwards.', 'gophenberg'),
									{ type: props.types[0]?.pluralLabel ?? __('a content type', 'gophenberg') },
								)}
							</Text>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Cancel', 'gophenberg')}
						</Button>
						<Button loading={add.isPending} onClick={() => add.mutate()}>
							{__('Create', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
