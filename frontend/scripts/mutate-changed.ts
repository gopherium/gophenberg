// SPDX-License-Identifier: Apache-2.0

import { execFileSync } from 'node:child_process'

import { repositoryRoot } from './config.ts'
import { mutatedFiles, strykerPatterns } from './mutants.ts'

const base = process.argv[2]
if (base === undefined) {
	console.error('name the base to compare with, as in pnpm run mutate:changed origin/main')
	process.exit(1)
}

const root = repositoryRoot()
const git = (args: string[]): string[] =>
	execFileSync('git', args, { cwd: root, encoding: 'utf8' }).split('\n').filter(Boolean)
const changed = [
	...git(['diff', '--name-only', '--diff-filter=d', '--merge-base', base]),
	...git(['ls-files', '--others', '--exclude-standard']),
]
const files = mutatedFiles(changed, strykerPatterns())

if (files.length === 0) {
	console.log('no changed source file to mutate')
} else {
	execFileSync('stryker', ['run', 'frontend/stryker.config.json', '--mutate', files.join(',')], {
		cwd: root,
		stdio: 'inherit',
	})
}
