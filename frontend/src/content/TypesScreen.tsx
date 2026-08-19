// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Dialog, InputControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, _x, sprintf } from '@wordpress/i18n'
import { ErrorNotice, LoadingRows, Page, useToaster } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ReactNode } from 'react'

import { FieldsDialog } from './FieldsDialog'
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
	return cause instanceof Error ? cause.message : __('The registry could not be reached.', 'gophenberg')
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
			title={__('Content Types', 'gophenberg')}
			subtitle={__('Every kind of content this site holds.', 'gophenberg')}
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
		return <ErrorNotice>{__('The content types could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	if (props.loading) {
		return <LoadingRows label={__('Loading content types.', 'gophenberg')} />
	}
	return (
		<div
			className="godmin-table-scroll godmin-arrival"
			role="region"
			aria-label={__('Content Types', 'gophenberg')}
			tabIndex={0}
		>
			<table className="godmin-table">
				<thead>
					<tr>
						<th scope="col">{__('Type', 'gophenberg')}</th>
						<th scope="col">{__('Address', 'gophenberg')}</th>
						<th scope="col">{_x('Status', 'content type', 'gophenberg')}</th>
						<th scope="col">{__('Actions', 'gophenberg')}</th>
					</tr>
				</thead>
				<tbody>
					{props.types.map((registered) => (
						<TypeRow
							key={registered.key}
							registered={registered}
							holder={props.types.find((listed) => listed.isDefault)}
							types={props.types}
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
function TypeRow(
	props: Reporter & { registered: ContentType; holder?: ContentType; types: ContentType[] },
): ReactNode {
	const { registered } = props
	const edit = useMutation({
		mutationFn: (asked: TypeEdit) => updateType(registered.key, asked),
		onSuccess: () => props.onDone(sprintf(__('%(type)s updated.', 'gophenberg'), { type: registered.pluralLabel })),
		onError: props.onRefused,
	})
	const remove = useMutation({
		mutationFn: () => deleteType(registered.key),
		onSuccess: () => props.onDone(sprintf(__('%(type)s removed.', 'gophenberg'), { type: registered.pluralLabel })),
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
					{registered.isDefault && <Badge>{__('Default', 'gophenberg')}</Badge>}
					{registered.hierarchical && <Badge>{__('Nests', 'gophenberg')}</Badge>}
					{!registered.active && <Badge intent="draft">{__('Closed', 'gophenberg')}</Badge>}
				</Stack>
			</td>
			<td>
				<Stack direction="row" gap="xs">
					<ChangeAddress registered={registered} onMove={(word) => edit.mutate({ routeWord: word })} />
					<FieldsDialog
						registered={registered}
						types={props.types}
						onDone={props.onDone}
						onRefused={props.onRefused}
					/>
					{!registered.isDefault && (
						<HandOverRoot
							registered={registered}
							holder={props.holder}
							onHandOver={() => edit.mutate({ isDefault: true })}
						/>
					)}
					{!registered.isDefault && registered.active && (
						<Button variant="outline" onClick={() => edit.mutate({ active: false })}>
							{__('Deactivate', 'gophenberg')}
						</Button>
					)}
					{!registered.active && (
						<Button variant="outline" onClick={() => edit.mutate({ active: true })}>
							{__('Activate', 'gophenberg')}
						</Button>
					)}
					{!registered.isDefault && (
						<Button variant="outline" loading={remove.isPending} onClick={() => remove.mutate()}>
							{__('Delete', 'gophenberg')}
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
				{__('Make default', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Hand the root to %(type)s', 'gophenberg'), { type: props.registered.pluralLabel })}
						</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text>
								{sprintf(__('%(type)s will answer at the root.', 'gophenberg'), { type: props.registered.pluralLabel })}
							</Text>
							{holder !== undefined && (
								<Text>
									{sprintf(
										__(
											'%(name)s moves to /%(word)s. Every address of both types changes.',
											'gophenberg',
										),
										{ name: holder.pluralLabel, word: slugify(holder.pluralLabel) },
									)}
								</Text>
							)}
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Keep it', 'gophenberg')}
						</Button>
						<Button
							onClick={() => {
								setOpen(false)
								props.onHandOver()
							}}
						>
							{__('Hand over the root', 'gophenberg')}
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
				{__('Change address', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Change the address of %(type)s', 'gophenberg'), { type: props.registered.pluralLabel })}
						</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text>
								{__(
									'Every address of this type moves. Links kept elsewhere to the old addresses stop working.',
									'gophenberg',
								)}
							</Text>
							<InputControl
								label={__('Route word', 'gophenberg')}
								value={word}
								onValueChange={setWord}
							/>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Keep it', 'gophenberg')}
						</Button>
						<Button
							onClick={() => {
								setOpen(false)
								props.onMove(word)
							}}
						>
							{__('Move every address', 'gophenberg')}
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
			props.onDone(sprintf(__('%(type)s registered.', 'gophenberg'), { type: plural }))
		},
		onError: (cause) => {
			setOpen(false)
			props.onRefused(cause)
		},
	})
	return (
		<>
			<Button onClick={() => setOpen(true)}>{__('Add New Type', 'gophenberg')}</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{__('Register a content type', 'gophenberg')}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<InputControl
								label={__('Singular name', 'gophenberg')}
								autoComplete="off"
								value={singular}
								onValueChange={setSingular}
							/>
							<InputControl
								label={__('Plural name', 'gophenberg')}
								autoComplete="off"
								value={plural}
								onValueChange={setPlural}
							/>
							<Text variant="body-sm">
								{sprintf(
									__('This type will answer under /%(word)s.', 'gophenberg'),
									{ word: slugify(plural) || __('address', 'gophenberg') },
								)}
							</Text>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Cancel', 'gophenberg')}
						</Button>
						<Button loading={add.isPending} onClick={() => add.mutate()}>
							{__('Register', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
