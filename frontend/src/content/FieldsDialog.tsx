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
import { __, _x, sprintf } from '@wordpress/i18n'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { typesQueryKey } from './nav'
import {
	createField,
	deleteField,
	fieldKinds,
	kindLabel,
	listFields,
	renameField,
	reorderFields,
	setFieldRequired,
	slugifyKey,
} from './types'
import { chosenOf } from './select'
import type { Choice } from './select'
import type { ContentField, ContentType } from './types'

/**
 * Returns the key for a field query over one type.
 * @param typeKey - The type the fields belong to.
 * @returns The query key.
 */
export function fieldsQueryKey(typeKey: string): string[] {
	return ['content-fields', typeKey]
}

/**
 * Renders the view declaring and removing the fields of one content type.
 * @param props - The type whose fields are managed, the other types, and the reporter.
 * @returns The control and its dialog.
 */
export function FieldsDialog(props: {
	registered: ContentType
	types: ContentType[]
	onDone: (said: string) => void
	onRefused: (cause: unknown) => void
}) {
	const [open, setOpen] = useState(false)
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				{__('Fields', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Fields of %(type)s', 'gophenberg'), { type: props.registered.pluralLabel })}
						</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<FieldsBody {...props} />
					</Dialog.Content>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the declared fields and the control adding another.
 * @param props - The type whose fields are managed, the other types, and the reporter.
 * @returns The body element.
 */
function FieldsBody(props: {
	registered: ContentType
	types: ContentType[]
	onDone: (said: string) => void
	onRefused: (cause: unknown) => void
}) {
	const client = useQueryClient()
	const typeKey = props.registered.key
	const held = useQuery({
		queryKey: fieldsQueryKey(typeKey),
		queryFn: () => listFields(typeKey),
	})
	/**
	 * Reports what a field write did and refreshes what the screen holds.
	 * @param said - The sentence naming what was done.
	 */
	async function done(said: string) {
		props.onDone(said)
		await Promise.all([
			client.invalidateQueries({ queryKey: fieldsQueryKey(typeKey) }),
			client.invalidateQueries({ queryKey: typesQueryKey }),
		])
	}
	const declared = held.data ?? []
	const reorder = useMutation({
		mutationFn: (keys: string[]) => reorderFields(typeKey, keys),
		onSuccess: () => done(__('Order stored.', 'gophenberg')),
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
				<Text>{__('This type declares no fields yet.', 'gophenberg')}</Text>
			) : (
				<ul className="gophenberg-fields__list">
					{declared.map((field, index) => (
						<li key={field.key}>
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
										typeKey={typeKey}
										field={field}
										onDone={done}
										onRefused={props.onRefused}
									/>
									<RenameField
										typeKey={typeKey}
										field={field}
										onDone={done}
										onRefused={props.onRefused}
									/>
									<DeleteField
										typeKey={typeKey}
										field={field}
										onDone={done}
										onRefused={props.onRefused}
									/>
								</Stack>
							</Stack>
						</li>
					))}
				</ul>
			)}
			<AddField
				registered={props.registered}
				types={props.types}
				onDone={done}
				onRefused={props.onRefused}
			/>
		</Stack>
	)
}

/**
 * Renders the control declaring a new field.
 * @param props - The type declaring the field, the other types, and the reporter.
 * @returns The control element.
 */
function AddField(props: {
	registered: ContentType
	types: ContentType[]
	onDone: (said: string) => Promise<void>
	onRefused: (cause: unknown) => void
}) {
	const [kinds] = useState(fieldKinds)
	const targets = props.types.map((listed) => ({
		label: listed.pluralLabel,
		value: listed.key,
	}))
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
			createField(props.registered.key, {
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
 * @param props - The type, the field, and the reporter.
 * @returns The control element.
 */
function RequireField(props: {
	typeKey: string
	field: ContentField
	onDone: (said: string) => Promise<void>
	onRefused: (cause: unknown) => void
}) {
	const flip = useMutation({
		mutationFn: () => setFieldRequired(props.typeKey, props.field.key, !props.field.required),
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
 * @param props - The type, the field, and the reporter.
 * @returns The control and its dialog.
 */
function RenameField(props: {
	typeKey: string
	field: ContentField
	onDone: (said: string) => Promise<void>
	onRefused: (cause: unknown) => void
}) {
	const [open, setOpen] = useState(false)
	const [label, setLabel] = useState(props.field.label)
	const rename = useMutation({
		mutationFn: () => renameField(props.typeKey, props.field.key, label),
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
			<Button
				variant="outline"
				size="compact"
				aria-label={sprintf(__('Rename %(field)s', 'gophenberg'), { field: props.field.label })}
				onClick={() => setOpen(true)}
			>
				{__('Rename', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{sprintf(__('Rename %(field)s', 'gophenberg'), { field: props.field.label })}</Dialog.Title>
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
 * Renders the control removing a field and everything stored under it.
 * @param props - The type, the field, and the reporter.
 * @returns The control and its dialog.
 */
function DeleteField(props: {
	typeKey: string
	field: ContentField
	onDone: (said: string) => Promise<void>
	onRefused: (cause: unknown) => void
}) {
	const [open, setOpen] = useState(false)
	const warning = __(
		'Every value stored under this field goes with it, in every item of this type and in the revisions behind them.',
		'gophenberg',
	)
	const remove = useMutation({
		mutationFn: () => deleteField(props.typeKey, props.field.key),
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
			<Button
				variant="outline"
				size="compact"
				aria-label={sprintf(__('Delete %(field)s', 'gophenberg'), { field: props.field.label })}
				onClick={() => setOpen(true)}
			>
				{__('Delete', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{sprintf(__('Delete %(field)s', 'gophenberg'), { field: props.field.label })}</Dialog.Title>
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
