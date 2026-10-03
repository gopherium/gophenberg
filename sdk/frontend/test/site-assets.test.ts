// SPDX-License-Identifier: Apache-2.0

import { mkdtempSync, readFileSync, readdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import {
	blocksCss,
	faviconSvg,
	presetsCss,
	writeSiteAssets,
} from '../scripts/siteAssets.ts'

test('presetsCss declares a custom property for every default colour', () => {
	const css = presetsCss()

	expect(css).toContain('--wp--preset--color--vivid-red: #cf2e2e;')
	expect(css).toContain('--wp--preset--color--black: #000000;')
})

test('presetsCss declares the font sizes the editor offers', () => {
	const css = presetsCss()

	expect(css).toContain('--wp--preset--font-size--large: 36px;')
	expect(css).toContain('--wp--preset--font-size--small: 13px;')
})

test('presetsCss declares the default gradients', () => {
	expect(presetsCss()).toContain('--wp--preset--gradient--')
})

test('presetsCss carries the classes the editor writes onto blocks', () => {
	const css = presetsCss()

	expect(css).toContain('.has-vivid-red-color')
	expect(css).toContain('.has-vivid-red-background-color')
	expect(css).toContain('.has-large-font-size')
})

test('presetsCss carries the layout rules the server would otherwise inject', () => {
	const css = presetsCss()

	expect(css).toContain('--wp--style--global--content-size')
	expect(css).toContain('.is-layout-flow')
	expect(css).toContain('.is-layout-constrained')
	expect(css).toContain('.is-layout-flex')
})

test('blocksCss carries the styles the pinned block library ships', () => {
	const css = blocksCss()

	expect(css.length).toBeGreaterThan(10000)
	expect(css).toContain('.wp-block-quote')
})

test('faviconSvg is a standalone scalable mark', () => {
	const svg = faviconSvg()

	expect(svg).toContain('<svg')
	expect(svg).toContain('xmlns="http://www.w3.org/2000/svg"')
	expect(svg).toContain('viewBox')
})

test('writeSiteAssets writes the three assets the public pages link', () => {
	const directory = mkdtempSync(join(tmpdir(), 'branded-'))

	writeSiteAssets(directory)

	expect(readdirSync(directory).sort()).toEqual([
		'blocks.css',
		'favicon.svg',
		'presets.css',
	])
	expect(readFileSync(join(directory, 'presets.css'), 'utf8')).toContain('--wp--preset--color--black')
})

test('writeSiteAssets creates the directory when it does not exist yet', () => {
	const directory = join(mkdtempSync(join(tmpdir(), 'branded-')), 'nested', 'gophenberg')

	writeSiteAssets(directory)

	expect(readdirSync(directory)).toHaveLength(3)
})
