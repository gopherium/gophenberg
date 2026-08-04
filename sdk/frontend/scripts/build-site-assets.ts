// SPDX-License-Identifier: Apache-2.0

import { argv } from 'node:process'

import { writeSiteAssets } from './siteAssets.ts'

writeSiteAssets(argv[2])
