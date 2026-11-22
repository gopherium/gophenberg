// SPDX-License-Identifier: Apache-2.0

/** The place a step's sentence holds for the plugin's own error. */
const ERROR_PLACE = '%s'

/** The part of a plugin's row its paused reason reads. */
export interface PausedRecord {
	/** The state the plugin stands in, the reason drawn only while it is paused. */
	status: string
	/** The step a paused plugin failed at, as its record names it, empty when it names none. */
	paused_step: string
	/** The plugin's own error, as it gave it. */
	paused_reason: string
}

/** What the paused reason draws from. */
export interface PausedReasonProps {
	/** The plugin's row. */
	plugin: PausedRecord
	/** Returns the sentence the app gives a step, holding one `%s` where the plugin's error goes. */
	sentenceOf: (step: string) => string
}

/**
 * Renders why a plugin paused: the sentence of its step, with its own error in a code element.
 * @param props - The plugin's row and the sentence of each step.
 * @returns One span holding the whole sentence, or nothing for a plugin that is not paused.
 */
export function PausedReason({ plugin, sentenceOf }: PausedReasonProps) {
	if (plugin.status !== 'paused') {
		return null
	}
	const [before, ...after] = sentenceOf(plugin.paused_step).split(ERROR_PLACE)
	return (
		<span>
			{before}
			<code>{plugin.paused_reason}</code>
			{after.join(ERROR_PLACE)}
		</span>
	)
}
