// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Stack, Text, uploadIcon } from '@gophenberg/frontend-sdk'
import { __, _x, sprintf } from '@wordpress/i18n'
import { ErrorNotice, FileButton, LoadingScreen, Page, useToaster } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ReactNode } from 'react'

import {
	activateTheme,
	deactivateTheme,
	listThemes,
	rollbackTheme,
	themesQueryKey,
	uploadTheme,
} from './api'
import type { Theme, ThemeList, ThemeOutcome } from './api'

const EMPTY_LIST: ThemeList = { themes: [], rollback: null }

/**
 * Returns the sentence naming the theme the public site is set to be served through.
 * @param listed - The listed themes, or undefined while none have been read.
 * @returns The sentence for the page subtitle, or undefined while nothing is known.
 */
export function servingLine(listed: ThemeList | undefined): string | undefined {
	if (listed === undefined) {
		return undefined
	}
	const active = listed.themes.find((theme) => theme.active)
	if (!active) {
		return __('The built-in renderer is serving the public site.', 'gophenberg')
	}
	const fallen = fallbackReason(active)
	if (fallen !== '') {
		return fallen
	}
	if (active.version === '') {
		return sprintf(__('%(theme)s is serving the public site.', 'gophenberg'), {
			theme: active.name,
		})
	}
	return sprintf(__('%(name)s %(version)s is serving the public site.', 'gophenberg'), {
		name: active.name,
		version: active.version,
	})
}

/**
 * Returns the sentence saying why the built-in renderer answers instead of the chosen theme.
 * @param active - The chosen theme.
 * @returns The sentence, empty when the theme is answering.
 */
function fallbackReason(active: Theme): string {
	if (active.broken !== '') {
		return sprintf(__('%(theme)s will not load, so the built-in renderer is serving.', 'gophenberg'), {
			theme: active.name,
		})
	}
	if (active.serving) {
		return ''
	}
	if (active.startFailed) {
		return sprintf(__('%(theme)s failed to start, so the built-in renderer is serving.', 'gophenberg'), {
			theme: active.name,
		})
	}
	return sprintf(__('%(theme)s is still starting, so the built-in renderer is serving.', 'gophenberg'), {
		theme: active.name,
	})
}

/**
 * Returns the label of the control returning the site to the previous choice.
 * @param target - The theme a rollback returns to, empty for the built-in renderer.
 * @returns The button label.
 */
export function rollbackLabel(target: string): string {
	return target === ''
		? __('Roll back to the built-in renderer', 'gophenberg')
		: sprintf(__('Roll back to %(theme)s', 'gophenberg'), { theme: target })
}

/**
 * Returns the sentence reporting that a theme now serves the public site.
 * @param name - The theme now serving, empty for the built-in renderer.
 * @returns The sentence to report.
 */
function servingNow(name: string): string {
	return name === ''
		? __('The built-in renderer is now serving the public site.', 'gophenberg')
		: sprintf(__('%(theme)s is now serving the public site.', 'gophenberg'), { theme: name })
}

/**
 * Renders the theme administration screen.
 * @returns The themes screen element.
 */
export function ThemesScreen() {
	const client = useQueryClient()
	const toaster = useToaster()
	const themes = useQuery({
		queryKey: themesQueryKey,
		queryFn: ({ signal }) => listThemes(signal),
	})

	/**
	 * Reports what the server answered a theme action with.
	 * @param outcome - The outcome the server answered.
	 * @param said - The sentence naming what was done.
	 * @param lead - The sentence naming what failed and why.
	 */
	async function done(outcome: ThemeOutcome, said: (name: string) => string, lead: (reason: string) => string) {
		if (outcome.kind === 'refused') {
			toaster.show(lead(outcome.reason))
			return
		}
		toaster.show(said(outcome.name))
		await client.invalidateQueries({ queryKey: themesQueryKey })
	}

	/**
	 * Reports that a theme action never reached the server.
	 * @param lead - The sentence naming what failed and why.
	 */
	function failed(lead: (reason: string) => string) {
		toaster.show(lead(__('The server could not be reached, so nothing was changed.', 'gophenberg')))
	}

	const report: Reporter = { done, failed }
	const listed = themes.data ?? EMPTY_LIST
	return (
		<Page
			title={__('Themes', 'gophenberg')}
			headingLevel={2}
			subtitle={servingLine(themes.data)}
			actions={<UploadTheme onOutcome={report} />}
		>
			<Stack direction="column" gap="md">
				{listed.rollback !== null && (
					<RollbackControl target={listed.rollback} onOutcome={report} />
				)}
				<ThemesBody
					themes={listed.themes}
					loading={themes.isPending}
					failed={themes.isError}
					onOutcome={report}
				/>
			</Stack>
		</Page>
	)
}

/**
 * Reports how a theme action ended.
 */
interface Reporter {
	done: (outcome: ThemeOutcome, said: (name: string) => string, lead: (reason: string) => string) => void
	failed: (lead: (reason: string) => string) => void
}

/**
 * Renders the themes list in whichever state it is in.
 * @param props - The listed themes, whether the list is loading or failed, and the outcome reporter.
 * @returns The list body element.
 */
function ThemesBody(props: {
	themes: Theme[]
	loading: boolean
	failed: boolean
	onOutcome: Reporter
}): ReactNode {
	if (props.failed) {
		return <ErrorNotice>{__('Themes could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	if (props.loading) {
		return <LoadingScreen label={__('Loading themes.', 'gophenberg')} />
	}
	if (props.themes.length === 0) {
		return <Text>{__('No themes are installed.', 'gophenberg')}</Text>
	}
	return (
		<div
			className="godmin-table-scroll"
			role="region"
			aria-label={__('Themes', 'gophenberg')}
			tabIndex={0}
		>
			<table className="godmin-table">
				<thead>
					<tr>
						<th scope="col">{__('Theme', 'gophenberg')}</th>
						<th scope="col">{__('Version', 'gophenberg')}</th>
						<th scope="col">{_x('Status', 'theme', 'gophenberg')}</th>
						<th scope="col" className="godmin-table__actions">
							{__('Actions', 'gophenberg')}
						</th>
					</tr>
				</thead>
				<tbody>
					{props.themes.map((theme) => (
						<ThemeRow key={theme.name} theme={theme} onOutcome={props.onOutcome} />
					))}
				</tbody>
			</table>
		</div>
	)
}

/**
 * Renders one installed theme with the action it offers.
 * @param props - The theme and the outcome reporter.
 * @returns The table row element.
 */
function ThemeRow(props: { theme: Theme; onOutcome: Reporter }) {
	const { theme } = props
	return (
		<tr>
			<td>{theme.name}</td>
			<td>{theme.version}</td>
			<td>
				<Stack direction="column" gap="xs" align="flex-start">
					<ThemeBadge theme={theme} />
					{theme.broken !== '' && <Text>{theme.broken}</Text>}
				</Stack>
			</td>
			<td className="godmin-table__actions">
				<ServingToggle theme={theme} onOutcome={props.onOutcome} />
			</td>
		</tr>
	)
}

/**
 * Renders the badge naming what state a theme is in.
 * @param props - The theme to describe.
 * @returns The badge element.
 */
function ThemeBadge(props: { theme: Theme }) {
	const { theme } = props
	if (theme.broken !== '') {
		return <Badge intent="high">{__('Broken', 'gophenberg')}</Badge>
	}
	if (theme.serving) {
		return <Badge intent="stable">{__('Serving', 'gophenberg')}</Badge>
	}
	if (theme.active) {
		return <Badge intent="high">{__('Not serving', 'gophenberg')}</Badge>
	}
	return <Badge intent="none">{__('Installed', 'gophenberg')}</Badge>
}

/**
 * Renders the control putting a theme in front of the public site or taking it away.
 * @param props - The theme and the outcome reporter.
 * @returns The toggle element.
 */
function ServingToggle(props: { theme: Theme; onOutcome: Reporter }) {
	const { theme } = props
	/**
	 * Returns the sentence naming what failed and why.
	 * @param reason - Why it failed.
	 * @returns The sentence to report.
	 */
	const lead = (reason: string) =>
		theme.active
			? sprintf(__('Could not deactivate the theme. %s', 'gophenberg'), reason)
			: sprintf(__('Could not activate the theme. %s', 'gophenberg'), reason)
	const serve = useMutation({
		mutationFn: () => (theme.active ? deactivateTheme() : activateTheme(theme.name)),
		onSuccess: (outcome) => props.onOutcome.done(outcome, servingNow, lead),
		onError: () => props.onOutcome.failed(lead),
	})
	const named = theme.active
		? sprintf(__('Deactivate %(theme)s', 'gophenberg'), { theme: theme.name })
		: sprintf(__('Activate %(theme)s', 'gophenberg'), { theme: theme.name })
	return (
		<Button
			variant="outline"
			aria-label={named}
			loading={serve.isPending}
			onClick={() => serve.mutate()}
		>
			{theme.active ? __('Deactivate', 'gophenberg') : __('Activate', 'gophenberg')}
		</Button>
	)
}

/**
 * Renders the control returning the public site to the previous choice.
 * @param props - The rollback target and the outcome reporter.
 * @returns The rollback element.
 */
function RollbackControl(props: { target: string; onOutcome: Reporter }) {
	/**
	 * Returns the sentence naming what failed and why.
	 * @param reason - Why it failed.
	 * @returns The sentence to report.
	 */
	const lead = (reason: string) => sprintf(__('Could not roll back the theme. %s', 'gophenberg'), reason)
	const roll = useMutation({
		mutationFn: rollbackTheme,
		onSuccess: (outcome) => props.onOutcome.done(outcome, servingNow, lead),
		onError: () => props.onOutcome.failed(lead),
	})
	return (
		<Stack direction="row" gap="sm">
			<Button variant="outline" loading={roll.isPending} onClick={() => roll.mutate()}>
				{rollbackLabel(props.target)}
			</Button>
		</Stack>
	)
}

/**
 * Renders the header control installing a packaged theme archive as soon as the reader picks it.
 * @param props - The outcome reporter.
 * @returns The upload button.
 */
function UploadTheme(props: { onOutcome: Reporter }) {
	/**
	 * Returns the sentence naming what failed and why.
	 * @param reason - Why it failed.
	 * @returns The sentence to report.
	 */
	const lead = (reason: string) => sprintf(__('Could not install the theme. %s', 'gophenberg'), reason)
	const install = useMutation({
		mutationFn: (chosen: File) => uploadTheme(chosen),
		onSuccess: (outcome) =>
			props.onOutcome.done(
				outcome,
				(name) => sprintf(__('%(theme)s was installed.', 'gophenberg'), { theme: name }),
				lead,
			),
		onError: () => props.onOutcome.failed(lead),
	})
	const label = __('Upload theme', 'gophenberg')
	return (
		<FileButton
			accept=".zip"
			icon={uploadIcon}
			busy={install.isPending}
			inputLabel={label}
			onChoose={([archive]) => install.mutate(archive)}
		>
			{label}
		</FileButton>
	)
}
