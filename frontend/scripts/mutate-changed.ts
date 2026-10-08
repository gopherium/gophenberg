// SPDX-License-Identifier: Apache-2.0

import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { repositoryRoot } from './config.ts'
import { mutateTargets, parseShard, shardTargets, strykerPatterns } from './mutants.ts'

const [base, shardArgument = '1/1'] = process.argv.slice(2)
if (base === undefined) {
	console.error('name the base to compare with, as in pnpm run mutate:changed origin/main')
	process.exit(1)
}

const shard = parseShard(shardArgument)
if (shard === undefined) {
	console.error('name the shard as an index up to the count, as in pnpm run mutate:changed origin/main 2/6')
	process.exit(1)
}

const root = repositoryRoot()

/**
 * Returns what git prints for the arguments, run at the repository root.
 * @param args - The git arguments.
 * @returns The output, however long it is.
 */
const git = (args: string[]): string => execFileSync('git', args, { cwd: root, encoding: 'utf8', maxBuffer: Infinity })

/**
 * Returns how many lines a file holds.
 * @param file - The file, relative to the repository root.
 * @returns Its line count.
 */
const fileLines = (file: string): number => readFileSync(join(root, file), 'utf8').split('\n').length
const roots = ['frontend/src', 'frontend/scripts', 'sdk/frontend', 'plugins']
const diff = git([
	'-c',
	'core.quotePath=false',
	'diff',
	'--unified=0',
	'--inter-hunk-context=0',
	'--no-color',
	'--no-ext-diff',
	'--src-prefix=a/',
	'--dst-prefix=b/',
	'--diff-filter=d',
	'--merge-base',
	base,
	'--',
	...roots,
])
const untracked = git(['ls-files', '-z', '--others', '--exclude-standard', '--', ...roots]).split('\0').filter(Boolean)
const targets = shardTargets(mutateTargets(diff, untracked, strykerPatterns()), shard, fileLines)

if (targets.length === 0) {
	console.log('no changed source line to mutate')
} else {
	execFileSync('stryker', ['run', 'frontend/stryker.config.json', '--ignoreStatic', '--mutate', targets.join(',')], {
		cwd: root,
		stdio: 'inherit',
	})
}
