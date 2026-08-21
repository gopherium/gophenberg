// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import { pot } from '@gopherium/gottext/build'

import { DOMAIN, potConfig, repositoryRoot } from './config.ts'

const root = repositoryRoot()

mkdirSync(join(root, 'languages'), { recursive: true })
writeFileSync(join(root, 'languages', `${DOMAIN}.pot`), pot(potConfig()))
