// SPDX-License-Identifier: Apache-2.0

// A stub theme server that swallows SIGTERM, so only a group kill ends it.

import { createServer } from 'node:http'

const { HOST, PORT } = process.env

process.on('SIGTERM', () => {
	process.stderr.write('stub ignoring SIGTERM\n')
})

createServer((request, response) => {
	response.setHeader('content-type', 'application/json')
	response.end(JSON.stringify({ gophenberg: '0.0', ready: true }))
}).listen(Number(PORT), HOST)
