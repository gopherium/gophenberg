// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { NavScreen } from '@gopherium/godmin'
import { Link } from '@tanstack/react-router'

import { useAddNew } from './useAddNew'
import { useContentType } from './useContentType'

/**
 * Renders the section sidebar screen of a content type.
 * @returns The drill-down screen listing the section's entries.
 */
export function ContentSidebar() {
	const listed = useContentType()
	const addNew = useAddNew(listed)
	return (
		<NavScreen
			title={listed.pluralLabel}
			back={<Link to="/" />}
			backLabel={__('Back', 'gophenberg')}
		>
			<Stack direction="column" gap="xs" render={<ul />}>
				<li>
					<Link
						to="/content/$typeKey"
						params={{ typeKey: listed.key }}
						className="gophenberg-menu__item"
					>
						{sprintf(__('All %(type)s', 'gophenberg'), { type: listed.pluralLabel })}
					</Link>
				</li>
				<li>
					<Button
						variant="unstyled"
						className="gophenberg-menu__item"
						loading={addNew.isPending}
						disabled={addNew.waiting}
						onClick={() => addNew.mutate()}
					>
						{__('Add New', 'gophenberg')}
					</Button>
				</li>
			</Stack>
			{addNew.error === null ? null : (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>{addNew.error.message}</Notice.Description>
				</Notice.Root>
			)}
		</NavScreen>
	)
}
