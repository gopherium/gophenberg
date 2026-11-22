// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, SelectControl } from '@gophenberg/frontend-sdk'
import { FormDialog } from '@gopherium/godmin'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'

import {
	fieldRead,
	formAnswer,
	renameFieldInGroup,
	setFieldRequiredInGroup,
	setFieldSettingsInGroup,
} from './groups'
import { ChoicesEditor } from './ChoicesEditor'
import { DialogButton } from './fieldControls'
import { chosenOf } from './select'
import {
	LISTABLE,
	pairedSettings,
	settingAnswers,
	settingsOffered,
	storedSettings,
	typedSettings,
} from './settingControls'
import { pairsOf, requirable } from './types'
import type { Asked, Inside } from './fieldControls'

/**
 * Renders the control editing the settings a field carries and, while it asks, its form.
 * @param props - The group, the field, and what to report.
 * @returns The control and its form.
 */
export function FieldSettings(props: Inside) {
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
		mutationFn: (signal: AbortSignal) =>
			setFieldSettingsInGroup(
				fieldRead(props, opened),
				pairedSettings(opened.kind, storedSettings(offered, typed, opened.settings), pairs),
				signal,
			),
	})
	const said = sprintf(__('%(field)s settled.', 'gophenberg'), { field: props.field.label })
	return (
		<FormDialog
			title={props.title}
			submitLabel={__('Save settings', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={(signal) => formAnswer(save.mutateAsync(signal), said, props, signal)}
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
 * Renders the control flipping whether a field gates publishing.
 * @param props - The group, the field, and what to report.
 * @returns The control element.
 */
export function RequireField(props: Inside) {
	const flip = useMutation({
		mutationFn: () => setFieldRequiredInGroup(fieldRead(props, props.field), !props.field.required),
		onSuccess: async () => {
			const said = props.field.required
				? __('%(field)s is optional again.', 'gophenberg')
				: __('%(field)s is required now.', 'gophenberg')
			await props.onDone(sprintf(said, { field: props.field.label }))
		},
		onError: (cause: unknown) => props.onRefused(cause),
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

/**
 * Renders the control flipping whether a field stands as a column on the admin list.
 * @param props - The group, the field, and what to report.
 * @returns The control element, or nothing where the list never reaches.
 */
export function ListField(props: Inside) {
	const listed = props.field.settings.listed === true
	const flip = useMutation({
		mutationFn: () =>
			setFieldSettingsInGroup(fieldRead(props, props.field), { ...props.field.settings, listed: !listed }),
		onSuccess: async () => {
			const said = listed
				? __('%(field)s stays out of the list.', 'gophenberg')
				: __('%(field)s stands in the list.', 'gophenberg')
			await props.onDone(sprintf(said, { field: props.field.label }))
		},
		onError: (cause: unknown) => props.onRefused(cause),
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
export function RenameField(props: Inside) {
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
		mutationFn: (signal: AbortSignal) => renameFieldInGroup(fieldRead(props, opened), label, signal),
	})
	return (
		<FormDialog
			title={props.title}
			description={__('The name changes. Nothing stored under this field moves.', 'gophenberg')}
			submitLabel={__('Rename', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={(signal) =>
				formAnswer(
					rename.mutateAsync(signal),
					sprintf(__('%(field)s renamed.', 'gophenberg'), { field: label }),
					props,
					signal,
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
