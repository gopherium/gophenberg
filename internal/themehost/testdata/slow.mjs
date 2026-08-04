// SPDX-License-Identifier: Apache-2.0

// A stub theme server that takes a moment before it reports ready.

import { createServer } from 'node:http'

const { HOST, PORT, GOPHENBERG_STUB_DELAY_MS } = process.env

setTimeout(() => {
	createServer((request, response) => {
		if (request.url === '/_gophenberg/health') {
			response.setHeader('content-type', 'application/json')
			response.end(JSON.stringify({ gophenberg: '0.0', ready: true }))
			return
		}
		response.end('stub served late')
	}).listen(Number(PORT), HOST)
}, Number(GOPHENBERG_STUB_DELAY_MS ?? 300))
