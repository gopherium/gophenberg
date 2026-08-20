// SPDX-License-Identifier: Apache-2.0

import {
	answeredForms,
	keepingAnswers,
	localeFor,
	meaningfulChange,
	namedByTemplate,
	translated,
	withPluralRuleOf,
} from './poeditor.ts'
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
	kept: string[]
}

/**
 * Returns the catalogue a sync writes and how many committed answers it restored.
 * @param current - The catalogue as committed, or nothing when none is committed yet.
 * @param exported - The catalogue the platform exported, trimmed to the template.
 * @param template - The catalogue template naming every message the site shows.
 * @returns The catalogue to write and the count of answers the export had lost.
 */
function merged(
	current: string | undefined,
	exported: string,
	template: string,
): { incoming: string, restored: number } {
	if (current === undefined) {
		return { incoming: exported, restored: 0 }
	}
	const clamped = withPluralRuleOf(current, exported)
	const incoming = keepingAnswers(current, clamped, template)
	return { incoming, restored: answeredForms(incoming) - answeredForms(clamped) }
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
	const kept: string[] = []
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
		const { incoming, restored } = merged(current, exported, template)
		if (restored > 0) {
			kept.push(`${locale}, keeping ${restored} answered where the platform holds nothing`)
		}
		if (!meaningfulChange(current, incoming)) {
			continue
		}
		held.write(locale, incoming)
		moved.push(locale)
	}
	return { moved, skipped, kept }
}
