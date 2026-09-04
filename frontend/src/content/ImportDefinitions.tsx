// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Dialog, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'

import { planDefinitions } from './definitions'
import type { DefinitionsPlan, PlanChange, PlanWarning } from './definitions'

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
 * Renders what the server said a definitions file would change.
 * @param props - The plan the server answered.
 * @returns The plan element.
 */
function PlanView(props: { plan: DefinitionsPlan }) {
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
			{props.plan.changes.map((held, at) => (
				<PlanRow key={`${held.subject}:${held.key}:${held.action}:${at}`} change={held} />
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
	const read = useMutation({
		mutationFn: (chosen: File) => chosen.text().then(planDefinitions),
		onSuccess: (answered) => {
			setNotice('')
			setPlan(answered)
		},
		onError: (cause) => {
			setPlan(null)
			setNotice(cause.message)
		},
	})
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
										read.mutate(chosen)
									}
								}}
							/>
							{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
							{plan !== null && <PlanView plan={plan} />}
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => setOpen(false)}>
							{__('Close', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
