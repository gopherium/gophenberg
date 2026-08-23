// SPDX-License-Identifier: Apache-2.0

import node from '@astrojs/node'
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import type { AstroIntegration } from 'astro'

import { kitName, kitVersion } from './kit.ts'
import { buildProfile, profileIssue } from './profile.ts'

/** The module the injected routes read the active theme through. */
const themeModuleId = 'virtual:gophenberg/theme'

/** The path a theme lives at when it names none. */
const defaultThemePath = './src/theme.ts'

/** The file a theme directory declares itself with. */
const manifestName = 'theme.json'

/** The addresses the kit serves, and the files it serves them from. */
const injectedRoutes = [
	{ pattern: '/', entrypoint: 'content.astro' },
	{ pattern: '/[...path]', entrypoint: 'content.astro' },
	{ pattern: '/404', entrypoint: 'not-found.astro' },
	{ pattern: '/_gophenberg/health', entrypoint: 'health.ts' },
]

/** How a theme names itself to the integration. */
export interface GophenbergOptions {
	theme?: string
}

/**
 * Returns the integration wiring a theme, the injected routes, and the build profile into a project.
 * @param options - The path the theme lives at.
 * @returns The integration an Astro config lists.
 */
export function gophenberg(options: GophenbergOptions = {}): AstroIntegration {
	const themePath = options.theme ?? defaultThemePath
	let settled: { root: URL; outDir: URL } | undefined
	return {
		name: kitName,
		hooks: {
			'astro:config:setup': ({ config, updateConfig, injectRoute }) => {
				const themeFile = fileURLToPath(new URL(themePath, config.root))
				if (!existsSync(themeFile)) {
					throw new Error(`gophenberg: no theme at ${themeFile}, name one with gophenberg({ theme })`)
				}
				const profile = buildProfile()
				const adapter = node({ mode: 'standalone' })
				updateConfig({
					...profile,
					adapter,
					integrations: [adapter],
					vite: { ...profile.vite, plugins: [themePlugin(themeFile)] },
				})
				for (const route of injectedRoutes) {
					injectRoute({
						pattern: route.pattern,
						entrypoint: `${kitName}/routes/${route.entrypoint}`,
						prerender: false,
					})
				}
			},
			'astro:config:done': ({ config }) => {
				const issue = profileIssue(config)
				if (issue) {
					throw new Error(issue)
				}
				settled = { root: config.root, outDir: config.outDir }
			},
			'astro:build:done': () => {
				if (settled) {
					stampManifest(settled.root, settled.outDir)
				}
			},
		},
	}
}

/**
 * Writes the manifest a built theme ships, naming the kit it was built with.
 * @param root - The project root the theme declares itself in.
 * @param outDir - Where the build leaves the artifact.
 */
function stampManifest(root: URL, outDir: URL): void {
	const source = new URL(manifestName, root)
	if (!existsSync(source)) {
		throw new Error(`gophenberg: no ${manifestName} at ${fileURLToPath(source)}, name the theme and its version there`)
	}
	const declared = JSON.parse(readFileSync(source, 'utf8')) as { name?: unknown; version?: unknown }
	if (typeof declared.name !== 'string' || declared.name === '') {
		throw new Error(`gophenberg: ${fileURLToPath(source)} names no theme, set name to what the zip is called`)
	}
	if (typeof declared.version !== 'string' || declared.version === '') {
		throw new Error(`gophenberg: ${fileURLToPath(source)} names no version, set version to this theme's own release`)
	}
	const stamped = { name: declared.name, version: declared.version, kit: kitVersion }
	writeFileSync(new URL(manifestName, outDir), `${JSON.stringify(stamped)}\n`)
}

/**
 * Returns the plugin serving the active theme to the injected routes.
 * @param themeFile - The file the theme lives at.
 * @returns The vite plugin.
 */
function themePlugin(themeFile: string) {
	const resolvedId = `\0${themeModuleId}`
	return {
		name: 'gophenberg:theme',
		resolveId: (id: string) => (id === themeModuleId ? resolvedId : undefined),
		load: (id: string) =>
			id === resolvedId ? `export { default as theme } from ${JSON.stringify(themeFile)}` : undefined,
	}
}
