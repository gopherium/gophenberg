// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Dialog, InputControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, _x, sprintf } from '@wordpress/i18n'
import { ErrorNotice, LoadingRows, Page, useToaster } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
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
export function errorMessage(cause: unknown): string {
	return cause instanceof Error ? cause.message : __('The registry could not be reached.', 'gophenberg')
}

/**
 * Renders the content type registry screen.
 * @returns The registry screen element.
 */
export function TypesScreen() {
	const client = useQueryClient()
	const toaster = useToaster()
	const [notice, setNotice] = useState('')
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })

	/**
	 * Reports what a registry write did, and refreshes what the admin holds.
	 * @param said - The sentence naming what was done.
	 */
	async function done(said: string) {
		setNotice('')
		toaster.show(said)
		await client.invalidateQueries({ queryKey: typesQueryKey })
	}

	/**
	 * Reports why the registry turned a write away.
	 * @param cause - What the write failed with.
	 */
	function refused(cause: unknown) {
		setNotice(errorMessage(cause))
	}

	return (
		<Page
			title={__('Content Types', 'gophenberg')}
			subtitle={__('Every kind of content this site holds.', 'gophenberg')}
			actions={<AddType onDone={done} onRefused={refused} />}
		>
			<Stack direction="column" gap="md">
				{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
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
	onDone: (said: string) => void | Promise<void>
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
 * Returns the plugin that declared the type, empty when the site made it.
 * @param registered - The type.
 * @returns The plugin's id, or an empty string.
 */
function originOf(registered: ContentType): string {
	return registered.origin ?? ''
}

/**
 * Renders the badges naming what stands out about a type.
 * @param props - The type.
 * @returns The badges element.
 */
function TypeBadges(props: { registered: ContentType }) {
	const { registered } = props
	const origin = originOf(registered)
	return (
		<Stack direction="row" gap="xs">
			{registered.isDefault && <Badge>{__('Default', 'gophenberg')}</Badge>}
			{registered.hierarchical && <Badge>{__('Nests', 'gophenberg')}</Badge>}
			{!registered.active && <Badge intent="draft">{__('Closed', 'gophenberg')}</Badge>}
			{origin !== '' && <Badge>{sprintf(__('From %(plugin)s', 'gophenberg'), { plugin: origin })}</Badge>}
		</Stack>
	)
}

/**
 * Renders the controls a type offers, holding back the ones its owner keeps.
 * @param props - The type, the root holder, the reporter, and what each control does.
 * @returns The controls element.
 */
function TypeActions(
	props: Reporter & {
		registered: ContentType
		holder?: ContentType
		removing: boolean
		onEdit: (asked: TypeEdit) => void
		onRemove: () => void
	},
) {
	const { registered } = props
	const shapeable = originOf(registered) === ''
	const demotable = shapeable && !registered.isDefault
	return (
		<Stack direction="row" gap="xs" wrap="wrap" className="gophenberg-types__actions">
			{shapeable && (
				<>
					<DescribeType registered={registered} onDone={props.onDone} onRefused={props.onRefused} />
					<ChangeAddress registered={registered} onDone={props.onDone} onRefused={props.onRefused} />
					<NestingControl registered={registered} onEdit={props.onEdit} />
				</>
			)}
			{demotable && (
				<HandOverRoot
					registered={registered}
					holder={props.holder}
					onHandOver={() => props.onEdit({ isDefault: true })}
				/>
			)}
			{!registered.isDefault && registered.active && (
				<Button variant="outline" onClick={() => props.onEdit({ active: false })}>
					{__('Deactivate', 'gophenberg')}
				</Button>
			)}
			{!registered.active && (
				<Button variant="outline" onClick={() => props.onEdit({ active: true })}>
					{__('Activate', 'gophenberg')}
				</Button>
			)}
			{demotable && (
				<Button variant="outline" loading={props.removing} onClick={props.onRemove}>
					{__('Delete', 'gophenberg')}
				</Button>
			)}
		</Stack>
	)
}

/**
 * Renders the control letting a type's items nest, or stopping them.
 * @param props - The type and what to do with the flag.
 * @returns The control element.
 */
function NestingControl(props: { registered: ContentType; onEdit: (asked: TypeEdit) => void }) {
	const nests = props.registered.hierarchical
	return (
		<Button variant="outline" onClick={() => props.onEdit({ hierarchical: !nests })}>
			{nests ? __('Stop nesting', 'gophenberg') : __('Let items nest', 'gophenberg')}
		</Button>
	)
}

/**
 * Returns whether a dialog is open and counts its openings, so a write settles only the opening it was sent from.
 * @returns Whether the dialog is open, what opens or closes it, what names the opening now, and what settles one.
 */
function useOpenings() {
	const [open, setOpen] = useState(false)
	const openings = useRef(0)

	/**
	 * Opens or closes the dialog, counting each opening.
	 * @param next - Whether the dialog opens.
	 */
	function change(next: boolean) {
		if (next) {
			openings.current += 1
		}
		setOpen(next)
	}

	/**
	 * Returns the opening the dialog is in now, for a write to carry until it settles.
	 * @returns The count of openings so far.
	 */
	function current() {
		return openings.current
	}

	/**
	 * Closes the dialog for a settled write, unless a newer opening began since the write was sent.
	 * @param opening - The opening the write was sent from.
	 * @returns Whether that opening is still the current one.
	 */
	function settle(opening: number | undefined) {
		const latest = opening === openings.current
		if (latest) {
			setOpen(false)
		}
		return latest
	}

	return { open, change, current, settle }
}

/**
 * Returns the write editing a type, pending until the registry turns it away or takes it and the list refreshes.
 * @param props - The type and the reporter.
 * @param dialog - The dialog the write is sent from, closed once the write settles in the opening it was sent from.
 * @returns The mutation sending the edit.
 */
function useTypeEdit(props: Reporter & { registered: ContentType }, dialog?: ReturnType<typeof useOpenings>) {
	return useMutation({
		mutationFn: (asked: TypeEdit) => updateType(props.registered.key, asked),
		onMutate: dialog?.current,
		onSuccess: async (_stored, _asked, opening) => {
			await props.onDone(sprintf(__('%(type)s updated.', 'gophenberg'), { type: props.registered.pluralLabel }))
			dialog?.settle(opening)
		},
		onError: (cause, _asked, opening) => {
			dialog?.settle(opening)
			props.onRefused(cause)
		},
	})
}

/**
 * Returns a dialog editing one stored value of a type, keeping a draft the registry turned away for the next open.
 * @param props - The type and the reporter.
 * @param stored - The value the type holds now.
 * @returns Whether the dialog is open, what opens or closes it, the draft, and the write sending it.
 */
function useTypeDraft(props: Reporter & { registered: ContentType }, stored: string) {
	const dialog = useOpenings()
	const [draft, setDraft] = useState(stored)
	const save = useTypeEdit(props, dialog)

	/**
	 * Opens or closes the dialog at any time, keeping a turned away draft until a close.
	 * @param next - Whether the dialog opens.
	 */
	function change(next: boolean) {
		if (!next) {
			save.reset()
		} else if (!save.isError) {
			setDraft(stored)
		}
		dialog.change(next)
	}

	return { open: dialog.open, change, draft, setDraft, save }
}

/**
 * Renders one registered type and what may be done to it.
 * @param props - The type and the reporter.
 * @returns The row element.
 */
function TypeRow(
	props: Reporter & { registered: ContentType; holder?: ContentType },
): ReactNode {
	const { registered } = props
	const edit = useTypeEdit(props)
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
				<TypeBadges registered={registered} />
			</td>
			<td>
				<TypeActions
					registered={registered}
					holder={props.holder}
					onDone={props.onDone}
					onRefused={props.onRefused}
					removing={remove.isPending}
					onEdit={edit.mutate}
					onRemove={remove.mutate}
				/>
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
 * Renders the dialog changing what a type's description says.
 * @param props - The type and the reporter.
 * @returns The control and its dialog.
 */
function DescribeType(props: Reporter & { registered: ContentType }) {
	const text = useTypeDraft(props, props.registered.description)
	return (
		<>
			<Button variant="outline" onClick={() => text.change(true)}>
				{__('Describe', 'gophenberg')}
			</Button>
			<Dialog.Root open={text.open} onOpenChange={text.change}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Describe %(type)s', 'gophenberg'), { type: props.registered.pluralLabel })}
						</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<InputControl
							label={__('Description', 'gophenberg')}
							description={__('A descriptive summary of the content type.', 'gophenberg')}
							autoComplete="off"
							value={text.draft}
							onValueChange={text.setDraft}
						/>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => text.change(false)}>
							{__('Cancel', 'gophenberg')}
						</Button>
						<Button loading={text.save.isPending} onClick={() => text.save.mutate({ description: text.draft })}>
							{__('Save', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/**
 * Renders the confirmation a route word change passes through.
 * @param props - The type and the reporter.
 * @returns The control and its dialog.
 */
function ChangeAddress(props: Reporter & { registered: ContentType }) {
	const word = useTypeDraft(props, props.registered.routeWord)
	return (
		<>
			<Button variant="outline" onClick={() => word.change(true)}>
				{__('Change address', 'gophenberg')}
			</Button>
			<Dialog.Root open={word.open} onOpenChange={word.change}>
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
								autoComplete="off"
								value={word.draft}
								onValueChange={word.setDraft}
							/>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => word.change(false)}>
							{__('Keep it', 'gophenberg')}
						</Button>
						<Button loading={word.save.isPending} onClick={() => word.save.mutate({ routeWord: word.draft })}>
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
	const dialog = useOpenings()
	const [singular, setSingular] = useState('')
	const [plural, setPlural] = useState('')
	const [description, setDescription] = useState('')
	const add = useMutation({
		mutationFn: createType,
		onMutate: dialog.current,
		onSuccess: (_stored, sent, opening) => {
			if (dialog.settle(opening)) {
				setSingular('')
				setPlural('')
				setDescription('')
			}
			props.onDone(sprintf(__('%(type)s registered.', 'gophenberg'), { type: sent.pluralLabel }))
		},
		onError: (cause, _sent, opening) => {
			dialog.settle(opening)
			props.onRefused(cause)
		},
	})

	/**
	 * Opens or closes the dialog at any time, so no opening inherits a type still being registered.
	 * @param next - Whether the dialog opens.
	 */
	function change(next: boolean) {
		add.reset()
		dialog.change(next)
	}

	return (
		<>
			<Button onClick={() => change(true)}>{__('Add New Type', 'gophenberg')}</Button>
			<Dialog.Root open={dialog.open} onOpenChange={change}>
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
								description={sprintf(
									__('This type will answer under /%(word)s.', 'gophenberg'),
									{ word: slugify(plural) || __('address', 'gophenberg') },
								)}
								autoComplete="off"
								value={plural}
								onValueChange={setPlural}
							/>
							<InputControl
								label={__('Description', 'gophenberg')}
								description={__('A descriptive summary of the content type.', 'gophenberg')}
								autoComplete="off"
								value={description}
								onValueChange={setDescription}
							/>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => change(false)}>
							{__('Cancel', 'gophenberg')}
						</Button>
						<Button
							loading={add.isPending}
							onClick={() =>
								add.mutate({
									key: slugify(singular),
									singularLabel: singular,
									pluralLabel: plural,
									description,
									routeWord: slugify(plural),
								})
							}
						>
							{__('Register', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
