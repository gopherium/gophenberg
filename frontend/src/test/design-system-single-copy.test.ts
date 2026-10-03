// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/config.ts'

/** The scoped design system packages that render unstyled or lose their state when two copies load. */
const SINGLE_COPY = [
	'@gopherium/godmin',
	'@gopherium/react-auth',
	'@wordpress/a11y',
	'@wordpress/icons',
	'@wordpress/style-runtime',
	'@wordpress/theme',
	'@wordpress/ui',
]

/** Returns the snapshot keys a pnpm lockfile holds for a scoped package, one per peer combination. */
function snapshotsOf(lockfile: string, name: string): string[] {
	const held: string[] = []
	let inSnapshots = false
	for (const line of lockfile.split('\n')) {
		if (line !== '' && !line.startsWith(' ')) {
			inSnapshots = line === 'snapshots:'
		} else if (inSnapshots && line.startsWith(`  '${name}@`)) {
			held.push(line.slice(3, line.indexOf("'", 3)))
		}
	}
	return held
}

test.each(SINGLE_COPY)('resolves one snapshot of %s', (name) => {
	const lockfile = readFileSync(join(repositoryRoot(), 'pnpm-lock.yaml'), 'utf8')

	expect(snapshotsOf(lockfile, name)).toHaveLength(1)
})

test('counts each peer combination of one version as its own snapshot', () => {
	const lockfile = [
		'packages:',
		'',
		"  '@gopherium/godmin@0.14.1':",
		'    resolution: {integrity: sha512-x}',
		'',
		'snapshots:',
		'',
		"  '@gopherium/godmin@0.14.1(aaa)':",
		'    dependencies: {}',
		'',
		"  '@gopherium/godmin@0.14.1(bbb)':",
		'    dependencies: {}',
		'',
		"  '@gopherium/godmin-extra@1.0.0': {}",
		'',
	].join('\n')

	expect(snapshotsOf(lockfile, '@gopherium/godmin')).toEqual([
		'@gopherium/godmin@0.14.1(aaa)',
		'@gopherium/godmin@0.14.1(bbb)',
	])
})
