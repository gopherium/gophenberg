// SPDX-License-Identifier: Apache-2.0

import { Toaster } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'
import type { ReactNode } from 'react'

import { useListSettings } from './settings/useListSettings'

/**
 * Renders the toast region, naming its controls in the reader's language and timing its toasts as the site set.
 * @param props - The tree the region wraps.
 * @returns The wrapped tree with its region.
 */
export function AdminToaster({ children }: { children: ReactNode }) {
	const settings = useListSettings().data
	return (
		<Toaster
			dismissLabel={__('Dismiss', 'gophenberg')}
			dismissAfter={settings?.toast_milliseconds}
			nameLength={settings?.toast_name_length}
		>
			{children}
		</Toaster>
	)
}
