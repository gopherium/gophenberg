// SPDX-License-Identifier: Apache-2.0

// A stub theme server that reports ready at once and echoes what it was given.

import { createServer } from 'node:http'

const { HOST, PORT } = process.env

createServer((request, response) => {
	if (request.url === '/_gophenberg/health') {
		response.setHeader('content-type', 'application/json')
		response.end(JSON.stringify({ gophenberg: '0.0', ready: true }))
		return
	}
	response.end(`stub served ${request.url}`)
}).listen(Number(PORT), HOST)
