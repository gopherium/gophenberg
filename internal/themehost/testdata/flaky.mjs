// SPDX-License-Identifier: Apache-2.0

// A stub theme server that serves for a moment and then dies, so a restart is observable.

import { createServer } from 'node:http'

const { HOST, PORT } = process.env

createServer((request, response) => {
	response.setHeader('content-type', 'application/json')
	response.end(JSON.stringify({ gophenberg: '0.0', ready: true }))
}).listen(Number(PORT), HOST)

setTimeout(() => {
	process.stderr.write('stub dying after serving\n')
	process.exit(1)
}, 150)
