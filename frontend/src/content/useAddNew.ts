// SPDX-License-Identifier: Apache-2.0

import { useMutation, useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import { createPost } from './api'
import { seededValues } from './fieldDefaults'
import { typesQueryKey } from './nav'
import { listTypes } from './types'
import type { ContentType } from './types'

/**
 * Returns the mutation creating a draft of a type on the defaults its fields name, then opening it in the editor.
 * @param listed - The type to create a draft of.
 * @returns The mutation, its failure read as the server's reason or as the draft that could not be created, and
 *   whether it waits for the type registry that names those defaults.
 */
export function useAddNew(listed: ContentType) {
	const navigate = useNavigate()
	const registry = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const addNew = useMutation({
		mutationFn: () =>
			createPost(
				listed.key,
				{ title: '', fields: seededValues(listed.fields) },
				__('The draft could not be created.', 'gophenberg'),
			),
		onSuccess: (post) =>
			navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: listed.key, postId: post.id } }),
	})
	return { ...addNew, waiting: registry.isPending }
}
