// SPDX-License-Identifier: AGPL-3.0-or-later

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

/** The packages that pin the translation runtime for themselves. */
const PINNING = ['frontend', 'sdk/frontend']

/**
 * Returns every version of the package the lockfile resolves.
 * @param lockfile - The pnpm lockfile as text.
 * @param name - The package to look for.
 * @returns The resolved versions, sorted.
 */
export function resolvedVersions(lockfile: string, name: string): string[] {
	const start = lockfile.indexOf('\npackages:\n')
	const end = lockfile.indexOf('\nsnapshots:\n')
	const packages = lockfile.slice(start, end === -1 ? undefined : end)
	const found = new Set<string>()
	for (const line of packages.split('\n')) {
		const match = /^ {2}'?(.+)@([^@']+)'?:$/.exec(line)
		if (match && match[1] === name) {
			found.add(match[2])
		}
	}
	return [...found].sort()
}

/**
 * Returns the version each package pins the runtime at.
 * @param root - The repository root the packages sit under.
 * @param name - The package to look for.
 * @returns The pinned versions, one per package that declares it.
 */
export function pinnedVersions(root: string, name: string): string[] {
	const pinned: string[] = []
	for (const held of PINNING) {
		const manifest = JSON.parse(readFileSync(join(root, held, 'package.json'), 'utf8')) as {
			dependencies?: Record<string, string>
		}
		const version = manifest.dependencies?.[name]
		if (version !== undefined) {
			pinned.push(version)
		}
	}
	return pinned
}
