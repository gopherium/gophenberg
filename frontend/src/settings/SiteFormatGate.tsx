// SPDX-License-Identifier: Apache-2.0

import { FormatLocaleGate } from '@gopherium/gottext/react'
import type { ReactNode } from 'react'

import { useListSettings } from './useListSettings'

/**
 * Renders the screens once the site's settings named the locale dates and numbers are written in.
 * @param props - What stands in while the settings load, and the screens.
 * @returns The screens, or the stand in while the settings load.
 */
export function SiteFormatGate({ loading, children }: { loading: ReactNode; children: ReactNode }) {
	const settings = useListSettings()
	return (
		<FormatLocaleGate locale={settings.data?.format_locale} failed={settings.isError} loading={loading}>
			{children}
		</FormatLocaleGate>
	)
}
