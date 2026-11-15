// SPDX-License-Identifier: Apache-2.0

import { Checkbox, InputControl, Notice, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice, SectionTitle, useToaster } from '@gopherium/godmin'
import { formatDate } from '@gopherium/gottext'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import {
	changedElsewhere,
	changedParts,
	draftOf,
	editedParts,
	editOf,
	rebased,
	withLivePicks,
	withPick,
} from './groupDraft'
import {
	groupErrorMessage,
	groupsQueryKey,
	listRuleSources,
	ruleSourcesQueryKey,
	StaleWriteError,
	staleGroupMessage,
	updateGroup,
} from './groups'
import { typesQueryKey } from './nav'
import { LinkedSources, namedSources, RulesBody } from './RulesDialog'
import type { DraftPart, GroupDraft } from './groupDraft'
import type { FieldGroup } from './groups'
import type { ContentType } from './types'

/** A draft of a group's details and the group as read when the draft began. */
interface Held {
	base: FieldGroup
	draft: GroupDraft
}

/** The details of a group as its page edits them, and what saves them or moves them onto a saved version. */
export interface GroupDetailsState {
	draft: GroupDraft
	changed: boolean
	saving: boolean
	stale: boolean
	loading: boolean
	failure: string
	lost: DraftPart[]
	edit: (next: GroupDraft) => void
	save: () => void
	load: () => void
}

/**
 * Returns the groups with one group's details as the server stored them and its fields as held.
 * @param listed - The groups held, none before the first read.
 * @param stored - The group the server answered with.
 * @returns The groups to hold.
 */
function withStored(listed: FieldGroup[] | undefined, stored: FieldGroup): FieldGroup[] | undefined {
	return listed?.map((one) => (one.id === stored.id ? { ...one, ...stored, fields: one.fields } : one))
}

/**
 * Returns the draft held with only the picks the group as last read still offers, or nothing once it holds no change.
 * @param kept - The draft held and the group it stands on, none while nothing is held.
 * @param held - The group as last read.
 * @returns The draft to edit and the group it stands on, or nothing while the details follow the group as read.
 */
function liveDraft(kept: Held | null, held: FieldGroup): Held | null {
	if (kept === null) {
		return null
	}
	const draft = withLivePicks(kept.draft, held)
	return changedParts(kept.base, draft).length === 0 ? null : { base: kept.base, draft }
}

/**
 * Returns the draft of a group's details, the save sending its changed parts and the move onto a saved version.
 * @param held - The group as last read.
 * @param groups - Every group read, which a Linked from pick may stand in.
 * @returns The details state.
 */
export function useGroupDetails(held: FieldGroup, groups: FieldGroup[]): GroupDetailsState {
	const client = useQueryClient()
	const toaster = useToaster()
	const [kept, setKept] = useState<Held | null>(null)
	const [stale, setStale] = useState(false)
	const [failure, setFailure] = useState('')
	const [lost, setLost] = useState<DraftPart[]>([])
	const live = liveDraft(kept, held)
	const base = live?.base ?? held
	const draft = live?.draft ?? draftOf(held)
	const parts = changedParts(base, draft)

	/**
	 * Holds a draft, or follows the group as read again once the draft holds no change.
	 * @param from - The group the draft stands on.
	 * @param next - The draft.
	 */
	function hold(from: FieldGroup, next: GroupDraft) {
		setKept(changedParts(from, next).length === 0 ? null : { base: from, draft: next })
	}

	/**
	 * Holds the draft as edited, dropping the line on each part the edit touched and the notices once nothing changed.
	 * @param next - The draft as edited.
	 */
	function edit(next: GroupDraft) {
		const touched = editedParts(draft, next)
		setLost((named) => named.filter((part) => !touched.includes(part)))
		if (changedParts(base, next).length === 0) {
			setStale(false)
			setFailure('')
		}
		hold(base, next)
	}

	const save = useMutation({
		mutationFn: () => updateGroup(base, editOf(draft, parts, held, groups)),
		onMutate: () => {
			setFailure('')
			setStale(false)
		},
		onSuccess: async (stored) => {
			client.setQueryData<FieldGroup[]>(groupsQueryKey, (listed) => withStored(listed, stored))
			setKept(null)
			setLost([])
			toaster.show(sprintf(__('%(group)s updated.', 'gophenberg'), { group: stored.title }))
			await Promise.all([
				client.invalidateQueries({ queryKey: groupsQueryKey }),
				client.invalidateQueries({ queryKey: typesQueryKey }),
			])
		},
		onError: (cause: unknown) => {
			const said = cause instanceof StaleWriteError ? staleGroupMessage(base.title) : groupErrorMessage(cause)
			toaster.show(sprintf(__('Could not save the field group. %s', 'gophenberg'), said))
			if (cause instanceof StaleWriteError) {
				setStale(true)
				return
			}
			setFailure(said)
		},
	})
	const load = useMutation({
		mutationFn: async () => {
			await client.refetchQueries({ queryKey: groupsQueryKey }, { throwOnError: true })
			return client.getQueryData<FieldGroup[]>(groupsQueryKey)?.find((one) => one.id === base.id)
		},
		onMutate: () => setFailure(''),
		onSuccess: (saved) => {
			if (saved === undefined) {
				return
			}
			const moved = rebased(base, draft, saved)
			hold(saved, moved.draft)
			setLost(moved.lost)
			setStale(false)
		},
	})
	return {
		draft,
		changed: parts.length > 0,
		saving: save.isPending,
		stale,
		loading: load.isPending,
		failure,
		lost,
		edit,
		save: () => save.mutate(),
		load: () => load.mutate(),
	}
}

/** What the details column of a group's page takes. */
interface GroupDetailsProps {
	form: string
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[]
	details: GroupDetailsState
}

/**
 * Renders the notices the header Save leaves: someone saved the group first, or the save failed.
 * @param props - The group as read and the details state.
 * @returns The notices, or nothing while the last save left none.
 */
export function SaveNotices(props: { held: FieldGroup; details: GroupDetailsState }) {
	const { details } = props
	return (
		<>
			{details.stale && <StaleNotice title={props.held.title} loading={details.loading} onLoad={details.load} />}
			{details.failure !== '' && <ErrorNotice>{details.failure}</ErrorNotice>}
		</>
	)
}

/**
 * Renders a group's title, active flag, location rules and Linked from sources as one form, and the day it was made.
 * @param props - The id the header Save submits, the group as read, every group, the types and the details state.
 * @returns The details column.
 */
export function GroupDetails(props: GroupDetailsProps) {
	const { details } = props
	const { draft } = details
	const sources = useQuery({ queryKey: ruleSourcesQueryKey, queryFn: listRuleSources })
	return (
		<>
			<form
				id={props.form}
				className="godmin-form"
				onSubmit={(event) => {
					event.preventDefault()
					details.save()
				}}
			>
				<InputControl
					label={__('Title', 'gophenberg')}
					autoComplete="off"
					value={draft.title}
					readOnly={details.saving}
					description={details.lost.includes('title') ? changedElsewhere() : undefined}
					onValueChange={(title) => details.edit({ ...draft, title })}
				/>
				<ActiveBox
					checked={draft.active}
					readOnly={details.saving}
					onChange={(active) => details.edit({ ...draft, active })}
				/>
				<SectionTitle>{__('Location rules', 'gophenberg')}</SectionTitle>
				{details.lost.includes('location') && <Text variant="body-sm">{changedElsewhere()}</Text>}
				<RulesBody
					draft={draft.location}
					sources={namedSources(sources.data ?? [], props.types)}
					failed={sources.isError}
					readOnly={details.saving}
					onDraft={(location) => details.edit({ ...draft, location })}
				/>
				<LinkedSources
					held={props.held}
					groups={props.groups}
					picked={draft.links}
					readOnly={details.saving}
					lost={details.lost}
					onPick={(field, pick) => details.edit(withPick(draft, field, pick))}
				/>
			</form>
			<Text className="gophenberg-field-group__created">
				{sprintf(__('Created %(date)s', 'gophenberg'), { date: formatDate(props.held.createdAt) })}
			</Text>
		</>
	)
}

/**
 * Renders the box turning a group on and off, labelled by the words beside it.
 * @param props - Whether the group is active, whether the box is read only, and what to call with the new flag.
 * @returns The box with its label.
 */
function ActiveBox(props: { checked: boolean; readOnly: boolean; onChange: (active: boolean) => void }) {
	return (
		<Stack direction="row" gap="sm" align="center" render={<label />}>
			<Checkbox checked={props.checked} readOnly={props.readOnly} onCheckedChange={props.onChange} />
			<Text>{_x('Active', 'field group', 'gophenberg')}</Text>
		</Stack>
	)
}

/**
 * Renders the notice that someone saved the group after the page read it, with the control loading their version.
 * @param props - The title of the group, whether the saved version is on its way, and what loads it.
 * @returns The notice.
 */
function StaleNotice(props: { title: string; loading: boolean; onLoad: () => void }) {
	return (
		<Notice.Root intent="warning" role="alert">
			<Notice.Description>{staleGroupMessage(props.title)}</Notice.Description>
			<Notice.Actions>
				<Notice.ActionButton loading={props.loading} onClick={props.onLoad}>
					{__('Load saved version', 'gophenberg')}
				</Notice.ActionButton>
			</Notice.Actions>
		</Notice.Root>
	)
}
