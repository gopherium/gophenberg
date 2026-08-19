// SPDX-License-Identifier: Apache-2.0

import { localeFor, meaningfulChange, namedByTemplate, translated, withPluralRuleOf } from './poeditor.ts'
import type { Poeditor } from './poeditor.ts'

/** Where the catalogues a sync reads and writes live. */
export interface Catalogues {
	read: (locale: string) => string | undefined
	write: (locale: string, source: string) => void
}

/** What one sync did, in words fit for a log. */
export interface Synced {
	moved: string[]
	skipped: string[]
}

/**
 * Carries every translation the platform holds for a language the site answers in.
 * @param platform - The translation platform to read.
 * @param supported - The languages the site answers in.
 * @param held - Where the catalogues live.
 * @param template - The catalogue template naming every message the site shows.
 * @returns The languages that moved and the ones passed over, each with its reason.
 */
export async function syncTranslations(
	platform: Poeditor,
	supported: string[],
	held: Catalogues,
	template: string,
): Promise<Synced> {
	const moved: string[] = []
	const skipped: string[] = []
	await platform.uploadTerms(template)
	for (const named of await platform.languages()) {
		const locale = localeFor(named, supported)
		if (locale === undefined) {
			skipped.push(`${named}, which the site does not answer in`)
			continue
		}
		const exported = namedByTemplate(await platform.exportPo(named), template)
		if (translated(exported) === 0) {
			skipped.push(`${named}, which nobody has translated yet`)
			continue
		}
		const current = held.read(locale)
		const incoming = current === undefined ? exported : withPluralRuleOf(current, exported)
		if (!meaningfulChange(current, incoming)) {
			continue
		}
		held.write(locale, incoming)
		moved.push(locale)
	}
	return { moved, skipped }
}
