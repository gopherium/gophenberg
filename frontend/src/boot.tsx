// SPDX-License-Identifier: Apache-2.0

import { LoadingScreen } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'

/**
 * Renders the ghost the auth gate shows while the session loads.
 * @returns The boot loading element.
 */
export function BootLoading() {
	return <LoadingScreen label={__('Loading…', 'gophenberg')} />
}
