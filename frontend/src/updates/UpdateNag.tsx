// SPDX-License-Identifier: Apache-2.0

import { Link, Notice } from '@gophenberg/frontend-sdk'
import { Link as RouterLink, useMatchRoute } from '@tanstack/react-router'
import { createInterpolateElement } from '@wordpress/element'
import { __, sprintf } from '@wordpress/i18n'
import type { ReactNode } from 'react'

import { useUpdates, waitingOffer } from './api'
import type { UpdateOffer, UpdatesOverview } from './api'
import { notesLink } from './links'

/**
 * Renders the link to the Updates screen that a nag wraps around its last words, named by those words.
 * @param props - The words linked.
 * @returns The link element.
 */
function UpdatesLink({ children }: { children?: ReactNode }) {
	return <Link render={<RouterLink to="/update-core" />}>{children}</Link>
}

/**
 * Renders the nag saying the version on offer is available, its release notes and the Updates screen linked.
 * @param props - The update on offer.
 * @returns The notice element.
 */
function AvailableNag({ offer }: { offer: UpdateOffer }) {
	const notes = notesLink(offer)
	const sentence = sprintf(
		__('<notes>Gophenberg %(version)s</notes> is available! <update>Please update now</update>.', 'gophenberg'),
		{ version: offer.version },
	)
	return (
		<Notice.Root intent="warning">
			<Notice.Description>
				{createInterpolateElement(sentence, { notes, update: <UpdatesLink /> })}
			</Notice.Description>
		</Notice.Root>
	)
}

/**
 * Renders the nag saying the update to a version failed, with the Updates screen linked to try again.
 * @param props - The version the failed update went to.
 * @returns The notice element.
 */
function FailedNag({ version }: { version: string }) {
	const sentence = sprintf(
		__(
			'The update to Gophenberg %(version)s failed to complete. <retry>Please attempt the update again now</retry>.',
			'gophenberg',
		),
		{ version },
	)
	return (
		<Notice.Root intent="warning">
			<Notice.Description>{createInterpolateElement(sentence, { retry: <UpdatesLink /> })}</Notice.Description>
		</Notice.Root>
	)
}

/**
 * Returns the update the nag offers: the one that waits, unless the last failure already names its version.
 * @param overview - The overview of the updates.
 * @returns The offer, or undefined when none waits or the failure nag already names it.
 */
function nagOffer(overview: UpdatesOverview): UpdateOffer | undefined {
	const offer = waitingOffer(overview)
	return offer?.version === overview.job?.failure?.to ? undefined : offer
}

/**
 * Renders the nags an update_core holder sees above every admin screen but Updates, in the order WordPress draws them.
 * @returns The nag of the update that waits unless the failure names it, then the failure nag, or null.
 */
export function UpdateNag() {
	const overview = useUpdates()
	const onUpdates = useMatchRoute()({ to: '/update-core' }) !== false
	if (onUpdates || overview === undefined) {
		return null
	}
	const offer = nagOffer(overview)
	const failure = overview.job?.failure
	return (
		<>
			{offer === undefined ? null : <AvailableNag offer={offer} />}
			{failure ? <FailedNag version={failure.to} /> : null}
		</>
	)
}
