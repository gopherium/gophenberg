// SPDX-License-Identifier: Apache-2.0

import { Link } from '@gophenberg/frontend-sdk'
import type { ReactElement } from 'react'

import type { UpdateOffer } from './api'

/** What a release notes address starts with when it names a web page. */
const webPage = /^https?:\/\//i

/** The documentation page that shows how to update Gophenberg. */
export const updatingPage = 'https://docs.gophenberg.org/self-hosting/updates-and-backups/'

/** The part of that page naming what to back up. */
export const backupAdvice = `${updatingPage}#what-to-back-up`

/** The part of that page showing how a server install restores a backup of the database. */
export const restoreSteps = `${updatingPage}#restoring-on-a-server-install`

/** The class that keeps the name and the number of a version on one line. */
const versionClass = 'gophenberg-updates__version'

/**
 * Returns the element a sentence wraps around the version on offer: its release notes linked, or the words alone.
 * @param offer - The update on offer.
 * @returns A link to the release notes when they are a web page, otherwise a span, both kept on one line.
 */
export function notesLink(offer: UpdateOffer): ReactElement {
	if (webPage.test(offer.notes)) {
		return <Link href={offer.notes} className={versionClass} />
	}
	return <span className={versionClass} />
}
