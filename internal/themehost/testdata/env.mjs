// SPDX-License-Identifier: Apache-2.0

// A stub theme server that reports every environment variable it was given.

import { createServer } from 'node:http'

const { HOST, PORT } = process.env

createServer((request, response) => {
	response.setHeader('content-type', 'application/json')
	if (request.url === '/_gophenberg/health') {
		response.end(JSON.stringify({ gophenberg: '0.0', ready: true }))
		return
	}
	response.end(JSON.stringify(process.env))
}).listen(Number(PORT), HOST)
