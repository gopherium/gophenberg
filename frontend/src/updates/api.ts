// SPDX-License-Identifier: Apache-2.0

import { UPDATE_CORE, can, useSession } from '@gophenberg/frontend-sdk'
import { useQuery } from '@tanstack/react-query'
import { z } from 'zod'

import { errorText } from '../i18n/errors'

const failureSchema = z.object({
	at: z.string(),
	from: z.string(),
	to: z.string(),
	step: z.string(),
	error: z.string(),
})

const jobSchema = z.object({
	step: z.string(),
	from: z.string(),
	to: z.string(),
	failure: failureSchema.nullable(),
})

const offerSchema = z.object({
	version: z.string(),
	notes: z.string(),
	postgresql: z.number(),
	long_jump: z.boolean(),
	needs_postgresql: z.boolean(),
})

const resultSchema = z.object({
	checked_at: z.string().nullable(),
	offer: offerSchema.nullable(),
	failed_at: z.string().nullable(),
	error: z.string(),
})

const backupsSchema = z.object({
	kept: z.number(),
	limit: z.number(),
	newest: z.string().nullable(),
})

const overviewSchema = resultSchema.extend({
	running: z.string(),
	can_update: z.boolean(),
	cannot_update: z.string().optional(),
	refused: z.array(z.string()),
	job: jobSchema.nullable(),
	backups: backupsSchema,
	next_check: z.string().nullable(),
})

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

/** What GET /api/updates answers: what the checks found, the version that runs, its install and its last update. */
export type UpdatesOverview = z.infer<typeof overviewSchema>

/** The update a check found for this site. */
export type UpdateOffer = z.infer<typeof offerSchema>

/** The last update the install names, with the failure of the last one that stopped before its switch. */
export type UpdateJob = z.infer<typeof jobSchema>

/** What POST /api/updates/check answers: the last good check and the failure of any check since it. */
export type CheckResult = z.infer<typeof resultSchema>

/** How many backups the install keeps, how many it may keep, and when the newest was taken. */
export type BackupCount = z.infer<typeof backupsSchema>

/** What an administrator asks of the start of an update: the version to update to, and whether to retry it. */
export interface UpdateStart {
	version: string
	retry: boolean
}

/** The key the overview of the updates is cached under, which the menu, the nag and the Updates screen share. */
export const updatesQueryKey = ['updates']

/**
 * Returns the overview of the updates, raising when it cannot be read.
 * @returns The overview the server answers.
 */
export async function fetchUpdates(): Promise<UpdatesOverview> {
	const response = await fetch('/api/updates')
	if (!response.ok) {
		throw new Error(`loading the updates failed with status ${response.status}`)
	}
	return overviewSchema.parse(await response.json())
}

/**
 * Checks the update site now, raising with the reason the server gave when it refuses.
 * @returns What the check found.
 */
export async function checkForUpdates(): Promise<CheckResult> {
	const response = await fetch('/api/updates/check', { method: 'POST' })
	if (!response.ok) {
		throw await errorFrom(response)
	}
	return resultSchema.parse(await response.json())
}

/**
 * Starts the update an administrator asked for, raising with the reason the server gave when it refuses.
 * @param asked - The version to update to and whether to retry it.
 * @returns The overview the server answers once the update started.
 */
export async function startUpdate(asked: UpdateStart): Promise<UpdatesOverview> {
	const response = await fetch('/api/updates/apply', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(asked),
	})
	if (!response.ok) {
		throw await errorFrom(response)
	}
	return overviewSchema.parse(await response.json())
}

/**
 * Returns the error a refused request carries, in the reader's language.
 * @param response - The answer the server gave.
 * @returns The error to raise.
 */
async function errorFrom(response: Response): Promise<Error> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	return new Error(parsed.success ? errorText(parsed.data) : errorText({ error: '' }))
}

/**
 * Returns the overview of the updates to a session carrying update_core, asking and showing nothing to any other.
 * @returns The overview, or undefined before it arrives and for a session without update_core.
 */
export function useUpdates(): UpdatesOverview | undefined {
	const holds = can(useSession().data, UPDATE_CORE)
	const { data } = useQuery({ queryKey: updatesQueryKey, queryFn: fetchUpdates, enabled: holds })
	return holds ? data : undefined
}

/**
 * Returns the update that waits: the one on offer, unless this site refused its version.
 * @param overview - The overview of the updates, or nothing before it arrives.
 * @returns The offer, or undefined when none waits.
 */
export function waitingOffer(overview: UpdatesOverview | undefined): UpdateOffer | undefined {
	if (!overview?.offer || overview.refused.includes(overview.offer.version)) {
		return undefined
	}
	return overview.offer
}
