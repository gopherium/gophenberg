// SPDX-License-Identifier: Apache-2.0

import { Stack, Text } from '@gophenberg/frontend-sdk'
import { Link, useParams } from '@tanstack/react-router'

/**
 * Renders the post editor.
 * @returns The editor screen element.
 */
export function EditorScreen() {
	const { postId } = useParams({ from: '/posts/$postId/edit' })
	return (
		<Stack direction="column" gap="md">
			<Text variant="heading-lg" render={<h1 />}>
				Editor
			</Text>
			<Text>Editing {postId}. The block editor arrives in Part E.</Text>
			<Link to="/posts">Back to posts</Link>
		</Stack>
	)
}
