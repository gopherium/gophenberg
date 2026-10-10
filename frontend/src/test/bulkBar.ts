// SPDX-License-Identifier: Apache-2.0

import { within } from '@testing-library/react'

/**
 * Returns the queries of the bulk bar under a list, apart from the primary actions each row draws as buttons.
 * @returns The queries bound to the action buttons of the bar.
 */
export function bulkBar() {
	return within(document.querySelector('.dataviews-bulk-actions-footer__action-buttons') as HTMLElement)
}
