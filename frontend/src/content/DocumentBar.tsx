// SPDX-License-Identifier: Apache-2.0

import { Text } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

import { useContentType } from './useContentType'

/**
 * Returns the name the editor knows a document by.
 * @param title - The title the post holds.
 * @returns The name to show.
 */
function documentName(title: string): string {
	return title === '' ? __('No title', 'gophenberg') : title
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
				{documentName(title)} <span aria-hidden="true">{'\u00b7'}</span>{' '}
				{listed.key === type ? listed.singularLabel : type}
			</Text>
		</div>
	)
}
