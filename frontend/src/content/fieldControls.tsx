// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk'
import { useState } from 'react'
import type { ReactNode } from 'react'

import type { ContentField } from './types'

/** The kinds of field write a failure toast names. */
export type FieldWrite = 'save' | 'add' | 'order' | 'delete'

/** What a field control reports back once its write settles. */
export interface Reporter {
	onDone: (said: string) => Promise<void>
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
 * Renders a compact outline button and, while it asks, the dialog it opens.
 * @param props - The words on the button, the title it reads out, and what draws the dialog given that title and
 * what takes it away.
 * @returns The button and its dialog.
 */
export function DialogButton(props: { words: string; title: string; dialog: (asked: Asked) => ReactNode }) {
	const [asking, setAsking] = useState(false)
	return (
		<>
			<Button variant="outline" size="compact" aria-label={props.title} onClick={() => setAsking(true)}>
				{props.words}
			</Button>
			{asking ? props.dialog({ title: props.title, onClose: () => setAsking(false) }) : null}
		</>
	)
}
