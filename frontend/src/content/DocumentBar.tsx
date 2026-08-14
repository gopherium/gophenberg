// SPDX-License-Identifier: Apache-2.0

import { Text } from '@gophenberg/frontend-sdk'

import { useContentType } from './useContentType'

/**
 * Returns the name the editor knows a document by.
 * @param title - The title the post holds.
 * @returns The name to show.
 */
function documentName(title: string): string {
	return title === '' ? 'No title' : title
}

/**
 * Renders the name and type of the post being edited.
 * @param props - The title the post holds and its type.
 * @returns The document bar.
 */
export function DocumentBar({ title, type }: { title: string, type: string }) {
	const listed = useContentType()
	return (
		<div className="gophenberg-editor__document">
			<Text data-testid="document-bar" className="gophenberg-editor__document-name">
				{documentName(title)} <span aria-hidden="true">&middot;</span>{' '}
				{listed.key === type ? listed.singularLabel : type}
			</Text>
		</div>
	)
}
