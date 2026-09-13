// SPDX-License-Identifier: Apache-2.0

// A stub theme server that marks each connection it accepts and never answers on it.

import { writeFileSync } from 'node:fs'
import { createServer } from 'node:net'
import { join } from 'node:path'

const { HOST, PORT } = process.env

createServer(() => {
	writeFileSync(join(import.meta.dirname, 'accepted'), '')
}).listen(Number(PORT), HOST)
