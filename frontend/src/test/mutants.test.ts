// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { mutateTargets, mutatedFiles, strykerPatterns } from '../../scripts/mutants.ts'

const PATTERNS = ['frontend/src/**/*.{ts,tsx}', '!**/*.test.{ts,tsx}', '!frontend/src/main.tsx']

/**
 * Returns a diff with no context lines touching one file with the given hunk headers.
 * @param file - The file the diff changes.
 * @param hunks - The hunk headers, as git writes them.
 * @returns The diff text.
 */
function diffOf(file: string, ...hunks: string[]): string {
	const body = hunks.flatMap((hunk) => [hunk, '+changed'])
	return [`diff --git a/${file} b/${file}`, `--- a/${file}`, `+++ b/${file}`, ...body].join('\n')
}

test('names the lines a hunk adds as a range', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -10,0 +11,3 @@ export function Screen() {')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:11-13'])
})

test('names a single changed line as a range of one', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -5 +5 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:5-5'])
})

test('skips a hunk that only removes lines', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -5,2 +4,0 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([])
})

test('keeps every hunk of a file as its own range', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -2 +2 @@', '@@ -20,0 +21,2 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([
		'frontend/src/users/screen.tsx:2-2',
		'frontend/src/users/screen.tsx:21-22',
	])
})

test('mutates a file git does not track yet whole', () => {
	expect(mutateTargets('', ['frontend/src/users/new.tsx', 'notes.md'], PATTERNS)).toEqual([
		'frontend/src/users/new.tsx',
	])
})

test('leaves out the changed lines of a file the patterns do not name', () => {
	const diff = [
		diffOf('frontend/src/test/users.test.tsx', '@@ -3 +3 @@'),
		diffOf('frontend/src/users/screen.tsx', '@@ -7 +7 @@'),
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:7-7'])
})

test('reads an added line starting with plus signs as content, not as a file header', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -1,0 +1,2 @@'),
		'+++ b/frontend/src/users/other.tsx',
		'@@ -9 +10 @@',
		'+later',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([
		'frontend/src/users/screen.tsx:1-2',
		'frontend/src/users/screen.tsx:10-10',
	])
})

test('reads a removed line starting with dashes as content, not as a file header', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -1 +0,0 @@'),
		'--- a removed line of dashes',
		'@@ -3 +2 @@',
		'+changed',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:2-2'])
})

test('reads a removed line and an added line shaped like file headers as content', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -1 +1,2 @@'),
		'--- a/frontend/src/users/other.tsx',
		'+++ b/frontend/src/users/other.tsx',
		'@@ -9 +10 @@',
		'+later',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual([
		'frontend/src/users/screen.tsx:1-2',
		'frontend/src/users/screen.tsx:10-10',
	])
})

test('reads a hunk header with counts of several digits', () => {
	const diff = diffOf('frontend/src/users/screen.tsx', '@@ -40,12 +40,15 @@')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:40-54'])
})

test('reads an added line holding hunk header text as content', () => {
	const diff = [diffOf('frontend/src/users/screen.tsx', '@@ -1 +1 @@'), "+const hunk = '@@ -9 +10 @@'"].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:1-1'])
})

test('reads a file name with a space without the tab git ends it with', () => {
	const diff = [
		'diff --git a/frontend/src/users/my screen.tsx b/frontend/src/users/my screen.tsx',
		'--- a/frontend/src/users/my screen.tsx\t',
		'+++ b/frontend/src/users/my screen.tsx\t',
		'@@ -5 +5 @@',
		'+changed',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/my screen.tsx:5-5'])
})

test('gives no lines of a file whose name git quotes to the file before it', () => {
	const diff = [
		diffOf('frontend/src/users/screen.tsx', '@@ -2 +2 @@'),
		'diff --git "a/frontend/src/users/tab\\tname.tsx" "b/frontend/src/users/tab\\tname.tsx"',
		'--- "a/frontend/src/users/tab\\tname.tsx"',
		'+++ "b/frontend/src/users/tab\\tname.tsx"',
		'@@ -7 +7 @@',
		'+changed',
	].join('\n')

	expect(mutateTargets(diff, [], PATTERNS)).toEqual(['frontend/src/users/screen.tsx:2-2'])
})

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
