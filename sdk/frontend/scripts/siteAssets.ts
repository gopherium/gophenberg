// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'
import { pathToFileURL } from 'node:url'

/** A colour the editor offers under a slug. */
interface ColorPreset {
	slug: string
	color: string
}

/** A font size the editor offers under a slug. */
interface FontSizePreset {
	slug: string
	size: number
}

/** A gradient the editor offers under a slug. */
interface GradientPreset {
	slug: string
	gradient: string
}

/** The presets the block editor falls back to when a theme declares none. */
interface EditorDefaults {
	colors: ColorPreset[]
	fontSizes: FontSizePreset[]
	gradients: GradientPreset[]
}

const require = createRequire(import.meta.url)

/** The block library stylesheets a public page needs, in cascade order. */
const BLOCK_STYLESHEETS = ['common.css', 'style.css', 'theme.css']

/** The content width, wide width, and block gap the layout rules read. */
const LAYOUT_TOKENS: Record<string, string> = {
	'--wp--style--global--content-size': '42rem',
	'--wp--style--global--wide-size': '60rem',
	'--wp--style--block-gap': '1.5rem',
}

/**
 * Returns the directory a pinned package was installed into.
 * @param name - The package name.
 * @returns The absolute path of the package root.
 */
function packageRoot(name: string): string {
	return dirname(require.resolve(`${name}/package.json`))
}

/** The colours, font sizes, and gradients the block editor falls back to. */
const { SETTINGS_DEFAULTS } = (await import(
	pathToFileURL(join(packageRoot('@wordpress/block-editor'), 'build-module/store/defaults.mjs')).href
)) as { SETTINGS_DEFAULTS: EditorDefaults }

/**
 * Returns the stylesheets the pinned block library ships for the front end.
 * @returns The concatenated block library CSS.
 */
export function blocksCss(): string {
	const root = packageRoot('@wordpress/block-library')
	return BLOCK_STYLESHEETS.map((name) => readFileSync(join(root, 'build-style', name), 'utf8')).join('\n')
}

/**
 * Returns the custom properties, preset classes, and layout rules a published page reads.
 * @returns The generated preset CSS.
 */
export function presetsCss(): string {
	const { colors, fontSizes, gradients } = SETTINGS_DEFAULTS
	return [
		`:root {\n${presetProperties(colors, fontSizes, gradients)}}`,
		presetClasses(colors, fontSizes, gradients),
		layoutRules(),
	].join('\n\n')
}

/**
 * Returns the custom property declarations of every preset.
 * @param colors - The default colour presets.
 * @param fontSizes - The default font size presets.
 * @param gradients - The default gradient presets.
 * @returns The declarations, one per line.
 */
function presetProperties(
	colors: ColorPreset[],
	fontSizes: FontSizePreset[],
	gradients: GradientPreset[],
): string {
	return [
		...Object.entries(LAYOUT_TOKENS).map(([name, value]) => `\t${name}: ${value};`),
		...colors.map((preset) => `\t--wp--preset--color--${preset.slug}: ${preset.color};`),
		...fontSizes.map((preset) => `\t--wp--preset--font-size--${preset.slug}: ${preset.size}px;`),
		...gradients.map((preset) => `\t--wp--preset--gradient--${preset.slug}: ${preset.gradient};`),
	]
		.join('\n')
		.concat('\n')
}

/**
 * Returns the classes the editor writes onto blocks carrying a preset.
 * @param colors - The default colour presets.
 * @param fontSizes - The default font size presets.
 * @param gradients - The default gradient presets.
 * @returns The rules, one per line.
 */
function presetClasses(
	colors: ColorPreset[],
	fontSizes: FontSizePreset[],
	gradients: GradientPreset[],
): string {
	return [
		...colors.map(
			(preset) =>
				`.has-${preset.slug}-color { color: var(--wp--preset--color--${preset.slug}) !important; }\n` +
				`.has-${preset.slug}-background-color { ` +
				`background-color: var(--wp--preset--color--${preset.slug}) !important; }`,
		),
		...fontSizes.map(
			(preset) =>
				`.has-${preset.slug}-font-size { font-size: var(--wp--preset--font-size--${preset.slug}) !important; }`,
		),
		...gradients.map(
			(preset) =>
				`.has-${preset.slug}-gradient-background { ` +
				`background: var(--wp--preset--gradient--${preset.slug}) !important; }`,
		),
	].join('\n')
}

/**
 * Returns the spacing and width rules the editor expects the server to inject.
 * @returns The layout CSS.
 */
function layoutRules(): string {
	return [
		'.is-layout-flow > * + * { margin-block-start: var(--wp--style--block-gap); }',
		'.is-layout-constrained > * { max-width: var(--wp--style--global--content-size); margin-inline: auto; }',
		'.is-layout-constrained > .alignwide { max-width: var(--wp--style--global--wide-size); }',
		'.is-layout-flex { display: flex; flex-wrap: wrap; gap: var(--wp--style--block-gap); }',
		'.is-layout-grid { display: grid; gap: var(--wp--style--block-gap); }',
	].join('\n')
}

/**
 * Returns the mark a browser shows for the site.
 * @returns The favicon as SVG.
 */
export function faviconSvg(): string {
	return [
		'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" role="img" aria-label="Gophenberg">',
		'<rect width="32" height="32" rx="7" fill="#1e1e1e"/>',
		'<path d="M21.5 12.2a6.4 6.4 0 1 0 1 5.6h-6.2v-3h9.1v1.5a9.4 9.4 0 1 1-1.6-5.3z" fill="#ffffff"/>',
		'</svg>',
		'',
	].join('\n')
}

/**
 * Writes the site's stylesheets and icon into a directory.
 * @param directory - The directory the assets are written into.
 */
export function writeSiteAssets(directory: string): void {
	mkdirSync(directory, { recursive: true })
	writeFileSync(join(directory, 'blocks.css'), blocksCss())
	writeFileSync(join(directory, 'presets.css'), presetsCss())
	writeFileSync(join(directory, 'favicon.svg'), faviconSvg())
}
