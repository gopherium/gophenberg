// SPDX-License-Identifier: Apache-2.0

import { AdminRoot } from '@gopherium/godmin'
import { type ReactNode, useState } from 'react'

import { useLoadedListSettings } from './settings/useListSettings'

/** The name of the meta element the server writes the dialog write limit into. */
const dialogWriteLimitName = 'gophenberg-dialog-write-timeout'

/**
 * Reads the dialog write limit the server wrote into this page.
 * @returns The limit in whole milliseconds, or nothing when the page carries none it can use.
 */
function dialogWriteLimit(): number | null {
	const limit = Number(document.querySelector(`meta[name="${dialogWriteLimitName}"]`)?.getAttribute('content'))
	if (!Number.isSafeInteger(limit) || limit <= 0) {
		return null
	}
	return limit
}

/**
 * Hosts the admin, giving a busy dialog's write the limit written into the page, else the one the settings serve.
 * @param props - The tree to host.
 * @returns The host element.
 */
export function AdminHost({ children }: { children: ReactNode }) {
	const [fromPage] = useState<number | null>(dialogWriteLimit)
	const fromSettings = useLoadedListSettings().data?.dialog_write_timeout_milliseconds
	return <AdminRoot writeTimeout={fromPage ?? fromSettings ?? Infinity}>{children}</AdminRoot>
}
