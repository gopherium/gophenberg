// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Dialog, InputControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice, LoadingRows, Page, useToaster } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ReactNode } from 'react'

import { typesQueryKey } from './nav'
import { createType, deleteType, listTypes, updateType } from './types'
import type { ContentType, TypeEdit } from './types'

/**
 * Returns the address a type answers under, as the screen shows it.
 * @param registered - The type the address belongs to.
 * @returns The address, or the root marker.
 */
export function addressOf(registered: ContentType): string {
	return registered.routeWord === '' ? '/' : `/${registered.routeWord}`
}

/**
 * Returns the key and route word a plural label suggests.
 * @param plural - The plural label the operator typed.
 * @returns The slug the label reduces to.
 */
export function slugify(plural: string): string {
	return plural
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
}

/**
 * Returns the sentence naming why a registry write was turned away.
 * @param cause - What the write failed with.
 * @returns The sentence to show.
 */
export function refusalMessage(cause: unknown): string {
	return cause instanceof Error ? cause.message : 'The registry could not be reached.'
}

/**
 * Renders the content type registry screen.
 * @returns The registry screen element.
 */
export function TypesScreen() {
	const client = useQueryClient()
	const toaster = useToaster()
	const [refusal, setRefusal] = useState('')
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })

	/**
	 * Reports what a registry write did, and refreshes what the admin holds.
	 * @param said - The sentence naming what was done.
	 */
	async function done(said: string) {
		setRefusal('')
		toaster.show(said)
		await client.invalidateQueries({ queryKey: typesQueryKey })
	}

	/**
	 * Reports why the registry turned a write away.
	 * @param cause - What the write failed with.
	 */
	function refused(cause: unknown) {
		setRefusal(refusalMessage(cause))
	}

	return (
		<Page
			title="Content Types"
			subtitle="Every kind of content this site holds."
			actions={<AddType onDone={done} onRefused={refused} />}
		>
			<Stack direction="column" gap="md">
				{refusal !== '' && <ErrorNotice>{refusal}</ErrorNotice>}
				<TypesBody
					types={types.data ?? []}
					loading={types.isPending}
					failed={types.isError}
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
 * Renders the registry in whichever state it is in.
 * @param props - The types, whether the read is loading or failed, and the reporter.
 * @returns The registry body element.
 */
function TypesBody(props: Reporter & { types: ContentType[]; loading: boolean; failed: boolean }) {
	if (props.failed) {
		return <ErrorNotice>The content types could not be loaded.</ErrorNotice>
	}
	if (props.loading) {
		return <LoadingRows label="Loading content types." />
	}
	return (
		<div
			className="godmin-table-scroll godmin-arrival"
			role="region"
			aria-label="Content Types"
			tabIndex={0}
		>
			<table className="godmin-table">
				<thead>
					<tr>
						<th scope="col">Type</th>
						<th scope="col">Address</th>
						<th scope="col">Status</th>
						<th scope="col">Actions</th>
					</tr>
				</thead>
				<tbody>
					{props.types.map((registered) => (
						<TypeRow
							key={registered.key}
							registered={registered}
							holder={props.types.find((listed) => listed.isDefault)}
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
 * Renders one registered type and what may be done to it.
 * @param props - The type and the reporter.
 * @returns The row element.
 */
function TypeRow(props: Reporter & { registered: ContentType; holder?: ContentType }): ReactNode {
	const { registered } = props
	const edit = useMutation({
		mutationFn: (asked: TypeEdit) => updateType(registered.key, asked),
		onSuccess: () => props.onDone(`${registered.pluralLabel} updated.`),
		onError: props.onRefused,
	})
	const remove = useMutation({
		mutationFn: () => deleteType(registered.key),
		onSuccess: () => props.onDone(`${registered.pluralLabel} removed.`),
		onError: props.onRefused,
	})
	return (
		<tr>
			<td>
				<Stack direction="column" gap="xs">
					<Text>{registered.pluralLabel}</Text>
					<Text variant="body-sm">{registered.key}</Text>
				</Stack>
			</td>
			<td>{addressOf(registered)}</td>
			<td>
				<Stack direction="row" gap="xs">
					{registered.isDefault && <Badge>Default</Badge>}
					{registered.hierarchical && <Badge>Nests</Badge>}
					{!registered.active && <Badge intent="draft">Closed</Badge>}
				</Stack>
			</td>
			<td>
				<Stack direction="row" gap="xs">
					<ChangeAddress registered={registered} onMove={(word) => edit.mutate({ routeWord: word })} />
					{!registered.isDefault && (
						<HandOverRoot
							registered={registered}
							holder={props.holder}
							onHandOver={() => edit.mutate({ isDefault: true })}
						/>
					)}
					{!registered.isDefault && registered.active && (
						<Button variant="outline" onClick={() => edit.mutate({ active: false })}>
							Deactivate
						</Button>
					)}
					{!registered.active && (
						<Button variant="outline" onClick={() => edit.mutate({ active: true })}>
							Activate
						</Button>
					)}
					{!registered.isDefault && (
						<Button variant="outline" loading={remove.isPending} onClick={() => remove.mutate()}>
							Delete
						</Button>
					)}
				</Stack>
			</td>
		</tr>
	)
}

/**
 * Renders the confirmation handing the root from one type to another.
 * @param props - The type taking the root, the type holding it, and what to do.
 * @returns The control and its dialog.
 */
function HandOverRoot(props: {
	registered: ContentType
	holder?: ContentType
	onHandOver: () => void
}) {
	const [open, setOpen] = useState(false)
	const holder = props.holder
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				Make default
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>Hand the root to {props.registered.pluralLabel}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text>{props.registered.pluralLabel} will answer at the root.</Text>
							{holder !== undefined && (
								<Text>
									{holder.pluralLabel} moves to /{slugify(holder.pluralLabel)}. Every address
									of both types changes.
								</Text>
							)}
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							Keep it
						</Button>
						<Button
							onClick={() => {
								setOpen(false)
								props.onHandOver()
							}}
						>
							Hand over the root
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the confirmation a route word change passes through.
 * @param props - The type and what to do with the new word.
 * @returns The control and its dialog.
 */
function ChangeAddress(props: { registered: ContentType; onMove: (word: string) => void }) {
	const [open, setOpen] = useState(false)
	const [word, setWord] = useState(props.registered.routeWord)
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				Change address
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>Change the address of {props.registered.pluralLabel}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text>
								Every address of this type moves. Links kept elsewhere to the old addresses
								stop working.
							</Text>
							<InputControl label="Route word" value={word} onValueChange={setWord} />
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							Keep it
						</Button>
						<Button
							onClick={() => {
								setOpen(false)
								props.onMove(word)
							}}
						>
							Move every address
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the control registering a new content type.
 * @param props - The reporter.
 * @returns The control and its dialog.
 */
function AddType(props: Reporter) {
	const [open, setOpen] = useState(false)
	const [singular, setSingular] = useState('')
	const [plural, setPlural] = useState('')
	const add = useMutation({
		mutationFn: () =>
			createType({
				key: slugify(singular),
				singularLabel: singular,
				pluralLabel: plural,
				routeWord: slugify(plural),
			}),
		onSuccess: () => {
			setOpen(false)
			setSingular('')
			setPlural('')
			props.onDone(`${plural} registered.`)
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button onClick={() => setOpen(true)}>Add New Type</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>Register a content type</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<InputControl
								label="Singular name"
								autoComplete="off"
								value={singular}
								onValueChange={setSingular}
							/>
							<InputControl
								label="Plural name"
								autoComplete="off"
								value={plural}
								onValueChange={setPlural}
							/>
							<Text variant="body-sm">
								This type will answer under /{slugify(plural) || 'address'}.
							</Text>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							Cancel
						</Button>
						<Button loading={add.isPending} onClick={() => add.mutate()}>
							Register
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
