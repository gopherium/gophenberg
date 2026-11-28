// SPDX-License-Identifier: Apache-2.0

import { InputControl, Notice, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice, SectionTitle } from '@gopherium/godmin'
import { formatDate } from '@gopherium/gottext'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import { TickBox } from './fieldControls'
import { changedElsewhere, changedParts, draftOf, editedParts, rebased, withLivePicks, withPick } from './groupDraft'
import {
	groupErrorMessage,
	groupsQueryKey,
	listRuleSources,
	ruleSourcesQueryKey,
	StaleWriteError,
	staleGroupMessage,
} from './groups'
import { LinkedSources, LocationRules } from './LocationRules'
import type { DraftPart, GroupDraft } from './groupDraft'
import type { FieldGroup } from './groups'
import type { ContentType } from './types'

/** A draft of a group's details and the group as read when the draft began. */
interface Held {
	base: FieldGroup
	draft: GroupDraft
}

/** The details of a group as its page edits them, what moves them onto a saved version and what a save tells them. */
export interface GroupDetailsState {
	/** The group the draft stands on, whose stamp a save sends. */
	base: FieldGroup
	draft: GroupDraft
	changed: boolean
	stale: boolean
	loading: boolean
	failure: string
	lost: DraftPart[]
	edit: (next: GroupDraft) => void
	load: () => void
	/** Takes the notices of the last save away as the next one starts. */
	saveStarted: () => void
	/** Follows the group as read again once a save stored the draft. */
	saveLanded: () => void
	/** Keeps the draft and shows why the group was not saved. */
	saveFailed: (cause: unknown) => void
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
 * Returns the draft of a group's details, the move onto a saved version, and what a save reports back to.
 * @param held - The group as last read.
 * @returns The details state.
 */
export function useGroupDetails(held: FieldGroup): GroupDetailsState {
	const client = useQueryClient()
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

	/** Takes the notices of the last save away as the next one starts. */
	function saveStarted() {
		setFailure('')
		setStale(false)
	}

	/** Follows the group as read again once a save stored the draft. */
	function saveLanded() {
		setKept(null)
		setLost([])
	}

	/**
	 * Keeps the draft and shows why the group was not saved: someone saved it first, or the server's words.
	 * @param cause - What the save of the group failed with.
	 */
	function saveFailed(cause: unknown) {
		if (cause instanceof StaleWriteError) {
			setStale(true)
			return
		}
		setFailure(groupErrorMessage(cause))
	}

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
		base,
		draft,
		changed: parts.length > 0,
		stale,
		loading: load.isPending,
		failure,
		lost,
		edit,
		load: () => load.mutate(),
		saveStarted,
		saveLanded,
		saveFailed,
	}
}

/** What the details column of a group's page takes. */
interface GroupDetailsProps {
	form: string
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[]
	details: GroupDetailsState
	saving: boolean
	onSave: () => void
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
 * @param props - The id the header Save submits, the group as read, every group, the types, the details state,
 * whether the header Save runs, and what runs it.
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
					props.onSave()
				}}
			>
				<InputControl
					label={__('Title', 'gophenberg')}
					autoComplete="off"
					value={draft.title}
					readOnly={props.saving}
					description={details.lost.includes('title') ? changedElsewhere() : undefined}
					onValueChange={(title) => details.edit({ ...draft, title })}
				/>
				<TickBox
					label={_x('Active', 'field group', 'gophenberg')}
					checked={draft.active}
					readOnly={props.saving}
					onChange={(active) => details.edit({ ...draft, active })}
				/>
				<SectionTitle>{__('Location rules', 'gophenberg')}</SectionTitle>
				{details.lost.includes('location') && <Text variant="body-sm">{changedElsewhere()}</Text>}
				<LocationRules
					draft={draft.location}
					sources={sources.data ?? []}
					types={props.types}
					failed={sources.isError}
					readOnly={props.saving}
					onDraft={(location) => details.edit({ ...draft, location })}
				/>
				<LinkedSources
					held={props.held}
					groups={props.groups}
					picked={draft.links}
					readOnly={props.saving}
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
