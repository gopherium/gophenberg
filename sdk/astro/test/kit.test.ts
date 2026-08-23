// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'

import { describe, expect, test } from 'vitest'

import {
	contentApiPath,
	generator,
	isBlockName,
	kitFeatureVersion,
	kitName,
	kitServes,
	kitVersion,
	servedBy,
} from '../kit.ts'

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

describe('kitServes, the rule a host answers a theme by', () => {
	test.each([
		{ served: '0.9.0', declared: '0.9.0', want: true, why: 'the same version' },
		{ served: '0.9.0', declared: '0.8.0', want: false, why: 'an earlier minor while at 0.x' },
		{ served: '0.9.0', declared: '0.10.0', want: false, why: 'a later minor while at 0.x' },
		{ served: '0.9.2', declared: '0.9.1', want: true, why: 'an earlier patch while at 0.x' },
		{ served: '0.9.0', declared: '0.9.1', want: false, why: 'a later patch while at 0.x' },
		{ served: '1.4.0', declared: '1.2.0', want: true, why: 'an earlier minor once past 1.0' },
		{ served: '1.2.0', declared: '1.4.0', want: false, why: 'a later minor once past 1.0' },
		{ served: '1.2.3', declared: '1.2.1', want: true, why: 'an earlier patch once past 1.0' },
		{ served: '1.0.0', declared: '2.0.0', want: false, why: 'a different major' },
		{ served: '2.0.0', declared: '1.0.0', want: false, why: 'an earlier major' },
		{ served: '1.0.0', declared: '0.9.0', want: false, why: 'zero against one' },
		{ served: '0.9.0', declared: '^0.9.0', want: false, why: 'a range rather than a version' },
		{ served: '0.9.0', declared: '', want: false, why: 'nothing at all' },
		{ served: 'latest', declared: '0.9.0', want: false, why: 'a served value that is not a version' },
	])('$why', ({ served, declared, want }) => {
		expect(kitServes(served, declared)).toBe(want)
	})
})

describe('servedBy, asked of the kit this theme was built with', () => {
	test('is served when the site lists this very version', () => {
		expect(servedBy([kitVersion])).toBe(true)
	})

	test('is served when the site lists it beside another major', () => {
		expect(servedBy(['2.0.0', kitVersion])).toBe(true)
	})

	test('is not served when the site lists nothing', () => {
		expect(servedBy([])).toBe(false)
	})

	test('is not served when every version the site lists answers another kit', () => {
		expect(servedBy(['0.1.0', '2.0.0'])).toBe(false)
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
