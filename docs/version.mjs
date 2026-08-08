// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { visit } from 'unist-util-visit'

const versionFile = fileURLToPath(new URL('../internal/version/VERSION', import.meta.url))

/** The release the docs describe. */
export const version = readFileSync(versionFile, 'utf8').trim()

/** The major and minor the product reports in its public headers. */
export const featureVersion = version.split('.').slice(0, 2).join('.')

const tokens = { '%VERSION%': version, '%FEATURE_VERSION%': featureVersion }
const tokenPattern = /%VERSION%|%FEATURE_VERSION%/g
const escaped = (text) => text.replace(/\./g, '\\.')
const literalPattern = new RegExp(
	`\\b${escaped(version)}\\b|Gophenberg ${escaped(featureVersion)}\\b`,
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
					`${file.path}: pins a version literal, use %VERSION% or %FEATURE_VERSION%\n  ${node.value.slice(0, 120)}`,
				)
			}
			node.value = substitute(node.value)
		})
	}
}
