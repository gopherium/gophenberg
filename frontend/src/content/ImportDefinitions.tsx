// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, CheckboxControl, Dialog, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { applyDefinitions, planDefinitions } from './definitions'
import { groupsQueryKey } from './groups'
import { typesQueryKey } from './nav'
import type { Confirmed, DefinitionsPlan, ImportOutcome, PlanChange, PlanWarning } from './definitions'

/**
 * Returns the definitions file a file field holds.
 * @param files - What the file field holds.
 * @returns The chosen file, or null when the field holds none.
 */
export function chosenDefinitions(files: FileList | null): File | null {
	return files?.[0] ?? null
}

/**
 * Returns the sentence naming what an import would do to one definition.
 * @param change - The change the plan holds.
 * @returns The sentence to show.
 */
function changeSentence(change: PlanChange): string {
	if (change.action === 'create') {
		return sprintf(__('Add %(name)s', 'gophenberg'), { name: change.label })
	}
	if (change.action === 'delete') {
		return sprintf(__('Remove %(name)s', 'gophenberg'), { name: change.label })
	}
	return sprintf(__('Change %(name)s', 'gophenberg'), { name: change.label })
}

/**
 * Returns the word a definition is listed under.
 * @param subject - The definition the change stands over.
 * @returns The word to show.
 */
function subjectWord(subject: string): string {
	if (subject === 'type') {
		return __('Content type', 'gophenberg')
	}
	if (subject === 'group') {
		return __('Field group', 'gophenberg')
	}
	return __('Field', 'gophenberg')
}

/**
 * Returns the sentence naming why a definition cannot simply be carried over.
 * @param reason - The reason the plan named, if it named one.
 * @returns The sentence to show, or nothing when the plan named no reason.
 */
function reasonSentence(reason: string | undefined): string {
	if (reason === 'kind_changed') {
		return __('Its kind changed, so its stored values go with it.', 'gophenberg')
	}
	if (reason === 'shape_changed') {
		return __('What it points at changed, so its stored values go with it.', 'gophenberg')
	}
	if (reason === 'moved') {
		return __('It moves to another group, so its stored values do not follow.', 'gophenberg')
	}
	return ''
}

/**
 * Returns the sentence naming what an import would change beyond the definitions.
 * @param warning - The warning the plan holds.
 * @returns The sentence to show.
 */
function warningSentence(warning: PlanWarning): string {
	if (warning.code === 'root_moved') {
		return sprintf(
			__('%(type)s becomes the type the site opens on, and the old one takes an address.', 'gophenberg'),
			{ type: warning.key },
		)
	}
	return sprintf(__('%(type)s answers on a new address, and stored links move with it.', 'gophenberg'), {
		type: warning.key,
	})
}

/**
 * Returns the change named the way the server names one it may take away.
 * @param change - The change to name.
 * @returns The confirmation naming it.
 */
export function confirmationFor(change: PlanChange): Confirmed {
	return { subject: change.subject, key: change.key, group: change.group }
}

/**
 * Returns whether the two confirmations name the same change.
 * @param one - The confirmation to compare.
 * @param other - The confirmation to compare it with.
 * @returns Whether they name the same change.
 */
function sameConfirmation(one: Confirmed, other: Confirmed): boolean {
	return one.subject === other.subject && one.key === other.key && one.group === other.group
}

/**
 * Renders one line of the plan.
 * @param props - The change to show.
 * @returns The line element.
 */
function PlanRow(props: { change: PlanChange }) {
	const said = reasonSentence(props.change.reason)
	return (
		<Stack direction="column" gap="xs">
			<Stack direction="row" gap="sm" align="center">
				<Badge>{subjectWord(props.change.subject)}</Badge>
				<Text>{changeSentence(props.change)}</Text>
			</Stack>
			{said !== '' && <Text variant="body-sm">{said}</Text>}
		</Stack>
	)
}

/**
 * Renders one line of the plan the admin has to agree to before it happens.
 * @param props - The change, whether it is confirmed, and the reporter of a change of mind.
 * @returns The line element.
 */
function LosingRow(props: { change: PlanChange; confirmed: boolean; onChange: (agreed: boolean) => void }) {
	const said = reasonSentence(props.change.reason)
	return (
		<Stack direction="column" gap="xs">
			<Stack direction="row" gap="sm" align="center">
				<Badge>{subjectWord(props.change.subject)}</Badge>
				<CheckboxControl
					__nextHasNoMarginBottom
					label={changeSentence(props.change)}
					checked={props.confirmed}
					onChange={props.onChange}
				/>
			</Stack>
			{said !== '' && <Text variant="body-sm">{said}</Text>}
		</Stack>
	)
}

/**
 * Renders what the server said a definitions file would change.
 * @param props - The plan, the confirmations so far, and the reporter of a change of mind.
 * @returns The plan element.
 */
function PlanView(props: {
	plan: DefinitionsPlan
	confirmed: Confirmed[]
	onConfirm: (change: PlanChange, agreed: boolean) => void
}) {
	if (props.plan.changes.length === 0 && props.plan.warnings.length === 0) {
		return <Text>{__('This file matches the site, so there is nothing to change.', 'gophenberg')}</Text>
	}
	return (
		<Stack direction="column" gap="md">
			{props.plan.warnings.map((held) => (
				<Text key={`${held.code}:${held.key}`} variant="body-sm">
					{warningSentence(held)}
				</Text>
			))}
			{props.plan.changes.map((held, at) => {
				const at_ = `${held.subject}:${held.key}:${held.action}:${at}`
				if (held.action !== 'delete') {
					return <PlanRow key={at_} change={held} />
				}
				const naming = confirmationFor(held)
				return (
					<LosingRow
						key={at_}
						change={held}
						confirmed={props.confirmed.some((one) => sameConfirmation(one, naming))}
						onChange={(agreed) => props.onConfirm(held, agreed)}
					/>
				)
			})}
		</Stack>
	)
}

/**
 * Renders what an import did and what it left standing.
 * @param props - The outcome the server answered.
 * @returns The outcome element.
 */
function OutcomeView(props: { outcome: ImportOutcome }) {
	return (
		<Stack direction="column" gap="md">
			<Text>{__('Done. The site now holds what the file describes.', 'gophenberg')}</Text>
			{props.outcome.skipped.length > 0 && (
				<Text variant="body-sm">
					{__('These were left alone, because nobody confirmed losing them.', 'gophenberg')}
				</Text>
			)}
			{props.outcome.skipped.map((held, at) => (
				<PlanRow key={`${held.subject}:${held.key}:${at}`} change={held} />
			))}
		</Stack>
	)
}

/**
 * Renders the control reading a definitions file and showing what it would change.
 * @returns The control and its dialog.
 */
export function ImportDefinitions() {
	const [open, setOpen] = useState(false)
	const [notice, setNotice] = useState('')
	const [plan, setPlan] = useState<DefinitionsPlan | null>(null)
	const [file, setFile] = useState('')
	const [confirmed, setConfirmed] = useState<Confirmed[]>([])
	const [outcome, setOutcome] = useState<ImportOutcome | null>(null)
	const client = useQueryClient()
	const read = useMutation({
		mutationFn: async (chosen: File) => {
			const text = await chosen.text()
			return { text, plan: await planDefinitions(text) }
		},
		onSuccess: (answered) => {
			setNotice('')
			setFile(answered.text)
			setPlan(answered.plan)
		},
		onError: (cause) => {
			setPlan(null)
			setNotice(cause.message)
		},
	})
	const perform = useMutation({
		mutationFn: () => applyDefinitions(file, confirmed),
		onSuccess: async (answered) => {
			setNotice('')
			setPlan(null)
			setOutcome(answered)
			await client.invalidateQueries({ queryKey: groupsQueryKey })
			await client.invalidateQueries({ queryKey: typesQueryKey })
		},
		onError: (cause) => setNotice(cause.message),
	})

	/**
	 * Records that the admin agreed to lose a change, or changed their mind.
	 * @param change - The change they answered for.
	 * @param agreed - Whether they agreed to lose it.
	 */
	function confirming(change: PlanChange, agreed: boolean) {
		const naming = confirmationFor(change)
		setConfirmed((held) =>
			agreed ? [...held, naming] : held.filter((one) => !sameConfirmation(one, naming)),
		)
	}
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				{__('Import definitions', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{__('Import definitions', 'gophenberg')}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<label htmlFor="definitions-file">{__('Definitions file', 'gophenberg')}</label>
							<input
								id="definitions-file"
								type="file"
								accept=".json,application/json"
								onChange={(event) => {
									const chosen = chosenDefinitions(event.target.files)
									event.target.value = ''
									if (chosen !== null) {
										setPlan(null)
										setOutcome(null)
										setConfirmed([])
										read.mutate(chosen)
									}
								}}
							/>
							{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
							{perform.isError && (
								<Text variant="body-sm">
									{__(
										'The site may already hold part of this file. Import it again to see what is left.',
										'gophenberg',
									)}
								</Text>
							)}
							{plan !== null && (
								<PlanView plan={plan} confirmed={confirmed} onConfirm={confirming} />
							)}
							{outcome !== null && <OutcomeView outcome={outcome} />}
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Close', 'gophenberg')}
						</Button>
						{plan !== null && (
							<Button loading={perform.isPending} onClick={() => perform.mutate()}>
								{__('Apply', 'gophenberg')}
							</Button>
						)}
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
