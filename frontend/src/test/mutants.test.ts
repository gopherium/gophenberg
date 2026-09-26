// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { mutatedFiles, strykerPatterns } from '../../scripts/mutants.ts'

const PATTERNS = ['frontend/src/**/*.{ts,tsx}', '!**/*.test.{ts,tsx}', '!frontend/src/main.tsx']

test('keeps a changed source file a pattern names', () => {
	expect(mutatedFiles(['frontend/src/users/screen.tsx'], PATTERNS)).toEqual(['frontend/src/users/screen.tsx'])
})

test('drops a changed file no pattern names', () => {
	expect(mutatedFiles(['frontend/vite.config.ts', 'README.md'], PATTERNS)).toEqual([])
})

test('drops a changed file a later negated pattern leaves out', () => {
	expect(mutatedFiles(['frontend/src/test/users.test.tsx', 'frontend/src/main.tsx'], PATTERNS)).toEqual([])
})

test('keeps a file a pattern names again after a negated one', () => {
	const patterns = [...PATTERNS, 'frontend/src/main.tsx']

	expect(mutatedFiles(['frontend/src/main.tsx'], patterns)).toEqual(['frontend/src/main.tsx'])
})

test('keeps the changed order of the files it keeps', () => {
	const changed = ['frontend/src/b.ts', 'frontend/vite.config.ts', 'frontend/src/a.ts']

	expect(mutatedFiles(changed, PATTERNS)).toEqual(['frontend/src/b.ts', 'frontend/src/a.ts'])
})

test('reads the patterns the mutation config holds', () => {
	const changed = ['frontend/src/users/screen.tsx', 'frontend/src/test/users.test.tsx', 'frontend/src/i18n/catalog.ts']

	expect(mutatedFiles(changed, strykerPatterns())).toEqual(['frontend/src/users/screen.tsx'])
})
