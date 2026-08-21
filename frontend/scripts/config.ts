// SPDX-License-Identifier: Apache-2.0

import { join } from 'node:path'

import type { PotOptions } from '@gopherium/gottext/build'

/** The text domain every Gophenberg owned string names. */
export const DOMAIN = 'gophenberg'

/** The globs holding every string the admin ships, read from the repository root. */
export const SOURCES = ['frontend/src/**/*.{ts,tsx}', 'sdk/frontend/**/*.{ts,tsx}']

/** The directories inside those globs that ship no string. */
export const IGNORED = ['**/test/**', '**/stubs/**', '**/scripts/**']

/** The directories the Go side's messages live under. */
export const GO_ROOTS = ['internal', 'cmd', 'plugins']

/**
 * Returns the repository root the source globs resolve against.
 * @returns The absolute path of the repository root.
 */
export function repositoryRoot(): string {
	return join(import.meta.dirname, '..', '..')
}

/**
 * Returns the extraction options the catalogue template builds from.
 * @param sources - The globs to read, the admin sources by default.
 * @returns The options naming the domain, the roots and the globs.
 */
export function potConfig(sources: string[] = SOURCES): PotOptions {
	return {
		domain: DOMAIN,
		root: repositoryRoot(),
		sources,
		ignored: IGNORED,
		goRoots: GO_ROOTS,
	}
}

/**
 * Returns every directory a compiled catalogue is shipped from.
 * @param root - The repository root the directories sit under.
 * @returns The directories, the reader's and the server's.
 */
export function catalogTargets(root: string): string[] {
	return [join(root, 'frontend', 'src', 'languages'), join(root, 'internal', 'i18n', 'catalogs')]
}
