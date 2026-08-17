// SPDX-License-Identifier: Apache-2.0

import { Toaster } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'
import type { ReactNode } from 'react'

/**
 * Renders the toast region, naming its controls in the reader's language.
 * @param props - The tree the region wraps.
 * @returns The wrapped tree with its region.
 */
export function AdminToaster({ children }: { children: ReactNode }) {
	return <Toaster dismissLabel={__('Dismiss', 'gophenberg')}>{children}</Toaster>
}
