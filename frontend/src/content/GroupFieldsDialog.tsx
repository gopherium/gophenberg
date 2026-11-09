// SPDX-License-Identifier: Apache-2.0

import {
	Badge,
	Button,
	Dialog,
	IconButton,
	InputControl,
	SelectControl,
	Stack,
	Text,
	downIcon,
	upIcon,
} from '@gophenberg/frontend-sdk'
import { ConfirmDialog, ErrorNotice, FormDialog } from '@gopherium/godmin'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ReactNode } from 'react'

import { irreversible } from '../lists/itemWords'
import {
	backlinkSettings,
	backlinkSources,
	createFieldInGroup,
	createSubField,
	deleteFieldInGroup,
	deleteSubField,
	formAnswer,
	groupErrorMessage,
	groupsQueryKey,
	moveField,
	renameFieldInGroup,
	reorderFieldsInGroup,
	setFieldRequiredInGroup,
	reorderSubFields,
	setFieldSettingsInGroup,
	StaleWriteError,
} from './groups'
import { FieldConditions } from './FieldConditions'
import { SourceOfLinks } from './SourceOfLinks'
import { chosenOf } from './select'
import { pluralOf } from './typeText'
import {
	holdsFields,
	holdsRows,
	kindLabel,
	kindsInside,
	pairsOf,
	pickedKind,
	requirable,
	slugifyKey,
	standsInside,
} from './types'
import type { ChoicePair } from './types'
import { typesQueryKey } from './nav'
import type { BacklinkSources, FieldGroup } from './groups'
import type { Choice } from './select'
import type { ContentField, ContentType, NewField, PickedKind } from './types'

/** What a field control reports back once its write settles. */
interface Reporter {
	onDone: (said: string) => Promise<void>
	onRefused: (cause: unknown) => Promise<void>
}

/** The group a field control acts inside. */
interface Inside extends Reporter {
	group: number
	field: ContentField
	path?: string
}

/**
 * Renders the control managing the fields a group holds.
 * @param props - The group, the other groups, the types, and what to report.
 * @returns The control and its dialog.
 */
export function GroupFieldsDialog(props: {
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[]
	onDone: (said: string) => void
}) {
	const client = useQueryClient()
	const [open, setOpen] = useState(false)
	const [notice, setNotice] = useState('')

	/**
	 * Reports what a field write did and refreshes what the screen holds.
	 * @param said - The sentence naming what was done.
	 */
	async function done(said: string) {
		setNotice('')
		props.onDone(said)
		await Promise.all([
			client.invalidateQueries({ queryKey: groupsQueryKey }),
			client.invalidateQueries({ queryKey: typesQueryKey }),
		])
	}

	/**
	 * Reports why a field write was turned away, where the operator is looking.
	 * @param cause - What the write failed with.
	 */
	async function refused(cause: unknown) {
		setNotice(groupErrorMessage(cause))
		if (cause instanceof StaleWriteError) {
			await client.invalidateQueries({ queryKey: groupsQueryKey })
		}
	}

	/**
	 * Opens the dialog on the stored fields, or closes it.
	 * @param next - Whether the dialog is opening.
	 */
	function change(next: boolean) {
		if (next) {
			setNotice('')
		}
		setOpen(next)
	}

	return (
		<>
			<Button variant="outline" onClick={() => change(true)}>
				{__('Fields', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={change}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Fields of %(group)s', 'gophenberg'), { group: props.held.title })}
						</Dialog.Title>
						<Dialog.CloseIconButton />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
							<FieldsBody
								held={props.held}
								groups={props.groups}
								types={props.types}
								onDone={done}
								onRefused={refused}
							/>
						</Stack>
					</Dialog.Content>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the fields a group holds and the control declaring another.
 * @param props - The group, the other groups, the types, and what to report.
 * @returns The body element.
 */
function FieldsBody(
	props: Reporter & { held: FieldGroup; groups: FieldGroup[]; types: ContentType[] },
) {
	const declared = props.held.fields
	const reorder = useMutation({
		mutationFn: (keys: string[]) => reorderFieldsInGroup(props.held.id, keys),
		onSuccess: () => props.onDone(__('Order stored.', 'gophenberg')),
		onError: props.onRefused,
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
				<ul className="gophenberg-fields__list">
					{declared.map((field, index) => (
						<li key={field.key} aria-label={field.label}>
							<Stack direction="column" gap="xs">
								<Stack direction="row" gap="sm" align="center" justify="space-between">
									<Stack direction="row" gap="xs" align="center">
										<Text>{field.label}</Text>
										<Text variant="body-sm">{kindLabel(field.kind)}</Text>
										{field.required && <Badge>{__('Required', 'gophenberg')}</Badge>}
									</Stack>
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
								<Stack direction="row" gap="xs" align="center">
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
			<AddField
				group={props.held.id}
				types={props.types}
				groups={props.groups}
				onDone={props.onDone}
				onRefused={props.onRefused}
			/>
		</Stack>
	)
}

/** Where a new field is declared: the group, the types, the groups a backlinks field reads, and the container. */
interface FieldPlace {
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
	declare: () => Promise<unknown>
	said: string
	clear: () => void
}

/**
 * Renders the control declaring a new field at the top of the group, its failure reported in the dialog around it.
 * @param props - The group, the groups a backlinks field reads, the types, and what to report.
 * @returns The controls and the button sending them.
 */
function AddField(props: Reporter & FieldPlace) {
	const draft = useFieldDraft(props)

	/** Declares the field, empties its name once it landed, and reports how the write went. */
	function declare() {
		const said = draft.said
		void draft.declare().then(async () => {
			draft.clear()
			await props.onDone(said)
		}, props.onRefused)
	}

	return (
		<Stack direction="column" gap="sm">
			{draft.controls}
			<Button loading={draft.pending} disabled={draft.blocked} onClick={declare}>
				{__('Add field', 'gophenberg')}
			</Button>
		</Stack>
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
function useFieldDraft(props: FieldPlace): FieldDraft {
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
		mutationFn: () => {
			const asked = askedField(draft)
			return props.path === undefined
				? createFieldInGroup(props.group, asked)
				: createSubField(props.group, props.path, asked)
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
		declare: () => add.mutateAsync(),
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

/** One setting a kind takes, as its control edits it. */
interface SettingControl {
	name: string
	label: string
	shape: 'text' | 'number' | 'choice'
}

/**
 * Returns the settings a field takes, in the order the controls show them.
 * @param field - The declared field to tune.
 * @returns The settings to offer.
 */
function settingsOffered(field: ContentField): SettingControl[] {
	const kind = field.kind
	const held: SettingControl[] = [
		{ name: 'instructions', label: __('Instructions', 'gophenberg'), shape: 'text' },
	]
	if (kind === 'media' && field.many) {
		held.push({ name: 'min', label: __('Fewest files', 'gophenberg'), shape: 'number' })
		held.push({ name: 'max', label: __('Most files', 'gophenberg'), shape: 'number' })
	}
	if (kind === 'text') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'text' })
		held.push({ name: 'placeholder', label: __('Placeholder', 'gophenberg'), shape: 'text' })
		held.push({ name: 'maxlength', label: __('Longest', 'gophenberg'), shape: 'number' })
	}
	if (kind === 'number') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'number' })
		held.push({ name: 'min', label: __('Lowest', 'gophenberg'), shape: 'number' })
		held.push({ name: 'max', label: __('Highest', 'gophenberg'), shape: 'number' })
		held.push({ name: 'step', label: __('Steps of', 'gophenberg'), shape: 'number' })
	}
	if (kind === 'boolean') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'choice' })
	}
	if (kind === 'choice') {
		held.push({ name: 'multiple', label: __('Many values', 'gophenberg'), shape: 'choice' })
		held.push({ name: 'allow_custom', label: __('Allow custom', 'gophenberg'), shape: 'choice' })
		held.push({ name: 'allow_null', label: __('Allow empty', 'gophenberg'), shape: 'choice' })
	}
	if (holdsRows(kind)) {
		held.push({ name: 'min', label: __('Fewest rows', 'gophenberg'), shape: 'number' })
		held.push({ name: 'max', label: __('Most rows', 'gophenberg'), shape: 'number' })
	}
	return held
}

/**
 * Returns the settings with the choice pairs the editor holds written in.
 * @param kind - The kind the field holds.
 * @param settings - The settings as the controls stored them.
 * @param pairs - The pairs the editor rows hold.
 * @returns The settings to store.
 */
function pairedSettings(
	kind: string,
	settings: Record<string, unknown>,
	pairs: ChoicePair[],
): Record<string, unknown> {
	if (kind !== 'choice') {
		return settings
	}
	const settled = pairs.filter((pair) => pair.value !== '' && pair.label !== '')
	const held = { ...settings }
	if (settled.length === 0) {
		delete held.choices
		return held
	}
	held.choices = settled
	return held
}

/**
 * Returns the answers a boolean setting offers, the first one leaving it unset.
 * @returns The answers to offer.
 */
function settingAnswers(): Choice[] {
	return [
		{ label: __('None', 'gophenberg'), value: '' },
		{ label: __('Yes', 'gophenberg'), value: 'true' },
		{ label: __('No', 'gophenberg'), value: 'false' },
	]
}

/**
 * Returns the settings as the controls hold them, every value written out.
 * @param offered - The settings the kind takes.
 * @param held - The settings the field carries.
 * @returns The text each control starts with.
 */
function typedSettings(offered: SettingControl[], held: Record<string, unknown>): Record<string, string> {
	const written: Record<string, string> = {}
	for (const setting of offered) {
		const value = held[setting.name]
		written[setting.name] = value === undefined || value === null ? '' : String(value)
	}
	return written
}

/**
 * Returns the settings to store, leaving out every control the operator left empty.
 * @param offered - The settings the kind takes.
 * @param typed - The text the controls hold.
 * @param held - The settings the field carries.
 * @returns The settings as the registry stores them.
 */
function storedSettings(
	offered: SettingControl[],
	typed: Record<string, string>,
	held: Record<string, unknown>,
): Record<string, unknown> {
	const settings: Record<string, unknown> = { ...held }
	for (const setting of offered) {
		const written = typed[setting.name].trim()
		if (written === '') {
			delete settings[setting.name]
			continue
		}
		settings[setting.name] = settingHeld(setting.shape, written)
	}
	return settings
}

/**
 * Returns what a control holds as the shape its setting stores.
 * @param shape - The shape the setting takes.
 * @param written - The text the control holds.
 * @returns The value to store.
 */
function settingHeld(shape: SettingControl['shape'], written: string): unknown {
	if (shape === 'number') {
		return Number(written)
	}
	if (shape === 'choice') {
		return written === 'true'
	}
	return written
}

/** What a form over one field takes beyond the field: its title and what takes it away. */
interface Asked {
	title: string
	onClose: () => void
}

/**
 * Renders a compact outline button and, while it asks, the dialog it opens.
 * @param props - The words on the button, the title it reads out, and what draws the dialog given that title and
 * what takes it away.
 * @returns The button and its dialog.
 */
function DialogButton(props: { words: string; title: string; dialog: (asked: Asked) => ReactNode }) {
	const [asking, setAsking] = useState(false)
	return (
		<>
			<Button variant="outline" size="compact" aria-label={props.title} onClick={() => setAsking(true)}>
				{props.words}
			</Button>
			{asking ? props.dialog({ title: props.title, onClose: () => setAsking(false) }) : null}
		</>
	)
}

/**
 * Renders the control editing the settings a field carries and, while it asks, its form.
 * @param props - The group, the field, and what to report.
 * @returns The control and its form.
 */
function FieldSettings(props: Inside) {
	return (
		<DialogButton
			words={__('Settings', 'gophenberg')}
			title={sprintf(__('Settings of %(field)s', 'gophenberg'), { field: props.field.label })}
			dialog={(asked) => <SettingsForm {...props} {...asked} />}
		/>
	)
}

/**
 * Renders the form editing the settings a field carries, started from the field as read on each ask.
 * @param props - The group, the field, what to report, the title and what takes the form away.
 * @returns The form dialog, kept open with what was typed after a failure other than a stale write.
 */
function SettingsForm(props: Inside & Asked) {
	const [opened] = useState(props.field)
	const offered = settingsOffered(opened)
	const answers = settingAnswers()
	const [typed, setTyped] = useState(() => typedSettings(offered, opened.settings))
	const [pairs, setPairs] = useState(() => pairsOf(opened.settings))
	const save = useMutation({
		mutationFn: () =>
			setFieldSettingsInGroup(
				props.group,
				props.path ?? opened.key,
				pairedSettings(opened.kind, storedSettings(offered, typed, opened.settings), pairs),
				opened.updatedAt,
			),
	})
	const said = sprintf(__('%(field)s settled.', 'gophenberg'), { field: props.field.label })
	return (
		<FormDialog
			title={props.title}
			submitLabel={__('Save settings', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={() => formAnswer(save.mutateAsync(), said, props)}
			onClose={props.onClose}
		>
			{(busy) => (
				<>
					{offered.map((setting) =>
						setting.shape === 'choice' ? (
							<SelectControl
								key={setting.name}
								label={setting.label}
								items={answers}
								value={chosenOf({ value: typed[setting.name] }, answers, answers[0])}
								readOnly={busy}
								onValueChange={(item) =>
									setTyped((was) => ({
										...was,
										[setting.name]: chosenOf(item, answers, answers[0]).value,
									}))
								}
							/>
						) : (
							<InputControl
								key={setting.name}
								label={setting.label}
								type={setting.shape}
								autoComplete="off"
								value={typed[setting.name]}
								readOnly={busy}
								onValueChange={(written) => setTyped((was) => ({ ...was, [setting.name]: written }))}
							/>
						),
					)}
					{opened.kind === 'choice' && <ChoicesEditor pairs={pairs} readOnly={busy} onChange={setPairs} />}
				</>
			)}
		</FormDialog>
	)
}

/**
 * Renders the rows editing the value and label pairs a choice field offers.
 * @param props - The pairs held, whether they are read only, and what to call with an edit.
 * @returns The editor element.
 */
function ChoicesEditor(props: { pairs: ChoicePair[]; readOnly: boolean; onChange: (pairs: ChoicePair[]) => void }) {
	return (
		<Stack direction="column" gap="sm">
			<Text variant="body-sm">{__('Choices', 'gophenberg')}</Text>
			{props.pairs.map((pair, at) => (
				<Stack key={at} direction="row" gap="sm">
					<InputControl
						label={__('Value', 'gophenberg')}
						autoComplete="off"
						value={pair.value}
						readOnly={props.readOnly}
						onValueChange={(written) =>
							props.onChange(props.pairs.map((held, row) =>
								row === at ? { ...held, value: written } : held,
							))
						}
					/>
					<InputControl
						label={__('Label', 'gophenberg')}
						autoComplete="off"
						value={pair.label}
						readOnly={props.readOnly}
						onValueChange={(written) =>
							props.onChange(props.pairs.map((held, row) =>
								row === at ? { ...held, label: written } : held,
							))
						}
					/>
					<Button
						variant="outline"
						size="compact"
						disabled={props.readOnly}
						onClick={() => props.onChange(props.pairs.filter((_, row) => row !== at))}
					>
						{__('Remove', 'gophenberg')}
					</Button>
				</Stack>
			))}
			<Button
				variant="outline"
				disabled={props.readOnly}
				onClick={() => props.onChange([...props.pairs, { value: '', label: '' }])}
			>
				{__('Add choice', 'gophenberg')}
			</Button>
		</Stack>
	)
}

/**
 * Renders the control flipping whether a field gates publishing.
 * @param props - The group, the field, and what to report.
 * @returns The control element.
 */
function RequireField(props: Inside) {
	const flip = useMutation({
		mutationFn: () =>
			setFieldRequiredInGroup(
				props.group,
				props.path ?? props.field.key,
				!props.field.required,
				props.field.updatedAt,
			),
		onSuccess: async () => {
			const said = props.field.required
				? __('%(field)s is optional again.', 'gophenberg')
				: __('%(field)s is required now.', 'gophenberg')
			await props.onDone(sprintf(said, { field: props.field.label }))
		},
		onError: props.onRefused,
	})
	const asking = props.field.required
		? sprintf(__('Make %(field)s optional', 'gophenberg'), { field: props.field.label })
		: sprintf(__('Require %(field)s', 'gophenberg'), { field: props.field.label })
	if (!requirable(props.field.kind)) {
		return null
	}
	return (
		<Button
			variant="outline"
			size="compact"
			aria-label={asking}
			loading={flip.isPending}
			onClick={() => flip.mutate()}
		>
			{props.field.required ? __('Make optional', 'gophenberg') : __('Require', 'gophenberg')}
		</Button>
	)
}

/** The kinds the admin list can show as a column. */
const LISTABLE = ['text', 'number', 'boolean', 'date', 'choice']

/**
 * Renders the control flipping whether a field stands as a column on the admin list.
 * @param props - The group, the field, and what to report.
 * @returns The control element, or nothing where the list never reaches.
 */
function ListField(props: Inside) {
	const listed = props.field.settings.listed === true
	const flip = useMutation({
		mutationFn: () =>
			setFieldSettingsInGroup(
				props.group,
				props.path ?? props.field.key,
				{ ...props.field.settings, listed: !listed },
				props.field.updatedAt,
			),
		onSuccess: async () => {
			const said = listed
				? __('%(field)s stays out of the list.', 'gophenberg')
				: __('%(field)s stands in the list.', 'gophenberg')
			await props.onDone(sprintf(said, { field: props.field.label }))
		},
		onError: props.onRefused,
	})
	if (props.path !== undefined || !LISTABLE.includes(props.field.kind)) {
		return null
	}
	const asking = listed
		? sprintf(__('Keep %(field)s out of the list', 'gophenberg'), { field: props.field.label })
		: sprintf(__('Show %(field)s in the list', 'gophenberg'), { field: props.field.label })
	return (
		<Button
			variant="outline"
			size="compact"
			aria-label={asking}
			loading={flip.isPending}
			onClick={() => flip.mutate()}
		>
			{listed ? __('Out of list', 'gophenberg') : __('In list', 'gophenberg')}
		</Button>
	)
}

/**
 * Renders the control carrying a new label for a field and, while it asks, its form.
 * @param props - The group, the field, and what to report.
 * @returns The control and its form.
 */
function RenameField(props: Inside) {
	return (
		<DialogButton
			words={__('Rename', 'gophenberg')}
			title={sprintf(__('Rename %(field)s', 'gophenberg'), { field: props.field.label })}
			dialog={(asked) => <RenameForm {...props} {...asked} />}
		/>
	)
}

/**
 * Renders the form carrying a new label for a field, started from the field as read on each ask.
 * @param props - The group, the field, what to report, the title and what takes the form away.
 * @returns The form dialog, kept open with the name typed after a failure other than a stale write.
 */
function RenameForm(props: Inside & Asked) {
	const [opened] = useState(props.field)
	const [label, setLabel] = useState(opened.label)
	const rename = useMutation({
		mutationFn: () => renameFieldInGroup(props.group, props.path ?? opened.key, label, opened.updatedAt),
	})
	return (
		<FormDialog
			title={props.title}
			description={__('The name changes. Nothing stored under this field moves.', 'gophenberg')}
			submitLabel={__('Rename', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={() =>
				formAnswer(
					rename.mutateAsync(),
					sprintf(__('%(field)s renamed.', 'gophenberg'), { field: label }),
					props,
				)
			}
			onClose={props.onClose}
		>
			{(busy) => (
				<InputControl
					label={_x('Name', 'field', 'gophenberg')}
					value={label}
					readOnly={busy}
					onValueChange={setLabel}
				/>
			)}
		</FormDialog>
	)
}

/** Where a field may come to stand: a group's top, or a container inside one, named for the select. */
interface Landing {
	label: string
	value: string
}

/**
 * Returns the containers the site keeps among the fields with the dotted path reaching each, however deep they stand.
 * @param fields - The fields to look through.
 * @param above - The dotted path holding the fields, empty at a group's top.
 * @returns The containers and their paths.
 */
function containersOf(fields: ContentField[], above: string): { label: string; path: string; kind: string }[] {
	const held: { label: string; path: string; kind: string }[] = []
	for (const field of fields) {
		if (!holdsFields(field.kind) || (field.origin ?? '') !== '') {
			continue
		}
		const path = above === '' ? field.key : `${above}.${field.key}`
		held.push({ label: field.label, path, kind: field.kind }, ...containersOf(field.fields, path))
	}
	return held
}

/**
 * Returns whether a container may take the field: not its own place, not itself, and not one of its own.
 * @param own - Whether the container stands in the field's own group.
 * @param path - The container's dotted path.
 * @param at - The field's dotted path.
 * @param parent - The dotted path of the container holding the field, empty at a group's top.
 * @returns Whether the container is offered.
 */
function takes(own: boolean, path: string, at: string, parent: string): boolean {
	if (!own) {
		return true
	}
	return path !== parent && path !== at && !path.startsWith(`${at}.`)
}

/**
 * Returns the places a field may be carried to: the top and every container of each group the site keeps, its own
 * place left out.
 * @param groups - The groups with their fields.
 * @param group - The group the field stands in.
 * @param at - The field's dotted path.
 * @param kind - The kind of the field being carried.
 * @returns The landings in group order, each group's top before its containers.
 */
function landingsFor(groups: FieldGroup[], group: number, at: string, kind: string): Landing[] {
	const parent = at.includes('.') ? at.slice(0, at.lastIndexOf('.')) : ''
	const held: Landing[] = []
	for (const listed of groups.filter((listed) => (listed.origin ?? '') === '')) {
		const own = listed.id === group
		if ((!own || parent !== '') && standsInside(kind)) {
			held.push({ label: listed.title, value: String(listed.id) })
		}
		for (const container of containersOf(listed.fields, '')) {
			if (takes(own, container.path, at, parent) && standsInside(kind, container.kind)) {
				held.push({ label: `${listed.title}: ${container.label}`, value: `${listed.id}:${container.path}` })
			}
		}
	}
	return held
}

/**
 * Renders the control carrying a field to a group's top or into a container.
 * @param props - The group, the field, its path, every group it may land in, and what to report.
 * @returns The control and its dialog, or nothing when there is nowhere to carry it.
 */
function CarryField(props: Inside & { groups: FieldGroup[] }) {
	const at = props.path ?? props.field.key
	const landings = landingsFor(props.groups, props.group, at, props.field.kind)
	const first = landings[0]
	if (first === undefined) {
		return null
	}
	return (
		<CarryDialog
			group={props.group}
			field={props.field}
			at={at}
			landings={landings}
			first={first}
			onDone={props.onDone}
			onRefused={props.onRefused}
		/>
	)
}

/** Where a field may be carried: its path, the landings on offer, and the first of them. */
interface Carried {
	at: string
	landings: Landing[]
	first: Landing
}

/**
 * Renders the control carrying a field elsewhere and, while it asks, the form choosing where it lands.
 * @param props - The group, the field, its path, the landings on offer, the first of them, and what to report.
 * @returns The control and its form.
 */
function CarryDialog(props: Inside & Carried) {
	return (
		<DialogButton
			words={__('Move', 'gophenberg')}
			title={sprintf(__('Move %(field)s elsewhere', 'gophenberg'), { field: props.field.label })}
			dialog={(asked) => <CarryForm {...props} {...asked} />}
		/>
	)
}

/**
 * Renders the form choosing where a field lands and carrying it there, the first landing picked on each ask.
 * @param props - The group, the field, the landings, what to report, the title and what takes the form away.
 * @returns The form dialog, kept open after a failure other than a stale write.
 */
function CarryForm(props: Inside & Carried & Asked) {
	const { at, landings } = props
	const [picked, setPicked] = useState(props.first)
	const landing = landings.find((held) => held.value === picked.value) ?? props.first
	const carry = useMutation({
		mutationFn: () => {
			const [toGroup, toParent] = landing.value.split(':')
			return moveField(props.group, at, Number(toGroup), toParent)
		},
	})
	const said = sprintf(__('%(field)s moved.', 'gophenberg'), { field: props.field.label })
	const follows = !at.includes('.') && !landing.value.includes(':')
	return (
		<FormDialog
			title={props.title}
			description={
				follows
					? __('The field keeps its values on the content both groups reach.', 'gophenberg')
					: __('The values stored under it do not follow.', 'gophenberg')
			}
			submitLabel={__('Move the field', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={() => formAnswer(carry.mutateAsync(), said, props)}
			onClose={props.onClose}
		>
			{(busy) => (
				<SelectControl
					label={__('Destination', 'gophenberg')}
					items={landings}
					value={landing}
					readOnly={busy}
					onValueChange={(item) => setPicked(chosenOf(item, landings, landing))}
				/>
			)}
		</FormDialog>
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
		onError: props.onRefused,
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
							<Stack direction="row" gap="xs" align="center">
								<Text>{field.label}</Text>
								<Text variant="body-sm">{kindLabel(field.kind)}</Text>
								{field.required && <Badge>{__('Required', 'gophenberg')}</Badge>}
							</Stack>
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
						<Stack direction="row" gap="xs" align="center">
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

/**
 * Renders the control declaring a field inside the container and, while it asks, its form.
 * @param props - The group, the container, its path, the types, and what to report.
 * @returns The control and its form, or nothing for a field holding none.
 */
function SubFields(props: Inside & { types: ContentType[]; path: string }) {
	const layouts = props.field.kind === 'flexible'
	const title = sprintf(
		layouts
			? __('Add layout to %(field)s', 'gophenberg')
			: __('Add field to %(field)s', 'gophenberg'),
		{ field: props.field.label },
	)
	const send = layouts ? __('Add layout', 'gophenberg') : __('Add field', 'gophenberg')
	if (!holdsFields(props.field.kind)) {
		return null
	}
	return (
		<DialogButton
			words={send}
			title={title}
			dialog={(asked) => (
				<SubFieldForm
					{...asked}
					group={props.group}
					types={props.types}
					path={props.path}
					parent={props.field.kind}
					send={send}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			)}
		/>
	)
}

/**
 * Renders the form declaring a field inside a container, empty on each ask.
 * @param props - Where the field is declared, what to report, the title, the send label and what takes it away.
 * @returns The form dialog, kept open with what was typed after a failure other than a stale write.
 */
function SubFieldForm(props: Reporter & FieldPlace & Asked & { send: string }) {
	const draft = useFieldDraft(props)
	return (
		<FormDialog
			title={props.title}
			submitLabel={props.send}
			cancelLabel={__('Cancel', 'gophenberg')}
			canSubmit={!draft.blocked}
			onSubmit={() => formAnswer(draft.declare(), draft.said, props)}
			onClose={props.onClose}
		>
			{draft.controls}
		</FormDialog>
	)
}

/**
 * Renders the control removing a field and, while it asks, the red confirm that removes it with its values.
 * @param props - The group, the field, and what to report.
 * @returns The control and its confirm, busy until the delete ends.
 */
function DeleteField(props: Inside) {
	const opener = sprintf(__('Delete %(field)s', 'gophenberg'), { field: props.field.label })
	const warning = __(
		'Every value stored under this field goes with it, in every item the group reaches and in the revisions behind them.',
		'gophenberg',
	)
	const remove = useMutation({
		mutationFn: () =>
			props.path === undefined
				? deleteFieldInGroup(props.group, props.field.key)
				: deleteSubField(props.group, props.path),
	})
	const said = sprintf(__('%(field)s deleted.', 'gophenberg'), { field: props.field.label })
	return (
		<DialogButton
			words={__('Delete', 'gophenberg')}
			title={opener}
			dialog={(asked) => (
				<ConfirmDialog
					intent="irreversible"
					title={sprintf(__('Delete "%(field)s"?', 'gophenberg'), { field: props.field.label })}
					description={`${warning} ${irreversible()}`}
					confirmLabel={__('Delete the field', 'gophenberg')}
					cancelLabel={__('Cancel', 'gophenberg')}
					onConfirm={() =>
						remove.mutateAsync().then(() => {
							void props.onDone(said)
						}, props.onRefused)
					}
					onClose={asked.onClose}
				/>
			)}
		/>
	)
}
