// SPDX-License-Identifier: Apache-2.0

import type { APIRoute } from 'astro'

import { kitVersion, servedBy } from '../kit.ts'
import { siteProfile } from '../site.ts'

export const prerender = false

/**
 * Reports whether the theme process is serving the site it was built for.
 * @returns The readiness the supervisor waits on.
 */
export const GET: APIRoute = async () => {
	const site = await siteProfile()
	if (!site) {
		return Response.json(
			{ ready: false, reason: 'the site did not answer the handshake this theme reads it through' },
			{ status: 503 },
		)
	}
	if (!servedBy(site.kit)) {
		return Response.json(
			{
				ready: false,
				kit: kitVersion,
				served: site.kit,
				reason: 'the site serves no kit this theme was built with, so rebuild it',
			},
			{ status: 503 },
		)
	}
	return Response.json({ gophenberg: site.gophenberg, ready: true })
}
