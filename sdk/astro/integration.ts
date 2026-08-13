// SPDX-License-Identifier: Apache-2.0

import node from '@astrojs/node'
import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import type { AstroIntegration } from 'astro'

import { kitName } from './kit.ts'
import { buildProfile, profileIssue } from './profile.ts'

/** The module the injected routes read the active theme through. */
const themeModuleId = 'virtual:gophenberg/theme'

/** The path a theme lives at when it names none. */
const defaultThemePath = './src/theme.ts'

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
			},
		},
	}
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
