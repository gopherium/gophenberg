// SPDX-License-Identifier: Apache-2.0

/** The image service that loads no native code. */
const passthroughImages = 'astro/assets/services/noop'

/** The adapter the artifact is built for. */
const nodeAdapter = '@astrojs/node'

/** The parts of an Astro config the profile reads. */
export interface ProfileConfig {
	output?: string
	adapter?: { name?: string }
	image?: { service?: { entrypoint?: string } }
	vite?: { ssr?: { noExternal?: unknown } }
}

/**
 * Returns the config a theme is built under.
 * @returns The settings the integration applies.
 */
export function buildProfile() {
	return {
		output: 'server' as const,
		image: { service: { entrypoint: passthroughImages, config: {} } },
		vite: { ssr: { noExternal: true as const } },
	}
}

/**
 * Returns the issue for a config not rendering on demand.
 * @param config - The config Astro settled on.
 * @returns The issue, or nothing when the setting holds.
 */
function outputIssue(config: ProfileConfig): string | undefined {
	if (config.output !== 'server') {
		return `gophenberg: output is ${config.output}, want server, since a theme is served on demand`
	}
	return undefined
}

/**
 * Returns the issue for a config naming a foreign adapter.
 * @param config - The config Astro settled on.
 * @returns The issue, or nothing when the setting holds.
 */
function adapterIssue(config: ProfileConfig): string | undefined {
	if (config.adapter && config.adapter.name !== nodeAdapter) {
		return (
			`gophenberg: adapter is ${config.adapter.name}, want ${nodeAdapter}, ` +
			'since the supervisor runs the artifact on node'
		)
	}
	return undefined
}

/**
 * Returns the issue for a config swapping the image service.
 * @param config - The config Astro settled on.
 * @returns The issue, or nothing when the setting holds.
 */
function imageIssue(config: ProfileConfig): string | undefined {
	const service = config.image?.service?.entrypoint
	if (service !== passthroughImages) {
		return (
			`gophenberg: image service is ${service}, want ${passthroughImages}, ` +
			'since a native addon cannot load from a theme artifact'
		)
	}
	return undefined
}

/**
 * Returns the issue for a config not bundling every dependency.
 * @param config - The config Astro settled on.
 * @returns The issue, or nothing when the setting holds.
 */
function bundlingIssue(config: ProfileConfig): string | undefined {
	if (config.vite?.ssr?.noExternal !== true) {
		return 'gophenberg: vite.ssr.noExternal was overridden, want true, since the artifact must run without node_modules'
	}
	return undefined
}

/** The checks a resolved config must hold, one per pinned setting. */
const profileChecks = [outputIssue, adapterIssue, imageIssue, bundlingIssue]

/**
 * Returns the issue with a resolved config, or nothing when it holds the profile.
 * @param config - The config Astro settled on.
 * @returns The issue to fail the build with.
 */
export function profileIssue(config: ProfileConfig): string | undefined {
	for (const check of profileChecks) {
		const issue = check(config)
		if (issue) {
			return issue
		}
	}
	return undefined
}
