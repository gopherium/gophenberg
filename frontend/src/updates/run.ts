// SPDX-License-Identifier: Apache-2.0

import type { UpdateJob, UpdateStart, UpdatesOverview } from './api'

/** An update the screen follows: its versions, its retry, the failure and backup at its start, and how it stands. */
export interface UpdateRun {
	to: string
	from: string
	retry: boolean
	priorFailure: string | undefined
	priorBackup: string | null
	moved: boolean
	ended: boolean
}

/** A line of the steps an update shows, in the order it reaches them. */
export type StepLine = 'download' | 'unpack' | 'verify' | 'backup' | 'restart' | 'check'

/** How an update ended: kept, gone back, failed on the new version, or failed before its switch. */
type Ending = 'updated' | 'rolled-back' | 'failed' | 'over'

/** Where an update stands: at a line of its steps, at one of its endings, or past the wait for its restart. */
type RunPhase = { kind: 'steps', line: StepLine } | { kind: Ending | 'not-back' }

/** What the screen sees of an update: the update, where it stands, and whether it restarts. */
export interface RunView {
	run: UpdateRun
	phase: RunPhase
	restarting: boolean
}

/** The overview last read, and whether the last read of it failed. */
interface Seen {
	overview: UpdatesOverview
	failing: boolean
}

/** The lines an update reaches, in order. */
export const STEP_LINES: StepLine[] = ['download', 'unpack', 'verify', 'backup', 'restart', 'check']

/** The line each step of the job shows, the download standing for any step not named. */
const LINE_OF_STEP: Record<string, StepLine> = { stage: 'unpack', backup: 'backup', switched: 'restart' }

/** The steps of a job that still runs on the version it started from. */
const UNDER_WAY = ['download', 'stage', 'backup']

/** The steps of a job past its switch that a version going back leaves. */
const PAST_SWITCH = ['switched', 'rolled-back']

/** The lines an update shows while Gophenberg restarts and checks the new version. */
const RESTARTING: StepLine[] = ['restart', 'check']

/** The ending each step of the job of an update names once the version it went to runs. */
const ENDING_ON_NEW: Partial<Record<string, Ending>> = { committed: 'updated', failed: 'failed' }

/** The phases an update may still move on from: its steps, and the wait for its restart that passed. */
const OPEN_PHASES: RunPhase['kind'][] = ['steps', 'not-back']

/**
 * Returns the update an administrator just started.
 * @param asked - The version it updates to and whether it retries it.
 * @param before - The overview the screen held when the update started.
 * @param answered - The overview the start answered with.
 * @returns The update, which has not moved yet.
 */
export function startedRun(asked: UpdateStart, before: UpdatesOverview, answered: UpdatesOverview): UpdateRun {
	return {
		to: asked.version,
		from: before.running,
		retry: asked.retry,
		priorFailure: before.job?.failure?.at,
		priorBackup: answered.backups.newest,
		moved: false,
		ended: false,
	}
}

/**
 * Returns the update the overview names as under way or as failed on the version that runs, which the screen follows.
 * @param overview - The overview of the updates.
 * @returns The update, or undefined when none is under way and none stays failed.
 */
function foundRun(overview: UpdatesOverview): UpdateRun | undefined {
	const job = overview.job
	if (job === null || !pickedUp(job, overview)) {
		return undefined
	}
	return {
		to: job.to,
		from: job.from,
		retry: false,
		priorFailure: undefined,
		priorBackup: overview.backups.newest,
		moved: true,
		ended: job.step === 'failed',
	}
}

/**
 * Reports whether the screen follows a job: under way on the version it left, or checking or failed on its new one.
 * @param job - The job the overview names.
 * @param overview - The overview of the updates.
 * @returns True when the screen follows the job.
 */
function pickedUp(job: UpdateJob, overview: UpdatesOverview): boolean {
	if (job.to === overview.running) {
		return job.step === 'failed' || openStep(job, overview.refused)
	}
	return job.from === overview.running && openStep(job, overview.refused)
}

/**
 * Reports whether the screen follows an update that may still move on: under way, or not back within the wait.
 * @param view - What the screen sees of the update, if anything.
 * @returns True while the update runs or has not come back yet.
 */
export function mayMoveOn(view: RunView | undefined): boolean {
	return view !== undefined && OPEN_PHASES.includes(view.phase.kind)
}

/**
 * Reports whether a job stands at a step before its switch, or at a switch the version it went to has not refused.
 * @param job - The job the overview names.
 * @param refused - The versions the install refuses.
 * @returns True while the job is under way.
 */
function openStep(job: UpdateJob, refused: string[]): boolean {
	if (UNDER_WAY.includes(job.step)) {
		return true
	}
	return job.step === 'switched' && !refused.includes(job.to)
}

/**
 * Returns what the screen sees of the update it follows, or of one under way when it follows none.
 * @param run - The update the screen started, if any.
 * @param overview - The overview last read, if any arrived.
 * @param failing - Whether the last read of the overview failed.
 * @param waited - Whether the wait for a restart passed.
 * @returns The view, or undefined while there is no update to follow.
 */
export function runView(
	run: UpdateRun | undefined,
	overview: UpdatesOverview | undefined,
	failing: boolean,
	waited: boolean,
): RunView | undefined {
	if (overview === undefined) {
		return undefined
	}
	const followed = run ?? foundRun(overview)
	if (followed === undefined) {
		return undefined
	}
	const seen = { overview, failing }
	const current = withMove(followed, seen)
	const line = stepLine(current, seen)
	const restarting = RESTARTING.includes(line)
	return { run: followed, phase: phaseOf(current, seen, line, waited && restarting), restarting }
}

/**
 * Returns the update the screen holds after a read: the one it follows, or one under way it picks up, marked as moved.
 * @param run - The update the screen follows, if any.
 * @param overview - The overview the read left, if any.
 * @param failing - Whether the read failed.
 * @returns The update, marked as moved once a read showed it moving, or undefined while there is none to follow.
 */
export function heldRun(
	run: UpdateRun | undefined,
	overview: UpdatesOverview | undefined,
	failing: boolean,
): UpdateRun | undefined {
	if (overview === undefined) {
		return run
	}
	const followed = run ?? foundRun(overview)
	return followed === undefined ? undefined : withMove(followed, { overview, failing })
}

/**
 * Returns the update marked as moved when the overview shows it moving, the same update otherwise.
 * @param run - The update followed.
 * @param seen - The overview last read and whether the read failed.
 * @returns The update.
 */
function withMove(run: UpdateRun, seen: Seen): UpdateRun {
	return run.moved || !movedOn(run, seen) ? run : { ...run, moved: true }
}

/**
 * Reports whether the overview shows the update moving: the server gone, another version running, or its own step.
 * @param run - The update followed.
 * @param seen - The overview last read and whether the read failed.
 * @returns True once the update moved past the overview it started from.
 */
function movedOn(run: UpdateRun, seen: Seen): boolean {
	return seen.failing || advanced(run, seen.overview)
}

/**
 * Reports whether another version runs, the job of the update is under way, or a backup was taken since it started.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns True when the overview shows the update under way or past its backup.
 */
function advanced(run: UpdateRun, overview: UpdatesOverview): boolean {
	return overview.running !== run.from || progressed(run, overview)
}

/**
 * Reports whether the job of the update is under way, or the install holds a newer backup than at its start.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns True when the job runs or backed up the database.
 */
function progressed(run: UpdateRun, overview: UpdatesOverview): boolean {
	return ownJobOpen(run, overview) || overview.backups.newest !== run.priorBackup
}

/**
 * Reports whether the job of the update stands before its switch, or at a switch the install does not refuse.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns True while the job of the update is under way.
 */
function ownJobOpen(run: UpdateRun, overview: UpdatesOverview): boolean {
	const job = ownJob(run, overview)
	return job !== undefined && openStep(job, overview.refused)
}

/**
 * Returns the job of the update, when the overview names it.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns The job, or undefined when the overview names another one or none.
 */
function ownJob(run: UpdateRun, overview: UpdatesOverview): UpdateJob | undefined {
	const job = overview.job
	return job?.to === run.to && job.from === run.from ? job : undefined
}

/**
 * Returns the step the job of the update stands at, empty when the overview names another job.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns The step, or an empty string.
 */
function ownStep(run: UpdateRun, overview: UpdatesOverview): string {
	return ownJob(run, overview)?.step ?? ''
}

/**
 * Returns the line the update stands at: the download until it moved, then its step, the restart, and the check.
 * @param run - The update followed.
 * @param seen - The overview last read and whether the read failed.
 * @returns The line under way.
 */
function stepLine(run: UpdateRun, seen: Seen): StepLine {
	if (seen.failing) {
		return 'restart'
	}
	if (seen.overview.running === run.to) {
		return 'check'
	}
	return run.moved ? (LINE_OF_STEP[ownStep(run, seen.overview)] ?? 'download') : 'download'
}

/**
 * Returns where the update stands: an ending the overview shows, the end of the wait, or the line under way.
 * @param run - The update followed.
 * @param seen - The overview last read and whether the read failed.
 * @param line - The line the update stands at.
 * @param waited - Whether the wait for its restart passed.
 * @returns The phase.
 */
function phaseOf(run: UpdateRun, seen: Seen, line: StepLine, waited: boolean): RunPhase {
	const ending = seen.failing ? undefined : endingOf(run, seen.overview)
	if (ending !== undefined) {
		return { kind: ending }
	}
	return waited ? { kind: 'not-back' } : { kind: 'steps', line }
}

/**
 * Returns the ending the overview shows: the new version kept or failed, a failure before, or the one before it back.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns The ending, or undefined while the update runs.
 */
function endingOf(run: UpdateRun, overview: UpdatesOverview): Ending | undefined {
	if (overview.running === run.to) {
		return ENDING_ON_NEW[ownStep(run, overview)]
	}
	if (failedBefore(run, overview)) {
		return 'over'
	}
	return ownOutcome(run) && wentBack(run, overview) ? 'rolled-back' : undefined
}

/**
 * Reports whether the overview tells of this update and not the try before it: on a first try, or once a retry moved.
 * @param run - The update followed.
 * @returns True when a version gone back in the overview is the outcome of this update.
 */
function ownOutcome(run: UpdateRun): boolean {
	return run.moved || !run.retry
}

/**
 * Reports whether the version before the update runs again, refusing the version the update switched to.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns True once the update went back.
 */
function wentBack(run: UpdateRun, overview: UpdatesOverview): boolean {
	return overview.running === run.from && refusedPastSwitch(run, overview)
}

/**
 * Reports whether the install refuses the version the update went to, its job standing past the switch.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns True when the new version failed its start check.
 */
function refusedPastSwitch(run: UpdateRun, overview: UpdatesOverview): boolean {
	return overview.refused.includes(run.to) && PAST_SWITCH.includes(ownStep(run, overview))
}

/**
 * Reports whether the overview holds a failure of the update other than the one it held when the update started.
 * @param run - The update followed.
 * @param overview - The overview last read.
 * @returns True once the update failed before its switch.
 */
function failedBefore(run: UpdateRun, overview: UpdatesOverview): boolean {
	const failure = overview.job?.failure
	return failure?.to === run.to && failure.at !== run.priorFailure
}
