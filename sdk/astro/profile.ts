// SPDX-License-Identifier: Apache-2.0

/** The image service that needs no native code, so a theme artifact stays portable. */
const passthroughImages = 'astro/assets/services/noop'

/** The image service Astro reaches for by default, which loads a native addon. */
const nativeImages = 'sharp'

/** The parts of an Astro config the profile reads. */
export interface ProfileConfig {
	output?: string
	image?: { service?: { entrypoint?: string } }
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
 * Returns what is wrong with a resolved config, or nothing when it holds the profile.
 * @param config - The config Astro settled on.
 * @returns The complaint to fail the build with.
 */
export function profileComplaint(config: ProfileConfig): string | undefined {
	if (config.output !== 'server') {
		return `gophenberg: output is ${config.output}, want server, since a theme is served on demand`
	}
	const service = config.image?.service?.entrypoint ?? passthroughImages
	if (service.includes(nativeImages)) {
		return (
			`gophenberg: image service is ${service}, want ${passthroughImages}, ` +
			'since sharp cannot load from a theme artifact'
		)
	}
	return undefined
}
