// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { poeditorAt } from './poeditor.ts'
import { DOMAIN, repositoryRoot } from './pot.ts'

const token = process.env.POEDITOR_API_TOKEN
const project = process.env.POEDITOR_PROJECT_ID

if (token === undefined || project === undefined) {
	console.error('set POEDITOR_API_TOKEN and POEDITOR_PROJECT_ID to retire terms')
	process.exit(1)
}

const template = readFileSync(join(repositoryRoot(), 'languages', `${DOMAIN}.pot`), 'utf8')
const deleted = await poeditorAt(token, project).retireTerms(template)

console.log(deleted === 0 ? 'the platform held nothing to retire' : `terms retired: ${deleted}`)
