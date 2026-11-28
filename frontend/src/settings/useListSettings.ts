// SPDX-License-Identifier: Apache-2.0

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useSyncExternalStore } from 'react'

import { fetchListSettings } from './api'
import type { ListSettings } from './api'

/** The query naming the settings every list follows. */
const listSettingsQueryKey = ['list-settings'] as const

/**
 * Loads the settings every list follows.
 * @returns The list settings query, whose data is the settings.
 */
export function useListSettings() {
	return useQuery({
		queryKey: listSettingsQueryKey,
		queryFn: fetchListSettings,
		staleTime: 'static',
		retry: false,
		retryOnMount: false,
	})
}

/**
 * Returns the time limit a busy write gets, the one the site serves once a screen loaded its settings.
 * @returns The time limit in milliseconds, Infinity while the settings are not loaded.
 */
export function useWriteTimeout(): number {
	const client = useQueryClient()
	const subscribe = useCallback((changed: () => void) => client.getQueryCache().subscribe(changed), [client])
	return useSyncExternalStore(
		subscribe,
		() => client.getQueryData<ListSettings>(listSettingsQueryKey)?.dialog_write_timeout_milliseconds ?? Infinity,
	)
}
