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
import { ErrorNotice } from '@gopherium/godmin'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
	createFieldInGroup,
	createSubField,
	deleteFieldInGroup,
	deleteSubField,
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
import { chosenOf } from './select'
import { fieldKinds, kindLabel, pairsOf, pickedKind, slugifyKey } from './types'
import type { ChoicePair } from './types'
import { typesQueryKey } from './nav'
import type { FieldGroup } from './groups'
import type { Choice } from './select'
import type { ContentField, ContentType } from './types'

/**
 * Reports whether a turned away write left edits the operator still has to mend.
 * @param state - What the write reported.
 * @returns Whether the dialog reopens on those edits.
 */
function holdsEdits(state: { isError: boolean; error: unknown }): boolean {
	return state.isError && !(state.error instanceof StaleWriteError)
}

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
						<Dialog.CloseIcon />
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
										elsewhere={props.groups.filter((listed) => listed.id !== props.held.id)}
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
				onDone={props.onDone}
				onRefused={props.onRefused}
			/>
		</Stack>
	)
}

/**
 * Renders the control declaring a new field into the group.
 * @param props - The group, the types a relation may point at, and what to report.
 * @returns The control element.
 */
function AddField(props: Reporter & { group: number; types: ContentType[]; path?: string }) {
	const [kinds] = useState(fieldKinds)
	const targets = props.types.map((listed) => ({ label: listed.pluralLabel, value: listed.key }))
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
	const target = targets.find((held) => held.value === targetKey) ?? targets[0]
	const picked = pickedKind(kind.value)
	const relating = picked.kind === 'relation'
	const add = useMutation({
		mutationFn: () => {
			const asked = {
				key: slugifyKey(label),
				label,
				kind: picked.kind,
				relatesTo: relating ? target?.value : undefined,
				many: relating ? holding.value === 'many' : picked.many,
				required: presence.value === 'required',
				settings: picked.settings,
			}
			return props.path === undefined
				? createFieldInGroup(props.group, asked)
				: createSubField(props.group, props.path, asked)
		},
		onSuccess: async () => {
			setLabel('')
			await props.onDone(sprintf(__('%(field)s declared.', 'gophenberg'), { field: label }))
		},
		onError: props.onRefused,
	})
	return (
		<Stack direction="column" gap="sm">
			<InputControl
				label={_x('Name', 'field', 'gophenberg')}
				autoComplete="off"
				value={label}
				onValueChange={setLabel}
			/>
			<SelectControl
				label={_x('Kind', 'field type', 'gophenberg')}
				items={kinds}
				value={kind}
				onValueChange={(item) => setKind(chosenOf(item, kinds, kind))}
			/>
			{relating && target !== undefined && (
				<SelectControl
					label={__('Points at', 'gophenberg')}
					items={targets}
					value={target}
					onValueChange={(item) => setTargetKey(chosenOf(item, targets, target).value)}
				/>
			)}
			{relating && (
				<SelectControl
					label={__('Holds', 'gophenberg')}
					items={holdings}
					value={holding}
					onValueChange={(item) => setHolding(chosenOf(item, holdings, holding))}
				/>
			)}
			<SelectControl
				label={__('Required', 'gophenberg')}
				items={presences}
				value={presence}
				onValueChange={(item) => setPresence(chosenOf(item, presences, presence))}
			/>
			<Button
				loading={add.isPending}
				disabled={relating && target === undefined}
				onClick={() => add.mutate()}
			>
				{__('Add field', 'gophenberg')}
			</Button>
		</Stack>
	)
}

/** One setting a kind takes, as its control edits it. */
interface SettingControl {
	name: string
	label: string
	shape: 'text' | 'number' | 'choice'
}

/**
 * Returns the settings a kind takes, in the order the controls show them.
 * @param kind - The kind the field holds.
 * @returns The settings to offer.
 */
function settingsOffered(kind: string): SettingControl[] {
	const held: SettingControl[] = [
		{ name: 'instructions', label: __('Instructions', 'gophenberg'), shape: 'text' },
	]
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

/**
 * Renders the control editing the settings a field carries.
 * @param props - The group, the field, and what to report.
 * @returns The control and its dialog.
 */
function FieldSettings(props: Inside) {
	const offered = settingsOffered(props.field.kind)
	const answers = settingAnswers()
	const [open, setOpen] = useState(false)
	const [typed, setTyped] = useState(() => typedSettings(offered, props.field.settings))
	const [pairs, setPairs] = useState(() => pairsOf(props.field.settings))
	const [opened, setOpened] = useState(props.field)
	const asking = sprintf(__('Settings of %(field)s', 'gophenberg'), { field: props.field.label })
	const save = useMutation({
		mutationFn: () =>
			setFieldSettingsInGroup(
				props.group,
				props.path ?? opened.key,
				pairedSettings(opened.kind, storedSettings(offered, typed, opened.settings), pairs),
				opened.updatedAt,
			),
		onSuccess: async () => {
			setOpen(false)
			await props.onDone(sprintf(__('%(field)s settled.', 'gophenberg'), { field: props.field.label }))
		},
		onError: async (cause) => {
			await props.onRefused(cause)
			setOpen(false)
		},
	})

	/**
	 * Opens the dialog on the stored settings, or closes it on the edits.
	 * @param next - Whether the dialog is opening.
	 */
	function change(next: boolean) {
		if (next && !holdsEdits(save)) {
			setTyped(typedSettings(offered, props.field.settings))
			setPairs(pairsOf(props.field.settings))
			setOpened(props.field)
		}
		setOpen(next)
	}

	return (
		<>
			<Button variant="outline" size="compact" aria-label={asking} onClick={() => change(true)}>
				{__('Settings', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={change}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{asking}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							{offered.map((setting) =>
								setting.shape === 'choice' ? (
									<SelectControl
										key={setting.name}
										label={setting.label}
										items={answers}
										value={chosenOf({ value: typed[setting.name] }, answers, answers[0])}
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
										onValueChange={(written) =>
											setTyped((was) => ({ ...was, [setting.name]: written }))
										}
									/>
								),
							)}
							{props.field.kind === 'choice' && (
								<ChoicesEditor pairs={pairs} onChange={setPairs} />
							)}
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => change(false)}>
							{__('Cancel', 'gophenberg')}
						</Button>
						<Button loading={save.isPending} onClick={() => save.mutate()}>
							{__('Save settings', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the rows editing the value and label pairs a choice field offers.
 * @param props - The pairs held and what to call with an edit.
 * @returns The editor element.
 */
function ChoicesEditor(props: { pairs: ChoicePair[]; onChange: (pairs: ChoicePair[]) => void }) {
	return (
		<Stack direction="column" gap="sm">
			<Text variant="body-sm">{__('Choices', 'gophenberg')}</Text>
			{props.pairs.map((pair, at) => (
				<Stack key={at} direction="row" gap="sm">
					<InputControl
						label={__('Value', 'gophenberg')}
						autoComplete="off"
						value={pair.value}
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
						onValueChange={(written) =>
							props.onChange(props.pairs.map((held, row) =>
								row === at ? { ...held, label: written } : held,
							))
						}
					/>
					<Button
						variant="outline"
						size="compact"
						onClick={() => props.onChange(props.pairs.filter((_, row) => row !== at))}
					>
						{__('Remove', 'gophenberg')}
					</Button>
				</Stack>
			))}
			<Button
				variant="outline"
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

/**
 * Renders the control carrying a new label for a field.
 * @param props - The group, the field, and what to report.
 * @returns The control and its dialog.
 */
function RenameField(props: Inside) {
	const [open, setOpen] = useState(false)
	const [label, setLabel] = useState(props.field.label)
	const [opened, setOpened] = useState(props.field)
	const asking = sprintf(__('Rename %(field)s', 'gophenberg'), { field: props.field.label })
	const rename = useMutation({
		mutationFn: () =>
			renameFieldInGroup(props.group, props.path ?? opened.key, label, opened.updatedAt),
		onSuccess: async () => {
			setOpen(false)
			await props.onDone(sprintf(__('%(field)s renamed.', 'gophenberg'), { field: label }))
		},
		onError: async (cause) => {
			await props.onRefused(cause)
			setOpen(false)
		},
	})
	/**
	 * Opens the dialog on the stored name, or closes it on the edits.
	 * @param next - Whether the dialog is opening.
	 */
	function change(next: boolean) {
		if (next && !holdsEdits(rename)) {
			setLabel(props.field.label)
			setOpened(props.field)
		}
		setOpen(next)
	}

	return (
		<>
			<Button variant="outline" size="compact" aria-label={asking} onClick={() => change(true)}>
				{__('Rename', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={change}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{asking}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text>{__('The name changes. Nothing stored under this field moves.', 'gophenberg')}</Text>
							<InputControl
								label={_x('Name', 'field', 'gophenberg')}
								value={label}
								onValueChange={setLabel}
							/>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Keep it', 'gophenberg')}
						</Button>
						<Button loading={rename.isPending} onClick={() => rename.mutate()}>
							{__('Rename', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the control carrying a field into another group.
 * @param props - The group, the field, the groups it may land in, and what to report.
 * @returns The control and its dialog, or nothing when there is nowhere to carry it.
 */
function CarryField(props: Inside & { elsewhere: FieldGroup[] }) {
	const landings = props.elsewhere.map((listed) => ({ label: listed.title, value: String(listed.id) }))
	const [open, setOpen] = useState(false)
	const [landing, setLanding] = useState(landings[0])
	const asking = sprintf(__('Move %(field)s elsewhere', 'gophenberg'), { field: props.field.label })
	const carry = useMutation({
		mutationFn: () => moveField(props.group, props.field.key, Number(landing?.value)),
		onSuccess: async () => {
			setOpen(false)
			await props.onDone(sprintf(__('%(field)s moved.', 'gophenberg'), { field: props.field.label }))
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	if (landing === undefined) {
		return null
	}
	return (
		<>
			<Button variant="outline" size="compact" aria-label={asking} onClick={() => setOpen(true)}>
				{__('Move', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{asking}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text>{__('The field keeps every value stored under it.', 'gophenberg')}</Text>
							<SelectControl
								label={__('Group', 'gophenberg')}
								items={landings}
								value={landing}
								onValueChange={(item) => setLanding(chosenOf(item, landings, landing))}
							/>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Keep it here', 'gophenberg')}
						</Button>
						<Button loading={carry.isPending} onClick={() => carry.mutate()}>
							{__('Move the field', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the sub fields a container declares, however deep they run.
 * @param props - The group, the types, the sub fields, the path holding them, and what to report.
 * @returns The list element, or nothing when the container declares none.
 */
function HeldFields(
	props: Reporter & { group: number; types: ContentType[]; declared: ContentField[]; at: string },
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
 * Renders the control declaring a field inside the container.
 * @param props - The group, the container, its path, the types, and what to report.
 * @returns The control and its dialog, or nothing for a field holding none.
 */
function SubFields(props: Inside & { types: ContentType[]; path: string }) {
	const [open, setOpen] = useState(false)
	const asking = sprintf(__('Add field to %(field)s', 'gophenberg'), { field: props.field.label })
	if (props.field.kind !== 'section' && props.field.kind !== 'repeater') {
		return null
	}
	return (
		<>
			<Button variant="outline" size="compact" aria-label={asking} onClick={() => setOpen(true)}>
				{__('Add field', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{asking}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<AddField
							group={props.group}
							types={props.types}
							path={props.path}
							onDone={async (said) => {
								setOpen(false)
								await props.onDone(said)
							}}
							onRefused={props.onRefused}
						/>
					</Dialog.Content>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the control removing a field and everything stored under it.
 * @param props - The group, the field, and what to report.
 * @returns The control and its dialog.
 */
function DeleteField(props: Inside) {
	const [open, setOpen] = useState(false)
	const asking = sprintf(__('Delete %(field)s', 'gophenberg'), { field: props.field.label })
	const warning = __(
		'Every value stored under this field goes with it, in every item the group reaches and in the revisions behind them.',
		'gophenberg',
	)
	const remove = useMutation({
		mutationFn: () =>
			props.path === undefined
				? deleteFieldInGroup(props.group, props.field.key)
				: deleteSubField(props.group, props.path),
		onSuccess: async () => {
			setOpen(false)
			await props.onDone(sprintf(__('%(field)s deleted.', 'gophenberg'), { field: props.field.label }))
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button variant="outline" size="compact" aria-label={asking} onClick={() => setOpen(true)}>
				{__('Delete', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{asking}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Text>{warning}</Text>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Keep it', 'gophenberg')}
						</Button>
						<Button loading={remove.isPending} onClick={() => remove.mutate()}>
							{__('Delete the field', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
