// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'

import { describe, expect, test } from 'vitest'

import { contentApiPath, generator, isBlockName, kitFeatureVersion, kitName, kitVersion } from '../kit.ts'

/** The manifest the package ships. */
const manifest = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8')) as {
	name: string
	version: string
}

describe('kit identity', () => {
	test('names the package a theme depends on', () => {
		expect(kitName).toBe('@gophenberg/astro')
	})

	test('names the package the manifest publishes', () => {
		expect(kitName).toBe(manifest.name)
	})

	test('addresses the content API a theme reads through', () => {
		expect(contentApiPath).toBe('/api/content/v1')
	})
})

/** The major and minor of the manifest version, taken by a mechanism the kit does not share. */
const majorMinor = (/^(\d+\.\d+)\./.exec(manifest.version) as RegExpExecArray)[1]

describe('what a themed page reports', () => {
	test('names the product and drops the patch version', () => {
		expect(generator).toBe(`Gophenberg ${majorMinor}`)
		expect(generator).toMatch(/^Gophenberg \d+\.\d+$/)
	})

	test('reports the feature version the readiness probe answers with', () => {
		expect(kitFeatureVersion).toBe(majorMinor)
	})
})

describe('the version the kit ships at', () => {
	test('matches the version the manifest publishes', () => {
		expect(kitVersion).toBe(manifest.version)
	})

	test('is a release version, never a range', () => {
		expect(kitVersion).toMatch(/^\d+\.\d+\.\d+$/)
	})
})

describe('isBlockName', () => {
	test('accepts the names the editor serializes', () => {
		expect(isBlockName('core/paragraph')).toBe(true)
		expect(isBlockName('paragraph')).toBe(true)
		expect(isBlockName('my-plugin/my-block')).toBe(true)
	})

	test('rejects anything that is not a block name', () => {
		expect(isBlockName('')).toBe(false)
		expect(isBlockName('Core/Paragraph')).toBe(false)
		expect(isBlockName('core/')).toBe(false)
		expect(isBlockName('/paragraph')).toBe(false)
		expect(isBlockName('core/para/graph')).toBe(false)
		expect(isBlockName('9lives')).toBe(false)
	})
})
