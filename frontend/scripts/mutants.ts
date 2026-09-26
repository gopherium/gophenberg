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
