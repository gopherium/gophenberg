// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import { errorText } from '../i18n/errors'

/** The path the site's content definitions download from. */
export const definitionsDownloadPath = '/api/definitions'

/** The path a definitions file is planned against the site at. */
const definitionsPlanPath = '/api/definitions/plan'

/** The path a definitions file is performed against the site at. */
const definitionsApplyPath = '/api/definitions/apply'

const changeSchema = z.object({
	action: z.string(),
	subject: z.string(),
	key: z.string(),
	group: z.string().optional(),
	label: z.string(),
	reason: z.string().optional(),
})

const warningSchema = z.object({ code: z.string(), key: z.string() })

const planSchema = z.object({
	changes: z.array(changeSchema),
	warnings: z.array(warningSchema),
})

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

/** One definition an import would add, carry over, or take away. */
export interface PlanChange {
	action: string
	subject: string
	key: string
	group?: string
	label: string
	reason?: string
}

/** One change an import would make beyond the definitions themselves. */
export interface PlanWarning {
	code: string
	key: string
}

/** What an import would change about the site's definitions. */
export interface DefinitionsPlan {
	changes: PlanChange[]
	warnings: PlanWarning[]
}

/** The path the definitions standing apart from the plugins are listed at. */
const definitionsDriftPath = '/api/definitions/drift'

/** The path the site takes a plugin's definition over at. */
const definitionsAdoptPath = '/api/definitions/adopt'

/** The key the drift is cached under. */
export const driftQueryKey = ['definitions-drift']

const straySchema = z.object({
	subject: z.string(),
	key: z.string(),
	origin: z.string(),
	label: z.string(),
})

const driftSchema = z.object({
	orphans: z.array(straySchema),
	collisions: z.array(straySchema),
})

/** One definition standing apart from what the plugins declare. */
export interface Stray {
	subject: string
	key: string
	origin: string
	label: string
}

/** What the site holds that no plugin declares, and what a plugin wants that the site holds. */
export interface Drift {
	orphans: Stray[]
	collisions: Stray[]
}

/**
 * Returns the definitions standing apart from what the plugins declared at the last start.
 * @returns The orphans and the collisions.
 */
export async function readDrift(): Promise<Drift> {
	const response = await fetch(definitionsDriftPath)
	if (!response.ok) {
		throw new Error(`reading the definitions drift failed with status ${response.status}`)
	}
	return driftSchema.parse(await response.json())
}

/**
 * Takes a plugin's definition over as the site's own.
 * @param stray - The definition to take over.
 */
export async function adoptDefinition(stray: Stray): Promise<void> {
	const response = await fetch(definitionsAdoptPath, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ subject: stray.subject, key: stray.key }),
	})
	if (!response.ok) {
		const parsed = errorSchema.safeParse(await response.json().catch(() => null))
		throw new Error(errorText(parsed.success ? parsed.data : { error: '' }))
	}
}

/** One change an import may take away. */
export interface Confirmed {
	subject: string
	key: string
	group?: string
}

/** What an import did and what it left standing. */
export interface ImportOutcome {
	applied: PlanChange[]
	skipped: PlanChange[]
}

const outcomeSchema = z.object({
	applied: z.array(changeSchema),
	skipped: z.array(changeSchema),
})

/**
 * Performs a definitions file against the site, taking away only what the confirmations name.
 * @param file - The definitions file exactly as it was written.
 * @param confirm - The changes the admin agreed to have taken away.
 * @returns What the import did.
 */
export async function applyDefinitions(file: string, confirm: Confirmed[]): Promise<ImportOutcome> {
	const body = { ...(JSON.parse(file) as object), confirm }
	const response = await fetch(definitionsApplyPath, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
	})
	if (!response.ok) {
		const parsed = errorSchema.safeParse(await response.json().catch(() => null))
		throw new Error(errorText(parsed.success ? parsed.data : { error: '' }))
	}
	return outcomeSchema.parse(await response.json())
}

/**
 * Reports what a definitions file would change about the site, changing nothing.
 * @param file - The definitions file exactly as it was written.
 * @returns The plan the server answered.
 */
export async function planDefinitions(file: string): Promise<DefinitionsPlan> {
	const response = await fetch(definitionsPlanPath, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: file,
	})
	if (!response.ok) {
		const parsed = errorSchema.safeParse(await response.json().catch(() => null))
		throw new Error(errorText(parsed.success ? parsed.data : { error: '' }))
	}
	return planSchema.parse(await response.json())
}
