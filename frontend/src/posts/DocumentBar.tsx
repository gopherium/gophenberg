// SPDX-License-Identifier: Apache-2.0

import { Text } from '@gophenberg/frontend-sdk'

const TYPE_LABELS: Record<string, string> = {
	post: 'Post',
	page: 'Page',
}

/**
 * Renders the name and type of the post being edited.
 * @param props - The title the post holds and its type.
 * @returns The document bar.
 */
export function DocumentBar({ title, type }: { title: string, type: string }) {
	return (
		<div className="gophenberg-editor__document">
			<Text data-testid="document-bar" className="gophenberg-editor__document-name">
				{title === '' ? 'No title' : title} <span aria-hidden="true">&middot;</span>{' '}
				{TYPE_LABELS[type] ?? type}
			</Text>
		</div>
	)
}
