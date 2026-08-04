// SPDX-License-Identifier: Apache-2.0

import type { APIRoute } from 'astro'

import { kitFeatureVersion } from '../kit.ts'

export const prerender = false

/**
 * Reports that the theme process is ready to serve.
 * @returns The readiness the supervisor waits on.
 */
export const GET: APIRoute = () =>
	Response.json({ gophenberg: kitFeatureVersion, ready: true })
