// SPDX-License-Identifier: Apache-2.0

import { type ChildProcess, spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { type Server, createServer } from 'node:http'
import type { AddressInfo } from 'node:net'
import { join } from 'node:path'

import { kitVersion } from '@gophenberg/astro'
import { afterAll, beforeAll, describe, expect, test } from 'vitest'

/** Where the content API lives under the instance, as the kit reads it. */
const contentApiPath = '/api/content/v1'

/** Where the build leaves the artifact a theme directory ships. */
const artifact = new URL('../dist/', import.meta.url).pathname

const built = existsSync(join(artifact, 'server', 'entry.mjs'))

/** The content API double, answering the handshake and failing every content read. */
let api: Server

/** The addresses the content API double was asked for. */
const reads: string[] = []

/** The theme process the artifact runs as. */
let theme: ChildProcess

/** The origin the theme process answers on. */
let origin: string

/**
 * Returns a loopback port no one is listening on.
 * @returns The port.
 */
async function freePort(): Promise<number> {
	const probe = createServer()
	await new Promise<void>((resolve) => probe.listen(0, '127.0.0.1', resolve))
	const { port } = probe.address() as AddressInfo
	await new Promise((resolve) => probe.close(resolve))
	return port
}

/**
 * Waits until the theme process answers its readiness probe at all.
 * @param address - The origin the theme answers on.
 */
async function answering(address: string): Promise<void> {
	for (let attempt = 0; attempt < 50; attempt++) {
		try {
			await fetch(`${address}/_gophenberg/health`, { signal: AbortSignal.timeout(1000) })
			return
		} catch {
			await new Promise((resolve) => setTimeout(resolve, 100))
		}
	}
	throw new Error(`the theme never answered on ${address}`)
}

beforeAll(async () => {
	if (!built) {
		return
	}
	api = createServer((request, response) => {
		reads.push(request.url ?? '')
		response.setHeader('Content-Type', 'application/json')
		if (request.url === contentApiPath) {
			response.end(JSON.stringify({ gophenberg: '0.20.0', api: 1, kit: [kitVersion] }))
			return
		}
		response.statusCode = 500
		response.end(JSON.stringify({ error: 'down' }))
	})
	await new Promise<void>((resolve) => api.listen(0, '127.0.0.1', resolve))
	const port = await freePort()
	origin = `http://127.0.0.1:${port}`
	theme = spawn(process.execPath, [join(artifact, 'server', 'entry.mjs')], {
		env: {
			...process.env,
			HOST: '127.0.0.1',
			PORT: String(port),
			GOPHENBERG_API_URL: `http://127.0.0.1:${(api.address() as AddressInfo).port}`,
		},
		stdio: 'ignore',
	})
	// The health probe also answers the handshake the kit remembers, as the supervisor's probe does in production,
	// so a failing page afterwards reads the content API for nothing but its own content.
	await answering(origin)
}, 30_000)

afterAll(async () => {
	if (theme && theme.exitCode === null && theme.signalCode === null) {
		const exited = new Promise((resolve) => theme.once('exit', resolve))
		theme.kill()
		await exited
	}
	await new Promise((resolve) => (api ? api.close(resolve) : resolve(undefined)))
})

describe.skipIf(!built)('the artifact while the content API fails', () => {
	test('calls the content API once for a request that fails', async () => {
		reads.length = 0

		const response = await fetch(`${origin}/hello-world`)

		expect(response.status).toBe(500)
		expect(reads).toEqual([`${contentApiPath}/resolve?path=%2Fhello-world&per_page=2`])
	})

	test('answers the failure with the error page the kit ships', async () => {
		const response = await fetch(`${origin}/hello-world`)

		expect(await response.text()).toContain('Something went wrong')
	})
})
