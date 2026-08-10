// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice, LoadingRows, Page, useToaster } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
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
		return 'The built-in renderer is serving the public site.'
	}
	const fallen = fallbackReason(active)
	if (fallen !== '') {
		return `${active.name} ${fallen}, so the built-in renderer is serving.`
	}
	return active.version
		? `${active.name} ${active.version} is serving the public site.`
		: `${active.name} is serving the public site.`
}

/**
 * Returns why the built-in renderer answers instead of the chosen theme.
 * @param active - The chosen theme.
 * @returns The reason, empty when the theme is answering.
 */
function fallbackReason(active: Theme): string {
	if (active.broken !== '') {
		return 'will not load'
	}
	return active.serving ? '' : 'is not answering'
}

/**
 * Returns the label of the control returning the site to the previous choice.
 * @param target - The theme a rollback returns to, empty for the built-in renderer.
 * @returns The button label.
 */
export function rollbackLabel(target: string): string {
	return target === '' ? 'Roll back to the built-in renderer' : `Roll back to ${target}`
}

/**
 * Returns the archive a file field holds.
 * @param files - What the file field holds.
 * @returns The chosen archive, or null when the field holds none.
 */
export function chosenArchive(files: FileList | null): File | null {
	return files?.[0] ?? null
}

/**
 * Returns the sentence reporting that a theme now serves the public site.
 * @param name - The theme now serving, empty for the built-in renderer.
 * @returns The sentence to report.
 */
function servingNow(name: string): string {
	return name === ''
		? 'The built-in renderer is now serving the public site.'
		: `${name} is now serving the public site.`
}

/**
 * Renders the theme administration screen.
 * @returns The themes screen element.
 */
export function ThemesScreen() {
	const client = useQueryClient()
	const toaster = useToaster()
	const [refusal, setRefusal] = useState('')
	const themes = useQuery({
		queryKey: themesQueryKey,
		queryFn: ({ signal }) => listThemes(signal),
	})

	/**
	 * Reports what the server answered a theme action with.
	 * @param outcome - The outcome the server answered.
	 * @param said - The sentence naming what was done.
	 */
	async function done(outcome: ThemeOutcome, said: (name: string) => string) {
		if (outcome.kind === 'refused') {
			setRefusal(outcome.reason)
			return
		}
		setRefusal('')
		toaster.show(said(outcome.name))
		await client.invalidateQueries({ queryKey: themesQueryKey })
	}

	/**
	 * Reports that a theme action never reached the server.
	 */
	function failed() {
		setRefusal('The server could not be reached, so nothing was changed.')
	}

	const report: Reporter = { done, failed }
	const listed = themes.data ?? EMPTY_LIST
	return (
		<Page title="Themes" subtitle={servingLine(themes.data)}>
			<Stack direction="column" gap="md">
				{refusal !== '' && <ErrorNotice>{refusal}</ErrorNotice>}
				<UploadControl onOutcome={report} />
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
	done: (outcome: ThemeOutcome, said: (name: string) => string) => void
	failed: () => void
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
		return <ErrorNotice>Themes could not be loaded.</ErrorNotice>
	}
	if (props.loading) {
		return <LoadingRows label="Loading themes." />
	}
	if (props.themes.length === 0) {
		return <Text>No themes are installed.</Text>
	}
	return (
		<div
			className="godmin-table-scroll godmin-arrival"
			role="region"
			aria-label="Themes"
			tabIndex={0}
		>
			<table className="godmin-table">
				<thead>
					<tr>
						<th scope="col">Theme</th>
						<th scope="col">Version</th>
						<th scope="col">Status</th>
						<th scope="col" className="godmin-table__actions">
							Actions
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
				<Stack direction="column" gap="xs">
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
		return <Badge intent="high">Broken</Badge>
	}
	if (theme.serving) {
		return <Badge intent="stable">Serving</Badge>
	}
	if (theme.active) {
		return <Badge intent="high">Not serving</Badge>
	}
	return <Badge intent="draft">Installed</Badge>
}

/**
 * Renders the control putting a theme in front of the public site or taking it away.
 * @param props - The theme and the outcome reporter.
 * @returns The toggle element.
 */
function ServingToggle(props: { theme: Theme; onOutcome: Reporter }) {
	const { theme } = props
	const serve = useMutation({
		mutationFn: () => (theme.active ? deactivateTheme() : activateTheme(theme.name)),
		onSuccess: (outcome) => props.onOutcome.done(outcome, servingNow),
		onError: props.onOutcome.failed,
	})
	return (
		<Button
			variant="outline"
			aria-label={`${theme.active ? 'Deactivate' : 'Activate'} ${theme.name}`}
			loading={serve.isPending}
			onClick={() => serve.mutate()}
		>
			{theme.active ? 'Deactivate' : 'Activate'}
		</Button>
	)
}

/**
 * Renders the control returning the public site to the previous choice.
 * @param props - The rollback target and the outcome reporter.
 * @returns The rollback element.
 */
function RollbackControl(props: { target: string; onOutcome: Reporter }) {
	const roll = useMutation({
		mutationFn: rollbackTheme,
		onSuccess: (outcome) => props.onOutcome.done(outcome, servingNow),
		onError: props.onOutcome.failed,
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
 * Renders the control installing a packaged theme archive.
 * @param props - The outcome reporter.
 * @returns The upload element.
 */
function UploadControl(props: { onOutcome: Reporter }) {
	const [archive, setArchive] = useState<File | null>(null)
	const install = useMutation({
		mutationFn: (chosen: File) => uploadTheme(chosen),
		onSuccess: (outcome) => props.onOutcome.done(outcome, (name) => `${name} was installed.`),
		onError: props.onOutcome.failed,
	})
	return (
		<Stack direction="row" gap="sm">
			<label htmlFor="theme-archive">Theme archive</label>
			<input
				id="theme-archive"
				type="file"
				accept=".zip"
				onChange={(event) => setArchive(chosenArchive(event.target.files))}
			/>
			{archive !== null && (
				<Button
					disabled={install.isPending}
					loading={install.isPending}
					onClick={() => install.mutate(archive)}
				>
					Install theme
				</Button>
			)}
		</Stack>
	)
}
