// SPDX-License-Identifier: Apache-2.0

import type { ErrorMessages } from '@gopherium/react-auth/wpds/users'

import { errorText } from '../i18n/errors'

/**
 * Returns the words the admin gives the codes react-auth has no words for.
 * @returns The words, keyed by code.
 */
export function accountErrorMessages(): ErrorMessages {
	return { request_cross_origin: errorText({ error: '', code: 'request_cross_origin' }) }
}
