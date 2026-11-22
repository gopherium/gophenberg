// SPDX-License-Identifier: Apache-2.0

import { FormDialog } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'

import { formAnswer } from './groups'
import { useFieldDraft } from './AddField'
import { DialogButton } from './fieldControls'
import { holdsFields } from './types'
import type { FieldPlace } from './AddField'
import type { Asked, Inside, Reporter } from './fieldControls'
import type { ContentType } from './types'

/**
 * Renders the control declaring a field inside the container and, while it asks, its form.
 * @param props - The group, the container, its path, the types, and what to report.
 * @returns The control and its form, or nothing for a field holding none.
 */
export function SubFields(props: Inside & { types: ContentType[]; path: string }) {
	const layouts = props.field.kind === 'flexible'
	const title = sprintf(
		layouts
			? __('Add layout to %(field)s', 'gophenberg')
			: __('Add field to %(field)s', 'gophenberg'),
		{ field: props.field.label },
	)
	const send = layouts ? __('Add layout', 'gophenberg') : __('Add field', 'gophenberg')
	if (!holdsFields(props.field.kind)) {
		return null
	}
	return (
		<DialogButton
			words={send}
			title={title}
			dialog={(asked) => (
				<SubFieldForm
					{...asked}
					group={props.group}
					types={props.types}
					path={props.path}
					parent={props.field.kind}
					send={send}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			)}
		/>
	)
}

/**
 * Renders the form declaring a field inside a container, empty on each ask.
 * @param props - Where the field is declared, what to report, the title, the send label and what takes it away.
 * @returns The form dialog, kept open with what was typed after a failure other than a stale write.
 */
function SubFieldForm(props: Reporter & FieldPlace & Asked & { send: string }) {
	const draft = useFieldDraft(props)
	return (
		<FormDialog
			title={props.title}
			submitLabel={props.send}
			cancelLabel={__('Cancel', 'gophenberg')}
			canSubmit={!draft.blocked}
			onSubmit={(signal) => formAnswer(draft.declare(signal), draft.said, props, signal)}
			onClose={props.onClose}
		>
			{draft.controls}
		</FormDialog>
	)
}
