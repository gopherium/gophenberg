// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, VisuallyHidden } from '@gophenberg/frontend-sdk'
import { ConfirmBody, useToaster } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { useRefresh } from './actions'
import { trashPost } from './api'

/**
 * Renders the control moving the post being edited to the trash, and its confirm.
 * @param props - The post to trash and the name it is known by.
 * @returns The trash control and its confirm.
 */
export function TrashPost({ postId, title }: { postId: string, title: string }) {
	const navigate = useNavigate()
	const refresh = useRefresh()
	const toaster = useToaster()
	const [open, setOpen] = useState(false)
	const label = __('Move to trash', 'gophenberg')
	const named = title === '' ? __('(no title)', 'gophenberg') : title
	const trash = useMutation({
		mutationFn: () => trashPost(postId),
		onSuccess: async () => {
			await navigate({ to: '/content/$typeKey' })
			await refresh([postId])
			toaster.show(sprintf(__('"%s" moved to the trash.', 'gophenberg'), toaster.name(named)))
		},
	})
	return (
		<Dialog.Root
			open={open}
			onOpenChange={(next) => {
				trash.reset()
				setOpen(next)
			}}
		>
			<Dialog.Trigger render={<Button variant="outline" />}>{label}</Dialog.Trigger>
			<Dialog.Popup size="small">
				<VisuallyHidden render={<Dialog.Title />}>{label}</VisuallyHidden>
				<Dialog.Content>
					<ConfirmBody
						confirmLabel={label}
						cancelLabel={__('Cancel', 'gophenberg')}
						busy={trash.isPending}
						failure={trash.error?.message}
						onConfirm={() => trash.mutate()}
						onCancel={trash.isPending ? undefined : () => setOpen(false)}
					>
						<Dialog.Description render={<span />}>
							{sprintf(__('Move "%(title)s" to the trash?', 'gophenberg'), { title: toaster.name(named) })}
						</Dialog.Description>
					</ConfirmBody>
				</Dialog.Content>
			</Dialog.Popup>
		</Dialog.Root>
	)
}
