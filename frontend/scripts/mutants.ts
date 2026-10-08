// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join, matchesGlob } from 'node:path'

import { repositoryRoot } from './config.ts'

/**
 * Returns the changed files a mutation run over the patterns would mutate.
 * @param changed - The changed files, relative to the repository root.
 * @param patterns - The mutate patterns in order, a leading ! leaving files out.
 * @returns The changed files the patterns name, in their changed order.
 */
export function mutatedFiles(changed: string[], patterns: string[]): string[] {
	return changed.filter((file) =>
		patterns.reduce(
			(kept, pattern) =>
				pattern.startsWith('!') ? kept && !matchesGlob(file, pattern.slice(1)) : kept || matchesGlob(file, pattern),
			false,
		),
	)
}

/** A hunk header, capturing where its lines start on the new side and how many there are. */
const HUNK = /^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@/

/**
 * Returns the mutate entries for the lines a change touched, as file:start-end, and each new file whole.
 * @param diff - The output of git diff --unified=0 with a/ and b/ prefixes.
 * @param untracked - The files git does not track yet, relative to the repository root.
 * @param patterns - The mutate patterns in order, a leading ! leaving files out.
 * @returns The entries the patterns name, in the order the change lists them.
 */
export function mutateTargets(diff: string, untracked: string[], patterns: string[]): string[] {
	const tracked = [...changedLines(diff)]
		.filter(([file]) => mutatedFiles([file], patterns).length > 0)
		.flatMap(([file, ranges]) => ranges.map((range) => `${file}:${range}`))
	return [...tracked, ...mutatedFiles(untracked, patterns)]
}

/** A shard argument, capturing its index and its count. */
const SHARD = /^(\d+)\/(\d+)$/

/** A mutate entry naming a range, capturing its first and last line. */
const RANGE = /:(\d+)-(\d+)$/

/** One shard of a mutation run, as its index counted from 1 and how many shards the run has. */
interface Shard {
	index: number
	count: number
}

/**
 * Returns the shard an argument names as index/count, or undefined when it names none.
 * @param text - The argument, as in 2/6.
 * @returns The shard, or undefined when the text is not an index from 1 to the count.
 */
export function parseShard(text: string): Shard | undefined {
	const parts = SHARD.exec(text)
	if (parts === null) {
		return undefined
	}
	const shard = { index: Number(parts[1]), count: Number(parts[2]) }
	return shard.index >= 1 && shard.index <= shard.count ? shard : undefined
}

/**
 * Returns the mutate entries one shard runs, every shard holding about the same number of lines.
 * @param targets - The mutate entries, as file:start-end or a whole file.
 * @param shard - The shard to return the entries of.
 * @param fileLines - Counts the lines of a whole file.
 * @returns The entries of the shard, in their given order.
 */
export function shardTargets(targets: string[], shard: Shard, fileLines: (file: string) => number): string[] {
	const loads = Array.from({ length: shard.count }, () => 0)
	const owners = new Map<string, number>()
	const sized = targets.toSorted().map((target) => ({ target, lines: targetLines(target, fileLines) }))
	for (const { target, lines } of sized.toSorted((left, right) => right.lines - left.lines)) {
		const lightest = loads.reduce((least, load, place) => (load < loads[least] ? place : least), 0)
		loads[lightest] += lines
		owners.set(target, lightest + 1)
	}
	return targets.filter((target) => owners.get(target) === shard.index)
}

/**
 * Returns how many lines a mutate entry covers.
 * @param target - The mutate entry, as file:start-end or a whole file.
 * @param fileLines - Counts the lines of a whole file.
 * @returns The lines of its range, or of the whole file.
 */
function targetLines(target: string, fileLines: (file: string) => number): number {
	const range = RANGE.exec(target)
	return range === null ? fileLines(target) : Number(range[2]) - Number(range[1]) + 1
}

/**
 * Returns the line ranges a diff with no context adds or changes, by file.
 * @param diff - The output of git diff --unified=0 with a/ and b/ prefixes.
 * @returns Each file with its changed ranges, as start-end.
 */
function changedLines(diff: string): Map<string, string[]> {
	const ranges = new Map<string, string[]>()
	let file = ''
	let header = false
	for (const line of diff.split('\n')) {
		const hunk = HUNK.exec(line)
		if (line.startsWith('diff --git ')) {
			file = ''
			header = true
		} else if (header && line.startsWith('+++ b/')) {
			file = line.slice('+++ b/'.length).replace(/\t$/, '')
		} else if (hunk !== null) {
			header = false
			addRange(ranges, file, Number(hunk[1]), Number(hunk[2] ?? '1'))
		}
	}
	return ranges
}

/**
 * Records the lines a hunk covers on the new side, unless it covers none.
 * @param ranges - The ranges found so far, by file.
 * @param file - The file the hunk changes.
 * @param start - The first line the hunk covers.
 * @param count - How many lines it covers.
 */
function addRange(ranges: Map<string, string[]>, file: string, start: number, count: number) {
	if (count > 0) {
		ranges.set(file, [...(ranges.get(file) ?? []), `${start}-${start + count - 1}`])
	}
}

/**
 * Returns the mutate patterns the Stryker config holds.
 * @returns The patterns, in the order the config lists them.
 */
export function strykerPatterns(): string[] {
	const config = JSON.parse(readFileSync(join(repositoryRoot(), 'frontend', 'stryker.config.json'), 'utf8')) as {
		mutate: string[]
	}
	return config.mutate
}
