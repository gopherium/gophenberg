// SPDX-License-Identifier: Apache-2.0

/**
 * Reports that the build carries no client side media processing.
 */
export function mediaProcessingUnavailable(): never {
	throw new Error('gophenberg: client side media processing is not part of this build')
}
