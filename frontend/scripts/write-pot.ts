// SPDX-License-Identifier: AGPL-3.0-or-later

import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import { pot, repositoryRoot } from './pot.ts'

const root = repositoryRoot()

mkdirSync(join(root, 'languages'), { recursive: true })
writeFileSync(join(root, 'languages', 'gophenberg.pot'), pot(root))
