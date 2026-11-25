// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'

import { fetchListSettings } from './api'

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
 * Reads the settings every list follows once a screen loaded them, never asking the server itself.
 * @returns The list settings query, whose data is the settings once a screen loaded them.
 */
function useLoadedListSettings() {
	return useQuery({ queryKey: listSettingsQueryKey, queryFn: fetchListSettings, staleTime: 'static', enabled: false })
}

/**
 * Returns the time limit a busy write gets, the one the site serves once a screen loaded its settings.
 * @returns The time limit in milliseconds, Infinity while the settings are not loaded.
 */
export function useWriteTimeout(): number {
	return useLoadedListSettings().data?.dialog_write_timeout_milliseconds ?? Infinity
}
