// SPDX-License-Identifier: Apache-2.0

import { Button, Checkbox, Stack, Text } from '@gophenberg/frontend-sdk'
import { useState } from 'react'
import type { ReactNode } from 'react'

import type { FieldDraft, FieldDrafts, RebasedField } from './fieldDraft'
import type { FieldNotes } from './fieldNotes'
import type { ContentField } from './types'

/** What a group's page holds for the fields it edits in place: their drafts, which stand open, and its Save. */
export interface FieldWork {
	drafts: FieldDrafts
	/** The dotted paths of the fields holding a changed draft, in page order. */
	changed: string[]
	/** The dotted paths of the fields standing open. */
	opened: ReadonlySet<string>
	/** What each field shows of its last save, under its dotted path. */
	notes: FieldNotes
	/** Whether the header Save runs. */
	saving: boolean
	/** Holds a field's draft under its dotted path, or drops it with its note when given none. */
	onDraft: (path: string, draft: FieldDraft | null) => void
	/** Opens or closes a field by its dotted path. */
	onOpen: (path: string, open: boolean) => void
	/** Drops the draft, the note and the open state of a field and of every field inside it. */
	onDrop: (path: string) => void
	/** Holds a field's draft moved onto the saved version, noting the parts where the saved value won. */
	onRebase: (path: string, moved: RebasedField) => void
	/** Runs the header Save while it is on. */
	onSave: () => void
}

/** The kinds of field write a failure toast names. */
export type FieldWrite = 'save' | 'add' | 'order' | 'delete'

/** What a field control reports back once its write settles. */
export interface Reporter {
	onDone: (said?: string) => Promise<void>
	onRefused: (cause: unknown, write?: FieldWrite) => Promise<void>
}

/** The group a field control acts inside. */
export interface Inside extends Reporter {
	group: number
	field: ContentField
	path?: string
}

/** What a form over one field takes beyond the field: its title and what takes it away. */
export interface Asked {
	title: string
	onClose: () => void
}

/**
 * Renders a checkbox labelled by the words beside it.
 * @param props - The words, whether the box is ticked, whether it is read only, and what to call with the new state.
 * @returns The box with its label.
 */
export function TickBox(props: {
	label: string
	checked: boolean
	readOnly: boolean
	onChange: (checked: boolean) => void
}) {
	return (
		<Stack direction="row" gap="sm" align="center" render={<label />}>
			<Checkbox checked={props.checked} readOnly={props.readOnly} onCheckedChange={props.onChange} />
			<Text>{props.label}</Text>
		</Stack>
	)
}

/**
 * Renders a compact outline button and, while it asks, the dialog it opens.
 * @param props - The words on the button, the title it reads out, whether it is off, and what draws the dialog given
 * that title and what takes it away.
 * @returns The button and its dialog.
 */
export function DialogButton(props: {
	words: string
	title: string
	disabled?: boolean
	dialog: (asked: Asked) => ReactNode
}) {
	const [asking, setAsking] = useState(false)
	return (
		<>
			<Button
				variant="outline"
				size="compact"
				aria-label={props.title}
				disabled={props.disabled}
				onClick={() => setAsking(true)}
			>
				{props.words}
			</Button>
			{asking ? props.dialog({ title: props.title, onClose: () => setAsking(false) }) : null}
		</>
	)
}
