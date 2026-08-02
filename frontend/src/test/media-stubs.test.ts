// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { mediaProcessingUnavailable } from '../../stubs/unavailable'
import * as videoConversion from '../../stubs/video-conversion-worker'
import * as vips from '../../stubs/vips-worker'

test('says why it cannot process media rather than failing obscurely', () => {
	expect(() => mediaProcessingUnavailable()).toThrow(
		/media processing is not part of this build/,
	)
})

test('leaves no stubbed name bound to anything but the loud failure', () => {
	const stubbed = Object.entries({ ...vips, ...videoConversion })

	expect(stubbed.length).toBeGreaterThan(0)
	for (const [name, stub] of stubbed) {
		expect(stub, name).toBe(mediaProcessingUnavailable)
	}
})
