// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, SelectControl } from '@gophenberg/frontend-sdk'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import type { ReactNode } from 'react'

import { backlinkSettings, backlinkSources, createFieldInGroup, createSubField } from './groups'
import { SourceOfLinks } from './SourceOfLinks'
import { chosenOf } from './select'
import { pluralOf } from './typeText'
import { kindsInside, pickedKind, requirable, slugifyKey } from './types'
import type { Reporter } from './fieldControls'
import type { BacklinkSources, FieldGroup } from './groups'
import type { Choice } from './select'
import type { ContentType, NewField, PickedKind } from './types'

/** Where a new field is declared: the group, the types, the groups a backlinks field reads, and the container. */
export interface FieldPlace {
	group: number
	types: ContentType[]
	groups?: FieldGroup[]
	path?: string
	parent?: string
}

/** A new field as its controls hold it, and the write declaring it. */
interface FieldDraft {
	controls: ReactNode
	blocked: boolean
	pending: boolean
	declare: (signal?: AbortSignal) => Promise<unknown>
	said: string
	clear: () => void
}

/**
 * Renders the form declaring a new field at the top of the group, its controls in one row and its button after them.
 * @param props - The group, the groups a backlinks field reads, the types, and what to report.
 * @returns The form.
 */
export function AddField(props: Reporter & FieldPlace) {
	const draft = useFieldDraft(props)

	/** Declares the field, empties its name once it landed, and reports how the write went. */
	function declare() {
		const said = draft.said
		void draft.declare().then(
			async () => {
				draft.clear()
				await props.onDone(said)
			},
			(cause: unknown) => props.onRefused(cause, 'add'),
		)
	}

	return (
		<form
			className="godmin-form godmin-form--inline"
			onSubmit={(event) => {
				event.preventDefault()
				declare()
			}}
		>
			<div className="godmin-form__row">{draft.controls}</div>
			<Button type="submit" loading={draft.pending} disabled={draft.blocked || draft.pending}>
				{__('Add field', 'gophenberg')}
			</Button>
		</form>
	)
}

/** What the controls of a new field hold: its name, the kind picked, and the answers that kind asks for. */
interface DraftAnswers {
	label: string
	kind: Choice
	picked: PickedKind
	target: Choice | undefined
	holding: Choice
	presence: Choice
	sources: BacklinkSources
}

/** The choices each select of a new field offers. */
interface DraftOffers {
	kinds: Choice[]
	targets: Choice[]
	holdings: Choice[]
	presences: Choice[]
}

/** What changes each answer the controls of a new field hold. */
interface DraftChanges {
	label: (label: string) => void
	kind: (kind: Choice) => void
	target: (key: string) => void
	holding: (holding: Choice) => void
	presence: (presence: Choice) => void
	sourceGroup: (key: string) => void
	sourceField: (key: string) => void
}

/**
 * Holds the controls declaring a new field and the write that stores what they hold.
 * @param props - Where the field is declared.
 * @returns The controls, read only while the write runs, whether they may be sent yet, and the write.
 */
export function useFieldDraft(props: FieldPlace): FieldDraft {
	const [kinds] = useState(() => kindsInside(props.parent))
	const targets = props.types.map((listed) => ({ label: pluralOf(listed), value: listed.key }))
	const [presences] = useState<Choice[]>([
		{ label: __('No', 'gophenberg'), value: 'optional' },
		{ label: __('Yes', 'gophenberg'), value: 'required' },
	])
	const [holdings] = useState<Choice[]>([
		{ label: __('Many targets', 'gophenberg'), value: 'many' },
		{ label: __('One target', 'gophenberg'), value: 'one' },
	])
	const [label, setLabel] = useState('')
	const [kind, setKind] = useState(kinds[0])
	const [targetKey, setTargetKey] = useState('')
	const [presence, setPresence] = useState(presences[0])
	const [holding, setHolding] = useState(holdings[0])
	const [sourceKey, setSourceKey] = useState('')
	const [throughKey, setThroughKey] = useState('')
	const draft: DraftAnswers = {
		label,
		kind,
		picked: pickedKind(kind.value),
		target: targets.find((held) => held.value === targetKey) ?? targets[0],
		holding,
		presence,
		sources: backlinkSources(props.groups ?? [], sourceKey, throughKey),
	}
	const add = useMutation({
		mutationFn: (signal: AbortSignal | undefined) => {
			const asked = askedField(draft)
			return props.path === undefined
				? createFieldInGroup(props.group, asked, signal)
				: createSubField(props.group, props.path, asked, signal)
		},
	})
	const change: DraftChanges = {
		label: setLabel,
		kind: setKind,
		target: setTargetKey,
		holding: setHolding,
		presence: setPresence,
		sourceGroup: setSourceKey,
		sourceField: setThroughKey,
	}
	return {
		controls: (
			<DraftControls
				draft={draft}
				offers={{ kinds, targets, holdings, presences }}
				change={change}
				readOnly={add.isPending}
			/>
		),
		blocked: draftBlocked(draft),
		pending: add.isPending,
		declare: (signal) => add.mutateAsync(signal),
		said: sprintf(__('%(field)s declared.', 'gophenberg'), { field: label }),
		clear: () => setLabel(''),
	}
}

/**
 * Reports whether a new field still lacks the answer its kind needs before it may be sent.
 * @param draft - What the controls hold.
 * @returns Whether sending waits.
 */
function draftBlocked(draft: DraftAnswers): boolean {
	if (draft.picked.kind === 'relation') {
		return draft.target === undefined
	}
	return draft.picked.kind === 'backlinks' && draft.sources.field.value === ''
}

/**
 * Returns the field the controls ask to declare.
 * @param draft - What the controls hold.
 * @returns The field to declare.
 */
function askedField(draft: DraftAnswers): NewField {
	const { label, picked } = draft
	const relating = picked.kind === 'relation'
	return {
		key: slugifyKey(label),
		label,
		kind: picked.kind,
		relatesTo: relating ? draft.target?.value : undefined,
		many: relating ? draft.holding.value === 'many' : picked.many,
		required: requirable(picked.kind) && draft.presence.value === 'required',
		settings: picked.kind === 'backlinks' ? backlinkSettings(draft.sources) : picked.settings,
	}
}

/**
 * Renders the controls of a new field: its name, its kind, and what that kind asks for.
 * @param props - What the controls hold, the choices they offer, what changes each, and whether they are read only.
 * @returns The controls.
 */
function DraftControls(props: { draft: DraftAnswers; offers: DraftOffers; change: DraftChanges; readOnly: boolean }) {
	const { draft, offers, change, readOnly } = props
	const { kind, target, holding, presence } = draft
	const relating = draft.picked.kind === 'relation'
	return (
		<>
			<InputControl
				className="godmin-form__grow"
				label={_x('Name', 'field', 'gophenberg')}
				autoComplete="off"
				value={draft.label}
				readOnly={readOnly}
				onValueChange={change.label}
			/>
			<SelectControl
				label={_x('Kind', 'field type', 'gophenberg')}
				items={offers.kinds}
				value={kind}
				readOnly={readOnly}
				onValueChange={(item) => change.kind(chosenOf(item, offers.kinds, kind))}
			/>
			{relating && target !== undefined && (
				<SelectControl
					label={__('Points at', 'gophenberg')}
					items={offers.targets}
					value={target}
					readOnly={readOnly}
					onValueChange={(item) => change.target(chosenOf(item, offers.targets, target).value)}
				/>
			)}
			{relating && (
				<SelectControl
					label={__('Holds', 'gophenberg')}
					items={offers.holdings}
					value={holding}
					readOnly={readOnly}
					onValueChange={(item) => change.holding(chosenOf(item, offers.holdings, holding))}
				/>
			)}
			{draft.picked.kind === 'backlinks' && (
				<SourceOfLinks
					sources={draft.sources}
					readOnly={readOnly}
					onGroup={change.sourceGroup}
					onField={change.sourceField}
				/>
			)}
			{requirable(draft.picked.kind) && (
				<SelectControl
					label={__('Required', 'gophenberg')}
					items={offers.presences}
					value={presence}
					readOnly={readOnly}
					onValueChange={(item) => change.presence(chosenOf(item, offers.presences, presence))}
				/>
			)}
		</>
	)
}
