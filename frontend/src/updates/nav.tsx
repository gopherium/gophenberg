// SPDX-License-Identifier: Apache-2.0

import { UPDATE_CORE, updateIcon } from '@gophenberg/frontend-sdk'
import type { MenuItem } from '@gophenberg/frontend-sdk'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'

import { useUpdates, waitingOffer } from './api'

/**
 * Returns the Updates row of the sidebar, counting the update that waits as WordPress counts it.
 * @returns The menu item, its count one while an update waits and zero otherwise.
 */
export function useUpdatesMenuItem(): MenuItem {
	const waiting = waitingOffer(useUpdates()) === undefined ? 0 : 1
	return {
		label: __('Updates', 'gophenberg'),
		to: '/update-core',
		icon: updateIcon,
		capability: UPDATE_CORE,
		count: {
			value: waiting,
			label: sprintf(_n('%s update available', '%s updates available', waiting, 'gophenberg'), formatNumber(waiting)),
		},
	}
}
