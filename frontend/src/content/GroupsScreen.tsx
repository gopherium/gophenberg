// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, IconButton, InputControl, Notice, Stack, downIcon, upIcon } from '@gophenberg/frontend-sdk'
import { __, _x, sprintf } from '@wordpress/i18n'
import { ConfirmDialog, ErrorNotice, FormDialog, LoadingScreen, Page, useToaster } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { irreversible } from '../lists/itemWords'
import {
	anyType,
	createGroup,
	deleteGroup,
	groupErrorMessage,
	groupsQueryKey,
	listGroups,
	reorderGroups,
	typeSource,
	updateGroup,
} from './groups'
import { definitionsDownloadPath } from './definitions'
import { DriftNotices } from './DriftNotices'
import { typesQueryKey } from './nav'
import { GroupFieldsDialog } from './GroupFieldsDialog'
import { ImportDefinitions } from './ImportDefinitions'
import { PluginFieldsDialog } from './PluginFieldsDialog'
import { RulesDialog } from './RulesDialog'
import { pluralOf } from './typeText'
import { listTypes } from './types'
import type { FieldGroup, GroupEdit, Location, LocationRule } from './groups'
import type { ContentType } from './types'

/** A content type as the placement sentence reads it. */
interface NamedType {
	key: string
	pluralLabel: string
	origin?: string
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
	if (rule.source !== typeSource) {
		return sprintf(__('%(source)s is %(value)s', 'gophenberg'), { source: rule.source, value: rule.value })
	}
	const found = types.find((held) => held.key === rule.value)
	const named = rule.value === anyType
		? __('Every content type', 'gophenberg')
		: (found === undefined ? rule.value : pluralOf(found))
	if (rule.operator === '!=') {
		return sprintf(__('Not %(type)s', 'gophenberg'), { type: named })
	}
	return named
}

/**
 * Returns whether one rule holds on the type, as the server evaluates it.
 * @param rule - The rule to evaluate.
 * @param typeKey - The type the screen would show.
 * @returns Whether the rule holds.
 */
function ruleHolds(rule: LocationRule, typeKey: string): boolean {
	if (rule.source !== typeSource) {
		return false
	}
	const matched = rule.value === anyType || rule.value === typeKey
	return rule.operator === '!=' ? !matched : matched
}

/**
 * Returns whether a location places its group on the type.
 * @param location - The rules the group carries.
 * @param typeKey - The type the screen would show.
 * @returns Whether any rule set fully holds.
 */
function matchesType(location: Location, typeKey: string): boolean {
	return location.some((set) => set.length > 0 && set.every((rule) => ruleHolds(rule, typeKey)))
}

/** One field key a group holds, as the overlap walk reads it. */
interface HeldKey {
	key: string
}

/** A group as the overlap walk reads it. */
interface PlacedGroup {
	id: number
	title: string
	active: boolean
	location: Location
	fields: HeldKey[]
}

/** One field key a group holds without showing it, and who serves it instead. */
export interface Shadowing {
	key: string
	winner: string
	loser: string
	loserID: number
	where: string[]
}

/**
 * Returns every field key a later group holds while an earlier one serves it.
 * @param groups - The groups in the order the listing serves them.
 * @param types - The registered types the rules may reach.
 * @returns The overlaps, each naming the winning and the losing group.
 */
export function shadowings(groups: PlacedGroup[], types: NamedType[]): Shadowing[] {
	const active = groups.filter((held) => held.active)
	const noted = new Map<string, Shadowing>()
	for (const listed of types) {
		const reaching = active.filter((held) => matchesType(held.location, listed.key))
		for (const overlap of overlapsAmong(reaching)) {
			noteOverlap(noted, overlap, pluralOf(listed))
		}
	}
	return [...noted.values()]
}

/** One field key two groups on the same type both hold. */
interface Overlap {
	key: string
	winner: PlacedGroup
	loser: PlacedGroup
}

/**
 * Returns every field key a later group holds while an earlier one serves it.
 * @param reaching - The active groups that reach one type, in listing order.
 * @returns The overlaps in the order they were found.
 */
function overlapsAmong(reaching: PlacedGroup[]): Overlap[] {
	const winners = new Map<string, PlacedGroup>()
	const overlaps: Overlap[] = []
	for (const held of reaching) {
		for (const f of held.fields) {
			const winner = winners.get(f.key)
			if (winner === undefined) {
				winners.set(f.key, held)
			} else {
				overlaps.push({ key: f.key, winner, loser: held })
			}
		}
	}
	return overlaps
}

/**
 * Records the overlap between two groups on one type.
 * @param noted - The overlaps already found, by key and pair, in the order they were found.
 * @param overlap - The field key and the two groups holding it.
 * @param label - The plural label of the type both reach.
 */
function noteOverlap(noted: Map<string, Shadowing>, { key, winner, loser }: Overlap, label: string) {
	const at = `${key}:${winner.id}:${loser.id}`
	const known = noted.get(at)
	if (known !== undefined) {
		known.where.push(label)
		return
	}
	noted.set(at, { key, winner: winner.title, loser: loser.title, loserID: loser.id, where: [label] })
}

/**
 * Returns the sentence one overlap warns with.
 * @param held - The overlap to phrase.
 * @returns The sentence the notice shows.
 */
function shadowingSentence(held: Shadowing): string {
	return sprintf(
		__('%(key)s is served by %(winner)s, so %(loser)s does not show it on %(where)s.', 'gophenberg'),
		{
			key: held.key,
			winner: held.winner,
			loser: held.loser,
			where: held.where.join(__(' and ', 'gophenberg')),
		},
	)
}

/**
 * Renders the field groups screen.
 * @returns The field groups screen element.
 */
export function GroupsScreen() {
	const client = useQueryClient()
	const toaster = useToaster()
	const groups = useQuery({ queryKey: groupsQueryKey, queryFn: listGroups })
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const overlaps = shadowings(groups.data ?? [], types.data ?? [])
	const typesLost = __(
		'The content types could not be loaded, so where these groups appear and which fields they shadow are not shown.',
		'gophenberg',
	)
	const typesStale = __(
		'The content types could not be refreshed, so where these groups appear and what they shadow may be out of date.',
		'gophenberg',
	)

	/**
	 * Reports what a group write did, and refreshes what the admin holds.
	 * @param said - The sentence naming what was done.
	 */
	async function done(said: string) {
		toaster.show(said)
		await client.invalidateQueries({ queryKey: groupsQueryKey })
		await client.invalidateQueries({ queryKey: typesQueryKey })
	}

	/**
	 * Reports why a group write was turned away.
	 * @param cause - What the write failed with.
	 */
	function refused(cause: unknown) {
		toaster.show(sprintf(__('Could not save the change. %s', 'gophenberg'), groupErrorMessage(cause)))
	}

	return (
		<Page
			title={__('Field Groups', 'gophenberg')}
			headingLevel={2}
			subtitle={__('Bundles of fields, shown where their rules place them.', 'gophenberg')}
			actions={
				<>
					<ExportDefinitions />
					<ImportDefinitions />
					<AddGroup types={types.data ?? []} onDone={done} />
				</>
			}
		>
			<Stack direction="column" gap="md">
				<DriftNotices onDone={done} onRefused={refused} />
				{types.isError && (
					<ErrorNotice>{types.data === undefined ? typesLost : typesStale}</ErrorNotice>
				)}
				{overlaps.map((held) => {
					const said = shadowingSentence(held)
					return (
						<Notice.Root key={`${held.key}:${held.winner}:${held.loserID}`} intent="warning">
							<Notice.Description>{said}</Notice.Description>
						</Notice.Root>
					)
				})}
				<GroupsBody
					groups={groups.data ?? []}
					types={types.data ?? []}
					shadowed={new Set(overlaps.map((held) => held.loserID))}
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
	props: Reporter & {
		groups: FieldGroup[]
		types: ContentType[]
		shadowed: Set<number>
		loading: boolean
		failed: boolean
	},
) {
	if (props.failed) {
		return <ErrorNotice>{__('The field groups could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	if (props.loading) {
		return <LoadingScreen label={__('Loading field groups.', 'gophenberg')} />
	}
	return (
		<div
			className="godmin-table-scroll"
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
							groups={props.groups}
							types={props.types}
							shadowed={props.shadowed.has(held.id)}
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
	props: Reporter & {
		held: FieldGroup
		groups: FieldGroup[]
		types: ContentType[]
		shadowed: boolean
		order: number[]
		at: number
	},
) {
	const { held } = props
	const origin = held.origin ?? ''
	const declared = origin !== ''
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
			<td>{formatNumber(held.fields.length)}</td>
			<td>
				<Stack direction="row" gap="xs">
					{!held.active && <Badge intent="none">{__('Inactive', 'gophenberg')}</Badge>}
					{props.shadowed && <Badge intent="high">{__('Shadowed', 'gophenberg')}</Badge>}
					{declared && (
						<Badge>{sprintf(__('From %(plugin)s', 'gophenberg'), { plugin: origin })}</Badge>
					)}
				</Stack>
			</td>
			<td>
				<Stack direction="row" gap="xs">
					<MoveGroup held={held} order={props.order} at={props.at} pending={move.isPending} onMove={move.mutate} />
					{declared ? (
						<PluginFieldsDialog held={held} origin={origin} />
					) : (
						<GroupFieldsDialog
							held={held}
							groups={props.groups}
							types={props.types}
							onDone={props.onDone}
						/>
					)}
					{!declared && (
						<RulesDialog
							held={held}
							groups={props.groups}
							types={props.types}
							onDone={props.onDone}
							onRefused={props.onRefused}
						/>
					)}
					<Button variant="outline" onClick={() => edit.mutate({ active: !held.active })}>
						{held.active ? __('Deactivate', 'gophenberg') : __('Activate', 'gophenberg')}
					</Button>
					{!declared && <RemoveGroup held={held} onDone={props.onDone} onRefused={props.onRefused} />}
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
 * Renders the control removing a group and, while it asks, the red confirm that removes it.
 * @param props - The group and the reporter.
 * @returns The control and its confirm, busy until the delete ends.
 */
function RemoveGroup(props: Reporter & { held: FieldGroup }) {
	const [asking, setAsking] = useState(false)
	const remove = useMutation({ mutationFn: () => deleteGroup(props.held.id) })
	const said = sprintf(__('%(group)s removed.', 'gophenberg'), { group: props.held.title })
	const warning = __(
		'The fields this group holds go with it, and so do the values every item stored in them.',
		'gophenberg',
	)
	return (
		<>
			<Button variant="outline" onClick={() => setAsking(true)}>
				{__('Delete', 'gophenberg')}
			</Button>
			{asking ? (
				<ConfirmDialog
					intent="irreversible"
					title={sprintf(__('Delete "%(group)s"?', 'gophenberg'), { group: props.held.title })}
					description={`${warning} ${irreversible()}`}
					confirmLabel={__('Delete the group', 'gophenberg')}
					cancelLabel={__('Cancel', 'gophenberg')}
					onConfirm={() => remove.mutateAsync().then(() => { props.onDone(said) }, props.onRefused)}
					onClose={() => setAsking(false)}
				/>
			) : null}
		</>
	)
}

/**
 * Renders the link downloading the site's content definitions as a file.
 * @returns The download link, dressed as a button.
 */
function ExportDefinitions() {
	return (
		<Button variant="outline" size="compact" render={<a href={definitionsDownloadPath} download />}>
			{__('Export definitions', 'gophenberg')}
		</Button>
	)
}

/** What the control storing a new field group takes. */
interface AddGroupProps {
	types: ContentType[]
	onDone: (said: string) => void
}

/**
 * Renders the control storing a new field group and, while it asks, its form.
 * @param props - The registered types and what reports the group created.
 * @returns The control and its form.
 */
function AddGroup(props: AddGroupProps) {
	const [asking, setAsking] = useState(false)
	return (
		<>
			<Button variant="solid" size="compact" onClick={() => setAsking(true)}>
				{__('Add Field Group', 'gophenberg')}
			</Button>
			{asking ? <AddGroupDialog {...props} onClose={() => setAsking(false)} /> : null}
		</>
	)
}

/**
 * Renders the form storing a new field group, its title empty on each ask and kept after a failure.
 * @param props - The registered types, what reports the group created and what takes the form away.
 * @returns The form dialog.
 */
function AddGroupDialog(props: AddGroupProps & { onClose: () => void }) {
	const [title, setTitle] = useState('')
	const firstType = props.types.at(0)
	const seeded = firstType?.key ?? ''
	const add = useMutation({
		mutationFn: () => createGroup(title, [[{ source: typeSource, operator: '==', value: seeded }]]),
	})
	const create = () =>
		add.mutateAsync().then(
			() => {
				props.onDone(sprintf(__('%(group)s created.', 'gophenberg'), { group: title }))
			},
			(cause: unknown) => ({
				error: sprintf(__('Could not save the change. %s', 'gophenberg'), groupErrorMessage(cause)),
			}),
		)
	return (
		<FormDialog
			title={__('Create a field group', 'gophenberg')}
			description={sprintf(
				__('It starts on %(type)s, and the rules may be edited afterwards.', 'gophenberg'),
				{ type: firstType === undefined ? __('a content type', 'gophenberg') : pluralOf(firstType) },
			)}
			submitLabel={__('Create', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={create}
			onClose={props.onClose}
		>
			{(busy) => (
				<InputControl
					label={__('Title', 'gophenberg')}
					autoComplete="off"
					value={title}
					readOnly={busy}
					onValueChange={setTitle}
				/>
			)}
		</FormDialog>
	)
}
