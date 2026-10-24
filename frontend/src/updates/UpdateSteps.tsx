// SPDX-License-Identifier: Apache-2.0

import { Link, Notice, Spinner, Text, speak } from '@gophenberg/frontend-sdk'
import { createInterpolateElement } from '@wordpress/element'
import { __, sprintf } from '@wordpress/i18n'
import { useEffect, useRef } from 'react'

import { adminBasepath } from '../basepath'
import { restoreSteps } from './links'
import { STEP_LINES } from './run'
import type { RunView, StepLine, UpdateRun } from './run'

/** The address of the Updates screen, which a full load opens on the admin the new version serves. */
const reloadAddress = `${adminBasepath}/update-core`

/**
 * Speaks the words of an ending once they show, since a notice does not announce itself.
 * @param words - The words to speak, or undefined to speak nothing.
 * @param urgency - How urgently a screen reader speaks them.
 */
export function useSpoken(words: string | undefined, urgency: 'polite' | 'assertive') {
	useEffect(() => {
		if (words !== undefined) {
			speak(words, urgency)
		}
	}, [words, urgency])
}

/**
 * Returns the words of each line of the steps, read fresh so the catalogue the reader loaded answers.
 * @param run - The update the steps follow.
 * @returns The words, keyed by line.
 */
function stepWords(run: UpdateRun): Record<StepLine, string> {
	return {
		download: sprintf(__('Downloading Gophenberg %(version)s…', 'gophenberg'), { version: run.to }),
		unpack: __('Unpacking the update…', 'gophenberg'),
		verify: __('Verifying the unpacked files…', 'gophenberg'),
		backup: __('Backing up the database…', 'gophenberg'),
		restart: __('Restarting Gophenberg…', 'gophenberg'),
		check: __('Checking the new version…', 'gophenberg'),
	}
}

/**
 * Renders the steps an update reached, one line each, the one under way marked, taking the focus nothing else holds.
 * @param props - The update and the line it stands at.
 * @returns The list element.
 */
export function UpdateSteps({ run, line }: { run: UpdateRun, line: StepLine }) {
	const list = useRef<HTMLOListElement>(null)
	useEffect(() => {
		if (document.activeElement === document.body) {
			(list.current as HTMLOListElement).focus()
		}
	}, [])
	const words = stepWords(run)
	const reached = STEP_LINES.slice(0, STEP_LINES.indexOf(line) + 1)
	return (
		<ol ref={list} tabIndex={-1} className="gophenberg-updates__steps" aria-live="polite">
			{reached.map((name) => (
				<li key={name} aria-current={name === line ? 'step' : undefined}>
					<Text>{words[name]}</Text>
					{name === line ? <Spinner /> : null}
				</li>
			))}
		</ol>
	)
}

/**
 * Renders the notice saying the update succeeded, with the link that loads the admin the new version serves.
 * @returns The notice element.
 */
function UpdatedNotice() {
	const words = __('Gophenberg updated successfully.', 'gophenberg')
	useSpoken(words, 'polite')
	return (
		<Notice.Root intent="success">
			<Notice.Description>{words}</Notice.Description>
			<Notice.Actions>
				<Notice.ActionLink href={reloadAddress}>{__('Reload the admin', 'gophenberg')}</Notice.ActionLink>
			</Notice.Actions>
		</Notice.Root>
	)
}

/**
 * Renders the notice saying the update failed and Gophenberg went back, which stays until dismissed.
 * @param props - The update and what dismisses the notice.
 * @returns The notice element.
 */
function WentBackNotice({ run, onDismiss }: { run: UpdateRun, onDismiss: () => void }) {
	const title = sprintf(__('The update to %(to)s failed.', 'gophenberg'), { to: run.to })
	const description = sprintf(__('Gophenberg went back to %(from)s and is running normally.', 'gophenberg'), {
		from: run.from,
	})
	useSpoken(`${title} ${description}`, 'assertive')
	return (
		<Notice.Root intent="warning">
			<Notice.Title>{title}</Notice.Title>
			<Notice.Description>{description}</Notice.Description>
			<Notice.CloseIconButton label={__('Dismiss', 'gophenberg')} onClick={onDismiss} />
		</Notice.Root>
	)
}

/**
 * Renders the notice saying the new version failed its start check and stays, with how to restore the backup.
 * @param props - The update that failed, which speaks its title unless it had ended when the screen found it.
 * @returns The notice element.
 */
function FailedNotice({ run }: { run: UpdateRun }) {
	const title = sprintf(
		__(
			'The update to %(to)s failed its start check, and Gophenberg cannot go back to %(from)s on its own.',
			'gophenberg',
		),
		{ to: run.to, from: run.from },
	)
	const steps = __(
		'Run <code>update:status</code> on the server to see the backup taken before the update. ' +
			'Then <restore>restore that backup</restore>, or fix what failed and start Gophenberg again.',
		'gophenberg',
	)
	useSpoken(run.ended ? undefined : title, 'assertive')
	return (
		<Notice.Root intent="error">
			<Notice.Title>{title}</Notice.Title>
			<Notice.Description>
				{createInterpolateElement(steps, { code: <code />, restore: <Link href={restoreSteps} /> })}
			</Notice.Description>
		</Notice.Root>
	)
}

/**
 * Renders the notice saying Gophenberg has not come back within the wait, and what to check on the server.
 * @returns The notice element.
 */
function NotBackNotice() {
	const title = __('Gophenberg has not come back yet.', 'gophenberg')
	const advice = __(
		'Gophenberg stopped after the switch so that its service manager, such as systemd, starts the new ' +
			'version. Check that the service manager starts it again after exit code <code>75</code>, and run ' +
			'<code>update:status</code> on the server to see where the update stands.',
		'gophenberg',
	)
	useSpoken(title, 'assertive')
	return (
		<Notice.Root intent="error">
			<Notice.Title>{title}</Notice.Title>
			<Notice.Description>{createInterpolateElement(advice, { code: <code /> })}</Notice.Description>
		</Notice.Root>
	)
}

/**
 * Renders the ending of the update the screen follows, or nothing while it runs or after it failed before its switch.
 * @param props - What the screen sees of the update, and what dismisses its ending.
 * @returns The notice element, or null.
 */
export function RunEnding({ view, onDismiss }: { view: RunView, onDismiss: () => void }) {
	switch (view.phase.kind) {
		case 'updated':
			return <UpdatedNotice />
		case 'rolled-back':
			return <WentBackNotice run={view.run} onDismiss={onDismiss} />
		case 'failed':
			return <FailedNotice run={view.run} />
		case 'not-back':
			return <NotBackNotice />
		default:
			return null
	}
}
