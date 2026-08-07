// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack } from '@gophenberg/frontend-sdk'
import { NavScreen } from '@gopherium/godmin'
import { useMutation } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'

import { createPost } from './api'

/**
 * Renders the posts section sidebar screen.
 * @returns The drill-down screen listing the section's entries.
 */
export function PostsSidebar() {
	const navigate = useNavigate()
	const addNew = useMutation({
		mutationFn: () => createPost(),
		onSuccess: (post) => navigate({ to: '/posts/$postId/edit', params: { postId: post.id } }),
	})
	return (
		<NavScreen title="Posts" back={<Link to="/" />}>
			<Stack direction="column" gap="xs" render={<ul />}>
				<li>
					<Link to="/posts" className="gophenberg-menu__item">
						All Posts
					</Link>
				</li>
				<li>
					<Button
						variant="unstyled"
						className="gophenberg-menu__item"
						loading={addNew.isPending}
						onClick={() => addNew.mutate()}
					>
						Add New
					</Button>
				</li>
			</Stack>
			{addNew.isError && (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>Could not create a draft.</Notice.Description>
				</Notice.Root>
			)}
		</NavScreen>
	)
}
