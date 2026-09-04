// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { adoptDefinition, driftQueryKey, readDrift } from './definitions'
import { groupsQueryKey } from './groups'
import { typesQueryKey } from './nav'
import type { Stray } from './definitions'

/**
 * Returns the sentence naming a definition whose plugin no longer declares it.
 * @param stray - The definition standing apart.
 * @returns The sentence to show.
 */
function orphanSentence(stray: Stray): string {
	return sprintf(
		__('%(name)s came from the %(plugin)s plugin, which no longer declares it.', 'gophenberg'),
		{ name: stray.label, plugin: stray.origin },
	)
}

/**
 * Returns the sentence naming a key a plugin wanted and the site keeps.
 * @param stray - The definition both want.
 * @returns The sentence to show.
 */
function collisionSentence(stray: Stray): string {
	return sprintf(
		__('The %(plugin)s plugin also declares %(name)s, and the site keeps the key.', 'gophenberg'),
		{ name: stray.label, plugin: stray.origin },
	)
}

/**
 * Renders the notices for what stands apart from the plugins, and the control taking an orphan over.
 * @param props - The reporters of what an adoption did or why it was turned away.
 * @returns The notices, or nothing when every definition stands where it belongs.
 */
export function DriftNotices(props: { onDone: (said: string) => void; onRefused: (cause: unknown) => void }) {
	const client = useQueryClient()
	const drift = useQuery({ queryKey: driftQueryKey, queryFn: readDrift })
	const adopt = useMutation({
		mutationFn: adoptDefinition,
		onSuccess: async (_answered, stray) => {
			props.onDone(sprintf(__('%(name)s belongs to the site now.', 'gophenberg'), { name: stray.label }))
			await client.invalidateQueries({ queryKey: driftQueryKey })
			await client.invalidateQueries({ queryKey: groupsQueryKey })
			await client.invalidateQueries({ queryKey: typesQueryKey })
		},
		onError: props.onRefused,
	})
	const held = drift.data
	if (held === undefined) {
		return null
	}
	return (
		<>
			{held.orphans.map((stray) => (
				<Notice.Root key={`orphan:${stray.subject}:${stray.key}`} intent="warning">
					<Notice.Description>
						<Stack direction="row" gap="sm" align="center">
							{orphanSentence(stray)}
							<Button
								variant="outline"
								loading={adopt.isPending}
								onClick={() => adopt.mutate(stray)}
							>
								{__('Adopt', 'gophenberg')}
							</Button>
						</Stack>
					</Notice.Description>
				</Notice.Root>
			))}
			{held.collisions.map((stray) => (
				<Notice.Root key={`collision:${stray.subject}:${stray.key}`} intent="warning">
					<Notice.Description>{collisionSentence(stray)}</Notice.Description>
				</Notice.Root>
			))}
		</>
	)
}
