// SPDX-License-Identifier: Apache-2.0

import { build } from 'vite'
import { expect, test } from 'vitest'

const CHUNK_CEILING = 4_000_000

type Emitted =
	| { type: 'chunk', fileName: string, code: string }
	| { type: 'asset', fileName: string, source: string | Uint8Array }

/**
 * Returns the bytes an emitted chunk or asset weighs.
 * @param emitted - The chunk or asset the build produced.
 * @returns The size in bytes.
 */
function weightOf(emitted: Emitted): number {
	return emitted.type === 'chunk' ? emitted.code.length : emitted.source.length
}

test('ships no chunk heavy enough to weigh a deployment down', async () => {
	const built = (await build({
		logLevel: 'silent',
		build: { write: false },
	})) as unknown as { output: Emitted[] }

	const heavy = built.output
		.filter((emitted) => weightOf(emitted) > CHUNK_CEILING)
		.map((emitted) => `${emitted.fileName} (${Math.round(weightOf(emitted) / 1000)} kB)`)

	expect(heavy).toEqual([])
}, 240000)
