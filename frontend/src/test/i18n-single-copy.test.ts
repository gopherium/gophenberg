// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/pot.ts'
import { pinnedVersions, resolvedVersions } from '../../scripts/singleCopy.ts'

const ROOT = repositoryRoot()
const RUNTIME = '@wordpress/i18n'

test('resolves exactly one copy of the translation runtime', () => {
	expect(resolvedVersions(readFileSync(join(ROOT, 'pnpm-lock.yaml'), 'utf8'), RUNTIME)).toHaveLength(1)
})

test('pins the runtime exactly, at the one resolved version', () => {
	const resolved = resolvedVersions(readFileSync(join(ROOT, 'pnpm-lock.yaml'), 'utf8'), RUNTIME)

	for (const pinned of pinnedVersions(ROOT, RUNTIME)) {
		expect(pinned).toBe(resolved[0])
	}
})

test('pins the runtime in every package that calls it', () => {
	expect(pinnedVersions(ROOT, RUNTIME)).toHaveLength(2)
})

test('reads a lockfile whose packages run to the end', () => {
	const lockfile = "\npackages:\n  'left-pad@1.3.0':\n    resolution: {integrity: sha512-x}\n"

	expect(resolvedVersions(lockfile, 'left-pad')).toEqual(['1.3.0'])
})

test('reports no pin for a package nothing declares', () => {
	expect(pinnedVersions(ROOT, '@gophenberg/not-a-package')).toEqual([])
})
