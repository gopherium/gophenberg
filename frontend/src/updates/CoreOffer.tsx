// SPDX-License-Identifier: Apache-2.0

import { AlertDialog, Button, Link, Notice, Stack, Text } from '@gophenberg/frontend-sdk'
import { SectionTitle } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { createInterpolateElement } from '@wordpress/element'
import { __, _n, sprintf } from '@wordpress/i18n'

import { errorText } from '../i18n/errors'
import type { BackupCount, UpdateJob, UpdateOffer, UpdateStart, UpdatesOverview } from './api'
import { backupAdvice, notesLink, updatingPage } from './links'
import { mayMoveOn } from './run'
import type { RunView } from './run'
import { RunEnding, UpdateSteps, useSpoken } from './UpdateSteps'
import { dateAndTime } from './words'

/** What the offer needs: the overview, the update the screen follows, and the controls of an update. */
interface OfferProps {
	overview: UpdatesOverview
	view: RunView | undefined
	onStart: (asked: UpdateStart) => Promise<void>
	onDismiss: () => void
}

/**
 * Renders WordPress's warning to back up before updating, linking the backup advice and the page on updating.
 * @returns The notice element.
 */
function BackUpFirst() {
	const sentence = __(
		'<strong>Important:</strong> Before updating, please <backup>back up your database and files</backup>. ' +
			'For help with updates, visit the <docs>Updating Gophenberg</docs> documentation page.',
		'gophenberg',
	)
	return (
		<Notice.Root intent="warning">
			<Notice.Description>
				{createInterpolateElement(sentence, {
					strong: <strong />,
					backup: <Link href={backupAdvice} />,
					docs: <Link href={updatingPage} />,
				})}
			</Notice.Description>
		</Notice.Root>
	)
}

/**
 * Renders the sentence saying the PostgreSQL server is too old for the update on offer.
 * @param props - The update on offer.
 * @returns The line element, the version on offer linked to its release notes.
 */
function BlockedLine({ offer }: { offer: UpdateOffer }) {
	const sentence = sprintf(
		__(
			'You cannot update because <notes>Gophenberg %(version)s</notes> requires PostgreSQL version ' +
				'%(postgresql)s or higher.',
			'gophenberg',
		),
		{ version: offer.version, postgresql: formatNumber(offer.postgresql) } as never,
	)
	return <Text render={<p />}>{createInterpolateElement(sentence, { notes: notesLink(offer) })}</Text>
}

/**
 * Renders the sentence leading to the button, naming the version that runs and the update on offer.
 * @param props - The version that runs and the update on offer.
 * @returns The line element, the version on offer linked to its release notes.
 */
function UpdateLine({ running, offer }: { running: string, offer: UpdateOffer }) {
	const sentence = sprintf(
		__('You can update from Gophenberg %(running)s to <notes>Gophenberg %(version)s</notes> manually:', 'gophenberg'),
		{ running, version: offer.version },
	)
	return <Text render={<p />}>{createInterpolateElement(sentence, { notes: notesLink(offer) })}</Text>
}

/**
 * Reports whether the install refuses a version, which then only a retry installs.
 * @param overview - The overview of the updates.
 * @param version - The version asked about.
 * @returns True when the version failed its start check here.
 */
function refuses(overview: UpdatesOverview, version: string): boolean {
	return overview.refused.includes(version)
}

/**
 * Returns what the confirm says before an update: the backup comes first, and what happens when the update fails.
 * @param offer - The update on offer.
 * @param running - The version that runs.
 * @returns The description of the confirm.
 */
function confirmWords(offer: UpdateOffer, running: string): string {
	if (offer.long_jump) {
		return __(
			'A backup of the database is taken first, and the site stays down until you restore it ' +
				'if the new version does not start.',
			'gophenberg',
		)
	}
	return sprintf(
		__(
			'A backup of the database is taken first, and Gophenberg goes back to %(running)s on its own ' +
				'if the new version does not start.',
			'gophenberg',
		),
		{ running } as never,
	)
}

/**
 * Returns the words of the button: a reinstall for a refused version, as WordPress names it, an update otherwise.
 * @param offer - The update on offer.
 * @param retry - Whether the button tries a refused version again.
 * @returns The words of the button.
 */
function buttonWords(offer: UpdateOffer, retry: boolean): string {
	const words = retry
		? __('Re-install version %(version)s', 'gophenberg')
		: __('Update to version %(version)s', 'gophenberg')
	return sprintf(words, { version: offer.version })
}

/**
 * Renders the button updating to the version on offer and the confirm it asks first, a retry for a refused version.
 * @param props - The update on offer, the overview, and what starts an update.
 * @returns The button and its confirm.
 */
function UpdateButton({ offer, overview, onStart }: Pick<OfferProps, 'overview' | 'onStart'> & { offer: UpdateOffer }) {
	const retry = refuses(overview, offer.version)
	const confirm = async () => {
		try {
			await onStart({ version: offer.version, retry })
		} catch (cause) {
			return { close: false, error: (cause as Error).message }
		}
	}
	return (
		<AlertDialog.Root onConfirm={confirm}>
			<AlertDialog.Trigger render={<Button variant={retry ? 'outline' : 'solid'} />}>
				{buttonWords(offer, retry)}
			</AlertDialog.Trigger>
			<AlertDialog.Popup
				title={sprintf(__('Update to Gophenberg %(version)s?', 'gophenberg'), { version: offer.version })}
				description={confirmWords(offer, overview.running)}
				confirmButtonText={__('Update', 'gophenberg')}
				cancelButtonText={__('Cancel', 'gophenberg')}
			/>
		</AlertDialog.Root>
	)
}

/**
 * Renders why this install updates by hand and where the steps are, in place of the button.
 * @param props - The reason the server names, if any.
 * @returns The lines element.
 */
function ByHand({ reason }: { reason: string | undefined }) {
	const why = errorText({ error: __('This install of Gophenberg updates by hand.', 'gophenberg'), code: reason })
	const steps = __(
		'Follow the steps on the <docs>Updating Gophenberg</docs> documentation page to update by hand.',
		'gophenberg',
	)
	return (
		<>
			<Text render={<p />}>{why}</Text>
			<Text render={<p />}>{createInterpolateElement(steps, { docs: <Link href={updatingPage} /> })}</Text>
		</>
	)
}

/**
 * Renders the steps of the update the screen follows while it runs, or nothing.
 * @param props - What the screen sees of the update, if anything.
 * @returns The steps element, or null.
 */
function RunSteps({ view }: { view: RunView | undefined }) {
	if (view === undefined || view.phase.kind !== 'steps') {
		return null
	}
	return <UpdateSteps run={view.run} line={view.phase.line} />
}

/**
 * Renders the steps by hand, or the button, led by its sentence outside a run unless the version on offer is refused.
 * @param props - The offer's props and the update on offer.
 * @returns The element, or null while the PostgreSQL server is too old for it.
 */
function OfferAction(props: OfferProps & { offer: UpdateOffer }) {
	const { view, overview, offer } = props
	if (offer.needs_postgresql) {
		return null
	}
	if (!overview.can_update) {
		return <ByHand reason={overview.cannot_update} />
	}
	const leads = view === undefined && !refuses(overview, offer.version)
	return (
		<>
			{leads ? <UpdateLine running={overview.running} offer={offer} /> : null}
			<UpdateButton offer={offer} overview={overview} onStart={props.onStart} />
			<Text render={<p />}>
				{__(
					'While your site is being updated, it keeps serving until Gophenberg restarts on the new version.',
					'gophenberg',
				)}
			</Text>
		</>
	)
}

/**
 * Renders the update on offer: the warning outside a run, why it cannot install, a long jump, a failed try, its action.
 * @param props - The offer's props and the update on offer.
 * @returns The offer elements, or null while an update may move on.
 */
function OfferDetails(props: OfferProps & { offer: UpdateOffer }) {
	const { overview, offer, view } = props
	if (mayMoveOn(view)) {
		return null
	}
	const refused = sprintf(
		__(
			'Gophenberg %(version)s failed its start check on this site. Fix what failed before you try it again.',
			'gophenberg',
		),
		{ version: offer.version },
	)
	const longJump = __(
		'This update needs a backup of the database, and the site stays down if the new version does not start.',
		'gophenberg',
	)
	return (
		<>
			{view === undefined ? <BackUpFirst /> : null}
			{offer.needs_postgresql ? <BlockedLine offer={offer} /> : null}
			{offer.long_jump ? <Text render={<p />}>{longJump}</Text> : null}
			{refuses(overview, offer.version) ? <Text render={<p />}>{refused}</Text> : null}
			<OfferAction {...props} offer={offer} />
		</>
	)
}

/**
 * Renders the last update that failed before anything changed, with the reason the server gave, or nothing.
 * @param props - The last update the install names, and whether it is the ending of the update followed.
 * @returns The notice element, or null.
 */
function FailedBefore({ job, live }: { job: UpdateJob | null, live: boolean }) {
	const title = __('The update failed before anything changed.', 'gophenberg')
	useSpoken(live ? title : undefined, 'assertive')
	const failure = job?.failure
	if (!failure) {
		return null
	}
	return (
		<Notice.Root intent="error">
			<Notice.Title>{title}</Notice.Title>
			<Notice.Description>{failure.error}</Notice.Description>
		</Notice.Root>
	)
}

/**
 * Renders how many backups the install keeps and when the newest was taken, or nothing while it keeps none.
 * @param props - The backups the server counts.
 * @returns The line element, or null.
 */
export function BackupsLine({ backups }: { backups: BackupCount }) {
	if (backups.newest === null) {
		return null
	}
	const sentence = sprintf(
		_n(
			'Backups kept: %(kept)s of %(limit)s, taken on %(date)s at %(time)s.',
			'Backups kept: %(kept)s of %(limit)s, the newest on %(date)s at %(time)s.',
			backups.kept,
			'gophenberg',
		),
		{ kept: formatNumber(backups.kept), limit: formatNumber(backups.limit), ...dateAndTime(backups.newest) },
	)
	return <Text render={<p />}>{sentence}</Text>
}

/**
 * Returns the heading of the core update: one on offer this site takes, the latest version once a check ran, or none.
 * @param overview - The overview of the updates.
 * @param moving - Whether the update the screen follows may still move on.
 * @returns The heading, or undefined before a check, while an update may move on, or while a version is refused.
 */
function offerHeading(overview: UpdatesOverview, moving: boolean): string | undefined {
	if (overview.offer !== null) {
		return refuses(overview, overview.offer.version)
			? undefined
			: __('An updated version of Gophenberg is available.', 'gophenberg')
	}
	const latest = overview.checked_at !== null && !refuses(overview, overview.running)
	return latest && !moving ? __('You have the latest version of Gophenberg.', 'gophenberg') : undefined
}

/**
 * Renders the core update: whether one is available, the ending of the update followed, the offer, and the steps.
 * @param props - The overview, the update the screen follows, and the controls of an update.
 * @returns The section element.
 */
export function CoreOffer(props: OfferProps) {
	const { overview, view } = props
	const offer = overview.offer
	const phase = view?.phase.kind
	const heading = offerHeading(overview, mayMoveOn(view))
	return (
		<Stack direction="column" gap="md">
			{heading === undefined ? null : <SectionTitle>{heading}</SectionTitle>}
			{view === undefined ? null : <RunEnding view={view} onDismiss={props.onDismiss} />}
			{phase === 'steps' ? null : <FailedBefore job={overview.job} live={phase === 'over'} />}
			{offer === null ? null : <OfferDetails {...props} offer={offer} />}
			<RunSteps view={view} />
		</Stack>
	)
}
