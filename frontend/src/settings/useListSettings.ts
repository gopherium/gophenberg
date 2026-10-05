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
	return useQuery({ queryKey: listSettingsQueryKey, queryFn: fetchListSettings, staleTime: 'static', retry: false })
}
