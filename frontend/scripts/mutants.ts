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
