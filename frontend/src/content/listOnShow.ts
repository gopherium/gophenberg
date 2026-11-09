// SPDX-License-Identifier: Apache-2.0

import type { ListRun } from '@gopherium/godmin'
import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useCallback, useLayoutEffect } from 'react'

/** The run of the posts list each admin shows, by the query client the admin runs on. */
const onShow = new WeakMap<QueryClient, ListRun>()

/**
 * Shares the run of the posts list on show with a write raised outside it, for as long as the list stays.
 * @param run - What the list does around a run.
 */
export function useShareListRun(run: ListRun): void {
	const client = useQueryClient()
	useLayoutEffect(() => {
		onShow.set(client, run)
		return () => {
			onShow.delete(client)
		}
	}, [client, run])
}

/**
 * Returns what reads the run of the posts list on show at the moment it is asked.
 * @returns The reader, giving none while no posts list shows.
 */
export function useListOnShow(): () => ListRun | undefined {
	const client = useQueryClient()
	return useCallback(() => onShow.get(client), [client])
}
