// SPDX-License-Identifier: Apache-2.0

import { SelectControl } from '@gophenberg/frontend-sdk'
import { FormDialog } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'

import { formAnswer, moveField } from './groups'
import { DialogButton } from './fieldControls'
import { chosenOf } from './select'
import { holdsFields, standsInside } from './types'
import type { Asked, Inside } from './fieldControls'
import type { FieldGroup } from './groups'
import type { ContentField } from './types'

/** Where a field may come to stand: a group's top, or a container inside one, named for the select. */
interface Landing {
	label: string
	value: string
}

/**
 * Returns the containers the site keeps among the fields with the dotted path reaching each, however deep they stand.
 * @param fields - The fields to look through.
 * @param above - The dotted path holding the fields, empty at a group's top.
 * @returns The containers and their paths.
 */
function containersOf(fields: ContentField[], above: string): { label: string; path: string; kind: string }[] {
	const held: { label: string; path: string; kind: string }[] = []
	for (const field of fields) {
		if (!holdsFields(field.kind) || (field.origin ?? '') !== '') {
			continue
		}
		const path = above === '' ? field.key : `${above}.${field.key}`
		held.push({ label: field.label, path, kind: field.kind }, ...containersOf(field.fields, path))
	}
	return held
}

/**
 * Returns whether a container may take the field: not its own place, not itself, and not one of its own.
 * @param own - Whether the container stands in the field's own group.
 * @param path - The container's dotted path.
 * @param at - The field's dotted path.
 * @param parent - The dotted path of the container holding the field, empty at a group's top.
 * @returns Whether the container is offered.
 */
function takes(own: boolean, path: string, at: string, parent: string): boolean {
	if (!own) {
		return true
	}
	return path !== parent && path !== at && !path.startsWith(`${at}.`)
}

/**
 * Returns the places a field may be carried to: the top and every container of each group the site keeps, its own
 * place left out.
 * @param groups - The groups with their fields.
 * @param group - The group the field stands in.
 * @param at - The field's dotted path.
 * @param kind - The kind of the field being carried.
 * @returns The landings in group order, each group's top before its containers.
 */
function landingsFor(groups: FieldGroup[], group: number, at: string, kind: string): Landing[] {
	const parent = at.includes('.') ? at.slice(0, at.lastIndexOf('.')) : ''
	const held: Landing[] = []
	for (const listed of groups.filter((listed) => (listed.origin ?? '') === '')) {
		const own = listed.id === group
		if ((!own || parent !== '') && standsInside(kind)) {
			held.push({ label: listed.title, value: String(listed.id) })
		}
		for (const container of containersOf(listed.fields, '')) {
			if (takes(own, container.path, at, parent) && standsInside(kind, container.kind)) {
				held.push({ label: `${listed.title}: ${container.label}`, value: `${listed.id}:${container.path}` })
			}
		}
	}
	return held
}

/**
 * Renders the control carrying a field to a group's top or into a container, off while the field holds a draft.
 * @param props - The group, the field, its dotted path, every group it may land in, whether it or a field inside it
 * holds unsaved changes, and what to report.
 * @returns The control and its dialog, or nothing when there is nowhere to carry it.
 */
export function CarryField(props: Inside & { groups: FieldGroup[]; path: string; held: boolean }) {
	const at = props.path
	const landings = landingsFor(props.groups, props.group, at, props.field.kind)
	const first = landings[0]
	if (first === undefined) {
		return null
	}
	return (
		<CarryDialog
			group={props.group}
			field={props.field}
			at={at}
			landings={landings}
			first={first}
			held={props.held}
			onDone={props.onDone}
			onRefused={props.onRefused}
		/>
	)
}

/** Where a field may be carried: its path, the landings on offer, and the first of them. */
interface Carried {
	at: string
	landings: Landing[]
	first: Landing
}

/**
 * Renders the control carrying a field elsewhere and, while it asks, the form choosing where it lands.
 * @param props - The group, the field, its path, the landings on offer, the first of them, whether a draft holds the
 * control back, and what to report.
 * @returns The control and its form.
 */
function CarryDialog(props: Inside & Carried & { held: boolean }) {
	return (
		<DialogButton
			words={__('Move', 'gophenberg')}
			title={sprintf(__('Move %(field)s elsewhere', 'gophenberg'), { field: props.field.label })}
			disabled={props.held}
			dialog={(asked) => <CarryForm {...props} {...asked} />}
		/>
	)
}

/**
 * Renders the form choosing where a field lands and carrying it there, the first landing picked on each ask.
 * @param props - The group, the field, the landings, what to report, the title and what takes the form away.
 * @returns The form dialog, kept open after a failure other than a stale write.
 */
function CarryForm(props: Inside & Carried & Asked) {
	const { at, landings } = props
	const [picked, setPicked] = useState(props.first)
	const landing = landings.find((held) => held.value === picked.value) ?? props.first
	const carry = useMutation({
		mutationFn: (signal: AbortSignal) => {
			const [toGroup, toParent] = landing.value.split(':')
			return moveField(props.group, at, { group: Number(toGroup), parent: toParent }, signal)
		},
	})
	const said = sprintf(__('%(field)s moved.', 'gophenberg'), { field: props.field.label })
	const follows = !at.includes('.') && !landing.value.includes(':')
	return (
		<FormDialog
			title={props.title}
			description={
				follows
					? __('The field keeps its values on the content both groups reach.', 'gophenberg')
					: __('The values stored under it do not follow.', 'gophenberg')
			}
			submitLabel={__('Move the field', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={(signal) => formAnswer(carry.mutateAsync(signal), said, props, signal)}
			onClose={props.onClose}
		>
			{(busy) => (
				<SelectControl
					label={__('Destination', 'gophenberg')}
					items={landings}
					value={landing}
					readOnly={busy}
					onValueChange={(item) => setPicked(chosenOf(item, landings, landing))}
				/>
			)}
		</FormDialog>
	)
}
