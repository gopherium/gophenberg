// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { mutateTargets, mutatedFiles, parseShard, shardTargets, strykerPatterns } from '../../scripts/mutants.ts'

const PATTERNS = ['frontend/src/**/*.{ts,tsx}', '!**/*.test.{ts,tsx}', '!frontend/src/main.tsx']

const RANGES = [
	'frontend/src/a.ts:12-13',
	'frontend/src/b.ts:41-50',
	'frontend/src/c.ts:30-30',
	'frontend/src/d.ts:3-9',
	'frontend/src/e.ts:7-9',
	'frontend/src/f.ts:100-104',
]

/**
 * Returns the share of every shard of a run split into the given number of shards.
 * @param targets - The mutate entries to split.
 * @param count - How many shards the run has.
 * @param fileLines - Counts the lines of a whole file.
 * @returns The share of each shard, the first shard first.
 */
function sharesOf(targets: string[], count: number, fileLines: (file: string) => number = () => 0): string[][] {
	return Array.from({ length: count }, (_, place) => shardTargets(targets, { index: place + 1, count }, fileLines))
}

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

test('reads a shard named as its index over the shard count', () => {
	expect(parseShard('2/6')).toEqual({ index: 2, count: 6 })
})

test('reads a shard index and count of several digits', () => {
	expect(parseShard('12/16')).toEqual({ index: 12, count: 16 })
})

test('reads the only shard of a run in one piece', () => {
	expect(parseShard('1/1')).toEqual({ index: 1, count: 1 })
})

test('reads the last shard of a run', () => {
	expect(parseShard('6/6')).toEqual({ index: 6, count: 6 })
})

test.each(['0/6', '7/6', '1/0', '0/0'])('reads no shard from %s, its index outside the count', (text) => {
	expect(parseShard(text)).toBeUndefined()
})

test.each(['', '2', '2/', '/6', 'a/6', 'x2/6', '2/6x', '2/6/1', '-1/6', '1.5/6', ' 2/6'])(
	'reads no shard from %j, not written as index over count',
	(text) => {
		expect(parseShard(text)).toBeUndefined()
	},
)

test.each([1, 2, 3, 4, 6, 8])('gives every target to exactly one shard of a run in %i shards', (count) => {
	expect(sharesOf(RANGES, count).flat().toSorted()).toEqual(RANGES)
})

test('spreads the changed lines evenly over two shards', () => {
	expect(sharesOf(RANGES, 2)).toEqual([
		['frontend/src/b.ts:41-50', 'frontend/src/c.ts:30-30', 'frontend/src/e.ts:7-9'],
		['frontend/src/a.ts:12-13', 'frontend/src/d.ts:3-9', 'frontend/src/f.ts:100-104'],
	])
})

test('spreads the changed lines evenly over three shards', () => {
	expect(sharesOf(RANGES, 3)).toEqual([
		['frontend/src/b.ts:41-50'],
		['frontend/src/a.ts:12-13', 'frontend/src/d.ts:3-9'],
		['frontend/src/c.ts:30-30', 'frontend/src/e.ts:7-9', 'frontend/src/f.ts:100-104'],
	])
})

test('counts both the first and the last line of a range', () => {
	const targets = ['frontend/src/a.ts:1-4', 'frontend/src/b.ts:1-2', 'frontend/src/c.ts:5-6', 'frontend/src/d.ts:9-9']

	expect(sharesOf(targets, 2)).toEqual([
		['frontend/src/a.ts:1-4', 'frontend/src/d.ts:9-9'],
		['frontend/src/b.ts:1-2', 'frontend/src/c.ts:5-6'],
	])
})

test('counts a whole file by the lines it holds', () => {
	const targets = ['frontend/src/a.ts:1-3', 'frontend/src/b.ts:1-3', 'frontend/src/new.tsx']
	const fileLines = (file: string) => (file === 'frontend/src/new.tsx' ? 6 : 0)

	expect(sharesOf(targets, 2, fileLines)).toEqual([
		['frontend/src/new.tsx'],
		['frontend/src/a.ts:1-3', 'frontend/src/b.ts:1-3'],
	])
})

test('counts a whole file named like a range by the lines it holds', () => {
	const targets = ['frontend/src/a.ts:1-3', 'frontend/src/b.ts:1-3', 'frontend/src/c:1-2.ts']
	const fileLines = (file: string) => (file === 'frontend/src/c:1-2.ts' ? 6 : 0)

	expect(sharesOf(targets, 2, fileLines)).toEqual([
		['frontend/src/c:1-2.ts'],
		['frontend/src/a.ts:1-3', 'frontend/src/b.ts:1-3'],
	])
})

test('puts targets of the same size in name order', () => {
	expect(sharesOf(['frontend/src/b.ts:1-1', 'frontend/src/a.ts:1-1'], 2)).toEqual([
		['frontend/src/a.ts:1-1'],
		['frontend/src/b.ts:1-1'],
	])
})

test('keeps the given order of the targets in a share', () => {
	expect(sharesOf(RANGES.toReversed(), 2)[0]).toEqual([
		'frontend/src/e.ts:7-9',
		'frontend/src/c.ts:30-30',
		'frontend/src/b.ts:41-50',
	])
})

test('gives the same shares whatever order the targets come in', () => {
	const shares = sharesOf(RANGES.toReversed(), 3).map((share) => share.toSorted())

	expect(shares).toEqual(sharesOf(RANGES, 3))
})

test('leaves a shard empty when the run has fewer targets than shards', () => {
	expect(sharesOf(['frontend/src/a.ts:1-1', 'frontend/src/b.ts:1-1'], 3)).toEqual([
		['frontend/src/a.ts:1-1'],
		['frontend/src/b.ts:1-1'],
		[],
	])
})
