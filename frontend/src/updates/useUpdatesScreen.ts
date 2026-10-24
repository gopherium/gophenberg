// SPDX-License-Identifier: Apache-2.0

import { hashKey, useQuery, useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import type { Dispatch, SetStateAction } from 'react'

import { useListSettings } from '../settings/useListSettings'
import { fetchUpdates, startUpdate, updatesQueryKey } from './api'
import type { UpdateStart, UpdatesOverview } from './api'
import { heldRun, mayMoveOn, runView, startedRun } from './run'
import type { RunView, UpdateRun } from './run'

/** The hash the query cache files the overview of the updates under. */
const updatesHash = hashKey(updatesQueryKey)

/** What the Updates screen holds: the overview, whether it or the timings failed, the update followed and controls. */
export interface UpdatesScreenState {
	overview: UpdatesOverview | undefined
	unread: boolean
	view: RunView | undefined
	start: (asked: UpdateStart) => Promise<void>
	dismiss: () => void
}

/**
 * Returns how often to ask for the overview: as the site sets while the update followed may move on, never otherwise.
 * @param view - What the screen sees of the update it follows.
 * @param every - How many milliseconds the site waits between two asks.
 * @returns The interval, or false to stop asking.
 */
function askingEvery(view: RunView | undefined, every: number | undefined): number | false | undefined {
	return mayMoveOn(view) ? every : false
}

/** A read of the overview: whether it failed, and the overview it left, if any. */
interface OverviewRead {
	isError: boolean
	data: UpdatesOverview | undefined
}

/**
 * Reports whether the screen can show nothing to update: no overview arrived, or the timings an update follows failed.
 * @param read - The read of the overview.
 * @param timingsFailed - Whether the read of the site settings failed.
 * @returns True when the screen shows that the updates could not be read.
 */
function unreadable(read: OverviewRead, timingsFailed: boolean): boolean {
	const overviewLost = read.isError && read.data === undefined
	return overviewLost || timingsFailed
}

/**
 * Marks the wait for a restart as passed once the update stays restarting for as long as the site sets.
 * @param view - What the screen sees of the update it follows.
 * @param wait - How many milliseconds the site waits for a restart.
 * @param setWaited - What marks the wait as passed.
 */
function useRestartWait(view: RunView | undefined, wait: number | undefined, setWaited: (waited: boolean) => void) {
	const restarting = view?.restarting === true
	useEffect(() => {
		if (!restarting) {
			return
		}
		const timer = setTimeout(() => setWaited(true), wait)
		return () => clearTimeout(timer)
	}, [restarting, wait, setWaited])
}

/**
 * Keeps the update the screen holds in step with each read of the overview, picking up one under way.
 * @param client - The query client the overview is cached in.
 * @param setRun - What sets the update the screen holds.
 */
function useHeldRun(client: QueryClient, setRun: Dispatch<SetStateAction<UpdateRun | undefined>>) {
	useEffect(
		() =>
			client.getQueryCache().subscribe((event) => {
				if (event.type === 'updated' && event.query.queryHash === updatesHash) {
					const { data, status } = event.query.state
					setRun((held) => heldRun(held, data as UpdatesOverview | undefined, status === 'error'))
				}
			}),
		[client, setRun],
	)
}

/**
 * Returns the overview of the updates and the update the screen follows, asking again while one runs.
 * @returns The screen's state and the controls that start an update and dismiss its ending.
 */
export function useUpdatesScreen(): UpdatesScreenState {
	const client = useQueryClient()
	const settings = useListSettings()
	const timings = settings.data
	const [run, setRun] = useState<UpdateRun>()
	const [waited, setWaited] = useState(false)
	const read = useQuery({
		queryKey: updatesQueryKey,
		queryFn: fetchUpdates,
		retry: false,
		refetchIntervalInBackground: true,
		refetchInterval: (query) =>
			askingEvery(
				runView(run, query.state.data, query.state.status === 'error', waited),
				timings?.updates_progress_milliseconds,
			),
	})
	const view = timings === undefined ? undefined : runView(run, read.data, read.isError, waited)
	useRestartWait(view, timings?.updates_restart_wait_milliseconds, setWaited)
	useHeldRun(client, setRun)

	/**
	 * Starts the update asked for, then follows it from the overview the start answered, reading it again when refused.
	 * @param asked - The version to update to and whether to retry it.
	 */
	async function start(asked: UpdateStart) {
		const before = read.data as UpdatesOverview
		const answered = await startUpdate(asked).catch((cause: unknown) => {
			void client.invalidateQueries({ queryKey: updatesQueryKey })
			throw cause
		})
		setWaited(false)
		setRun(startedRun(asked, before, answered))
		client.setQueryData(updatesQueryKey, answered)
	}

	return {
		overview: read.data,
		unread: unreadable(read, settings.isError),
		view,
		start,
		dismiss: () => setRun(undefined),
	}
}
