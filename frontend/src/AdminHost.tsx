// SPDX-License-Identifier: Apache-2.0

import { AdminRoot } from '@gopherium/godmin'
import type { ReactNode } from 'react'

import { useLoadedListSettings } from './settings/useListSettings'

/**
 * Hosts the admin, giving a busy dialog's write the time limit the site serves once its settings loaded.
 * @param props - The tree to host.
 * @returns The host element.
 */
export function AdminHost({ children }: { children: ReactNode }) {
	const settings = useLoadedListSettings().data
	return <AdminRoot writeTimeout={settings?.dialog_write_timeout_milliseconds ?? Infinity}>{children}</AdminRoot>
}
