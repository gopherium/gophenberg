// SPDX-License-Identifier: Apache-2.0

/**
 * Returns what a dialog answers for a failed write, or hands the failure back once its time limit ran out.
 * @param signal - The signal the dialog handed its write.
 * @param failure - What the write failed with.
 * @param answer - What the dialog answers for a failure before the time limit.
 * @returns The answer, or a rejection with the failure once the time limit ran out, so the dialog says no answer came.
 */
export function unlessTimedOut<T>(signal: AbortSignal, failure: unknown, answer: () => T): T | Promise<never> {
	if (signal.aborted) {
		return Promise.reject(failure)
	}
	return answer()
}
