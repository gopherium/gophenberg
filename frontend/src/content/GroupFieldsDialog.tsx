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
	deleteFieldInGroup,
	groupErrorMessage,
	groupsQueryKey,
	moveField,
	renameFieldInGroup,
	reorderFieldsInGroup,
	setFieldRequiredInGroup,
} from './groups'
import { chosenOf } from './select'
import { fieldKinds, kindLabel, slugifyKey } from './types'
import { typesQueryKey } from './nav'
import type { FieldGroup } from './groups'
import type { Choice } from './select'
import type { ContentField, ContentType } from './types'

/** What a field control reports back once its write settles. */
interface Reporter {
	onDone: (said: string) => Promise<void>
	onRefused: (cause: unknown) => void
}

/** The group a field control acts inside. */
interface Inside extends Reporter {
	group: number
	field: ContentField
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
	function refused(cause: unknown) {
		setNotice(groupErrorMessage(cause))
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
									<CarryField
										group={props.held.id}
										field={field}
										elsewhere={props.groups.filter((listed) => listed.id !== props.held.id)}
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
function AddField(props: Reporter & { group: number; types: ContentType[] }) {
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
	const [target, setTarget] = useState(targets[0])
	const [presence, setPresence] = useState(presences[0])
	const [holding, setHolding] = useState(holdings[0])
	const relating = kind.value === 'relation'
	const add = useMutation({
		mutationFn: () =>
			createFieldInGroup(props.group, {
				key: slugifyKey(label),
				label,
				kind: kind.value,
				relatesTo: relating ? target.value : undefined,
				many: relating && holding.value === 'many',
				required: presence.value === 'required',
			}),
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
			{relating && (
				<SelectControl
					label={__('Points at', 'gophenberg')}
					items={targets}
					value={target}
					onValueChange={(item) => setTarget(chosenOf(item, targets, target))}
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
			<Button loading={add.isPending} onClick={() => add.mutate()}>
				{__('Add field', 'gophenberg')}
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
		mutationFn: () => setFieldRequiredInGroup(props.group, props.field.key, !props.field.required),
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
	const asking = sprintf(__('Rename %(field)s', 'gophenberg'), { field: props.field.label })
	const rename = useMutation({
		mutationFn: () => renameFieldInGroup(props.group, props.field.key, label),
		onSuccess: async () => {
			setOpen(false)
			await props.onDone(sprintf(__('%(field)s renamed.', 'gophenberg'), { field: label }))
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button variant="outline" size="compact" aria-label={asking} onClick={() => setOpen(true)}>
				{__('Rename', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
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
		mutationFn: () => deleteFieldInGroup(props.group, props.field.key),
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
