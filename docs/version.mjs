// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { visit } from 'unist-util-visit'

const versionFile = fileURLToPath(new URL('../internal/version/VERSION', import.meta.url))
const kitFile = fileURLToPath(new URL('../sdk/astro/package.json', import.meta.url))

/** The release the docs describe. */
export const version = readFileSync(versionFile, 'utf8').trim()

/** The major and minor the product reports in its public headers. */
export const featureVersion = version.split('.').slice(0, 2).join('.')

/** The theme kit release the docs describe, versioned apart from the product. */
export const kitVersion = JSON.parse(readFileSync(kitFile, 'utf8')).version

const tokens = {
	'%VERSION%': version,
	'%FEATURE_VERSION%': featureVersion,
	'%KIT_VERSION%': kitVersion,
}
const tokenPattern = /%VERSION%|%FEATURE_VERSION%|%KIT_VERSION%/g
const escaped = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
const literalPattern = new RegExp(
	`\\b${escaped(version)}\\b|\\b${escaped(kitVersion)}\\b|Gophenberg ${escaped(featureVersion)}\\b`,
)

/**
 * Returns the text with every version token replaced.
 * @param text - The source text to substitute.
 * @returns The text carrying the release version.
 */
export function substitute(text) {
	return text.replace(tokenPattern, (token) => tokens[token])
}

/**
 * Reports whether the text pins the release version a bump would leave stale.
 * @param text - The source text to inspect.
 * @returns True when a literal is present.
 */
export function pinsALiteral(text) {
	return literalPattern.test(text)
}

/**
 * Returns a remark plugin substituting version tokens and refusing literals.
 * @returns The remark plugin.
 */
export function remarkVersion() {
	return (tree, file) => {
		visit(tree, ['text', 'code', 'inlineCode', 'yaml', 'html'], (node) => {
			if (pinsALiteral(node.value)) {
				throw new Error(
					`${file.path}: pins a version literal, use %VERSION%, %FEATURE_VERSION% or %KIT_VERSION%\n  ${node.value.slice(0, 120)}`,
				)
			}
			node.value = substitute(node.value)
		})
	}
}
