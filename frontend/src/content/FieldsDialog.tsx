// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, InputControl, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { typesQueryKey } from './nav'
import { createField, deleteField, listFields, renameField, slugifyKey } from './types'
import type { ContentField, ContentType } from './types'

/** The kinds a field may be declared as, in the order the admin offers them. */
const KINDS = [
	{ label: 'Text', value: 'text' },
	{ label: 'Number', value: 'number' },
	{ label: 'Yes or no', value: 'boolean' },
	{ label: 'Date', value: 'date' },
	{ label: 'Media', value: 'media' },
	{ label: 'Relation', value: 'relation' },
]

/**
 * Returns the key for a field query over one type.
 * @param typeKey - The type the fields belong to.
 * @returns The query key.
 */
export function fieldsQueryKey(typeKey: string): string[] {
	return ['content-fields', typeKey]
}

/** One choice a select offers. */
interface Choice {
	label: string
	value: string
}

/**
 * Returns the choice a select change asks for.
 * @param item - The item the select reported, or nothing.
 * @param offered - The choices the select holds.
 * @param current - The choice held.
 * @returns The choice to hold.
 */
export function chosenOf(
	item: { value: string | null } | null,
	offered: Choice[],
	current: Choice,
): Choice {
	if (item === null || item.value === null) {
		return current
	}
	return offered.find((choice) => choice.value === item.value) ?? current
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
				Fields
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>Fields of {props.registered.pluralLabel}</Dialog.Title>
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
	return (
		<Stack direction="column" gap="md">
			{declared.length === 0 ? (
				<Text>This type declares no fields yet.</Text>
			) : (
				<ul className="godmin-plain-list">
					{declared.map((field) => (
						<li key={field.key}>
							<Stack direction="row" gap="xs">
								<Text>{field.label}</Text>
								<Text variant="body-sm">{field.kind}</Text>
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
	const targets = props.types.map((listed) => ({
		label: listed.pluralLabel,
		value: listed.key,
	}))
	const [label, setLabel] = useState('')
	const [kind, setKind] = useState(KINDS[0])
	const [target, setTarget] = useState(targets[0])
	const relating = kind.value === 'relation'
	const add = useMutation({
		mutationFn: () =>
			createField(props.registered.key, {
				key: slugifyKey(label),
				label,
				kind: kind.value,
				relatesTo: relating ? target.value : undefined,
				many: relating,
			}),
		onSuccess: async () => {
			setLabel('')
			await props.onDone(`${label} declared.`)
		},
		onError: props.onRefused,
	})
	return (
		<Stack direction="column" gap="sm">
			<InputControl label="Name" autoComplete="off" value={label} onValueChange={setLabel} />
			<SelectControl
				label="Kind"
				items={KINDS}
				value={kind}
				onValueChange={(item) => setKind(chosenOf(item, KINDS, kind))}
			/>
			{relating && (
				<SelectControl
					label="Points at"
					items={targets}
					value={target}
					onValueChange={(item) => setTarget(chosenOf(item, targets, target))}
				/>
			)}
			<Button loading={add.isPending} onClick={() => add.mutate()}>
				Add field
			</Button>
		</Stack>
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
			await props.onDone(`${label} renamed.`)
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				Rename {props.field.label}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>Rename {props.field.label}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text>The name changes. Nothing stored under this field moves.</Text>
							<InputControl label="Name" value={label} onValueChange={setLabel} />
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							Keep it
						</Button>
						<Button loading={rename.isPending} onClick={() => rename.mutate()}>
							Rename
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
	const remove = useMutation({
		mutationFn: () => deleteField(props.typeKey, props.field.key),
		onSuccess: async () => {
			setOpen(false)
			await props.onDone(`${props.field.label} deleted.`)
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				Delete {props.field.label}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>Delete {props.field.label}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Text>
							Every value stored under this field goes with it, in every item of this type
							and in the revisions behind them.
						</Text>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							Keep it
						</Button>
						<Button loading={remove.isPending} onClick={() => remove.mutate()}>
							Delete the field
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
