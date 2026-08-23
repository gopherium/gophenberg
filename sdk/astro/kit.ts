// SPDX-License-Identifier: Apache-2.0

import { version } from './package.json'

/** The package a theme depends on. */
export const kitName = '@gophenberg/astro'

/** The address a theme reads published content through. */
export const contentApiPath = '/api/content/v1'

/** The version this kit ships at. */
export const kitVersion = version

/**
 * Returns the major and minor of a version, the part a public page reports.
 * @param version - The full version.
 * @returns The feature version.
 */
function featureVersion(version: string): string {
	return version.split('.').slice(0, 2).join('.')
}

/** The feature version a themed page reports, the major and minor of kitVersion. */
export const kitFeatureVersion = featureVersion(kitVersion)

/** How a themed page names what served it. */
export const generator = `Gophenberg ${kitFeatureVersion}`

/**
 * Returns how a page names what served it, from the version the site reports.
 * @param version - The version the site runs.
 * @returns The generator string.
 */
export function generatorFor(version: string): string {
	return `Gophenberg ${featureVersion(version)}`
}

/** A kit version is three plain numbers and nothing else. */
const plainVersion = /^(\d+)\.(\d+)\.(\d+)$/

/**
 * Returns the numbers a kit version carries, or nothing when they are not three exact whole numbers.
 * @param declared - The version to read.
 * @returns The major, minor and patch.
 */
function parts(declared: string): [number, number, number] | undefined {
	const found = plainVersion.exec(declared)
	if (!found) {
		return undefined
	}
	const numbers = found.slice(1).map(Number)
	if (!numbers.every(Number.isSafeInteger)) {
		return undefined
	}
	return numbers as [number, number, number]
}

/**
 * Reports whether a host serving one kit version answers a theme built with another.
 * @param served - The kit version the host serves.
 * @param declared - The kit version the theme was built with.
 * @returns True when the served version answers everything the theme asks for.
 */
export function kitServes(served: string, declared: string): boolean {
	const host = parts(served)
	const theme = parts(declared)
	if (!host || !theme) {
		return false
	}
	if (host[0] !== theme[0]) {
		return false
	}
	if (host[0] === 0 && host[1] !== theme[1]) {
		return false
	}
	return host[1] > theme[1] || (host[1] === theme[1] && host[2] >= theme[2])
}

/**
 * Reports whether a host serving these kit versions answers a theme built with this kit.
 * @param served - The kit versions the host serves.
 * @returns True when one of them answers this kit.
 */
export function servedBy(served: string[]): boolean {
	return served.some((candidate) => kitServes(candidate, kitVersion))
}

/** A block name is a lowercase slug, optionally namespaced by a vendor. */
const blockName = /^[a-z][a-z0-9-]*(\/[a-z][a-z0-9-]*)?$/

/**
 * Reports whether a value is a block name the editor serializes.
 * @param value - The candidate name.
 * @returns True when the editor could have written it.
 */
export function isBlockName(value: string): boolean {
	return blockName.test(value)
}
