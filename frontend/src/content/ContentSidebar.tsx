// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { NavScreen } from '@gopherium/godmin'
import { useMutation } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'

import { createPost } from './api'
import { useContentType } from './useContentType'

/**
 * Renders the section sidebar screen of a content type.
 * @returns The drill-down screen listing the section's entries.
 */
export function ContentSidebar() {
	const navigate = useNavigate()
	const listed = useContentType()
	const addNew = useMutation({
		mutationFn: () => createPost(listed.key),
		onSuccess: (post) =>
			navigate({
				to: '/content/$typeKey/$postId/edit',
				params: { typeKey: listed.key, postId: post.id },
			}),
	})
	return (
		<NavScreen title={listed.pluralLabel} back={<Link to="/" />}>
			<Stack direction="column" gap="xs" render={<ul />}>
				<li>
					<Link
						to="/content/$typeKey"
						params={{ typeKey: listed.key }}
						className="gophenberg-menu__item"
					>
						{sprintf(__('All %s', 'gophenberg'), listed.pluralLabel)}
					</Link>
				</li>
				<li>
					<Button
						variant="unstyled"
						className="gophenberg-menu__item"
						loading={addNew.isPending}
						onClick={() => addNew.mutate()}
					>
						{__('Add New', 'gophenberg')}
					</Button>
				</li>
			</Stack>
			{addNew.isError && (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>{__('Could not create a draft.', 'gophenberg')}</Notice.Description>
				</Notice.Root>
			)}
		</NavScreen>
	)
}
