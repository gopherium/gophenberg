// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import type { QueryClient } from '@tanstack/react-query'
import { waitFor } from '@testing-library/react'
import { expect } from 'vitest'

import { updatesQueryKey } from '../updates/api'

/**
 * Serves the overview GET /api/updates answers.
 * @param overview - The overview to answer.
 */
export function serveUpdates(overview: object) {
	server.use(http.get('/api/updates', () => HttpResponse.json(overview)))
}

/**
 * Waits until the admin holds the overview of the updates, or knows it cannot.
 * @param client - The query client the admin renders with.
 * @param status - How the request for the overview settles.
 */
export async function updatesAnswered(client: QueryClient, status: 'success' | 'error' = 'success') {
	await waitFor(() => expect(client.getQueryState(updatesQueryKey)?.status).toBe(status))
}

/** The overview a site answers while no update is on offer. */
export const upToDate = {
	running: '0.20.0',
	can_update: true,
	refused: [] as string[],
	job: null,
	backups: { kept: 2, limit: 3, newest: '2026-10-21T14:02:00Z' },
	next_check: '2026-10-22T21:12:00Z',
	checked_at: '2026-10-22T09:12:00Z',
	offer: null,
	failed_at: null,
	error: '',
}

/** The offer a check found for the next patch release, with its release notes. */
export const patchOffer = {
	version: '0.20.1',
	minimum_version: '0.19.0',
	postgresql: 0,
	kits: [],
	notes: 'https://example.com/releases/0.20.1',
	auto: false,
	long_jump: false,
	needs_postgresql: false,
}

/** The overview a site answers while the patch release waits. */
export const patchWaiting = { ...upToDate, offer: patchOffer }

/**
 * Returns the job of the update from the version that runs to the patch release, standing at a step.
 * @param step - The step the job stands at.
 * @returns The job as the overview names it.
 */
function patchJob(step: string) {
	return { step, from: '0.20.0', to: '0.20.1', failure: null }
}

/**
 * Returns the overview a site answers while the update to the patch release stands at a step.
 * @param step - The step the update stands at.
 * @returns The overview, the version before the update still running.
 */
export function patchAt(step: string) {
	return { ...patchWaiting, job: patchJob(step) }
}

/**
 * Returns the overview the patch release answers once it runs, its update standing at a step.
 * @param step - The step the update stands at.
 * @returns The overview, which no longer offers the version that runs.
 */
export function patchRunning(step: string) {
	return { ...upToDate, running: '0.20.1', job: patchJob(step) }
}

/** The overview a site answers once the patch release failed its start check and the version before it runs again. */
export const patchWentBack = { ...patchAt('switched'), refused: ['0.20.1'] }

/** The overview a site answers once the rollback unit took back the patch release after it failed its start check. */
export const patchRolledBack = { ...patchAt('rolled-back'), refused: ['0.20.1'] }

/** The overview a site answers once a retry of the patch release the rollback unit took back failed at its download. */
export const patchRetryFailed = {
	...patchRolledBack,
	job: {
		...patchRolledBack.job,
		failure: {
			at: '2026-10-22T09:40:00Z',
			from: '0.20.0',
			to: '0.20.1',
			step: 'download',
			error: 'the checksum of the archive does not match the index',
		},
	},
}

/**
 * Returns the overview a site answers once a retry took its own backup and went back as the try before it did.
 * @param wentBack - The overview the site answered once the try before went back.
 * @returns The overview, which differs from it in its backups alone.
 */
export function backedUpAgain<T extends object>(wentBack: T): T {
	return { ...wentBack, backups: { kept: 3, limit: 3, newest: '2026-10-22T09:41:00Z' } }
}

/** The overview a site answers while the patch release, offered as raising the minimum version, waits. */
export const longJumpWaiting = { ...patchWaiting, offer: { ...patchOffer, long_jump: true } }

/** The overview the patch release answers once it raised the minimum version and failed its start check. */
export const patchStaysFailed = { ...patchRunning('failed'), refused: ['0.20.1'] }

/**
 * Returns what a check answers when it finds what the overview offers.
 * @param overview - The overview the check leaves behind.
 * @param overview.offer - The update the check found, or none.
 * @returns The result of the check.
 */
export function checkFinding(overview: { offer: object | null }) {
	return { checked_at: '2026-10-22T09:30:00Z', offer: overview.offer, failed_at: null, error: '' }
}

/** The settings that make the Updates screen ask for progress often and wait for a restart only briefly. */
export const briefUpdateTimings = {
	updates_progress_milliseconds: 10,
	updates_restart_wait_milliseconds: 800,
}

/** The overview a site answers after an update to the patch release failed at its download. */
export const patchFailed = {
	...patchWaiting,
	job: {
		step: 'committed',
		from: '0.19.0',
		to: '0.20.0',
		failure: {
			at: '2026-10-22T09:20:00Z',
			from: '0.20.0',
			to: '0.20.1',
			step: 'download',
			error: 'the checksum of the archive does not match the index',
		},
	},
}
