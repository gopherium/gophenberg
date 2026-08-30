# Changelog

All notable changes to `@gophenberg/astro` are documented in this
file. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
package follows [Semantic Versioning](https://semver.org/). While at
0.x, minor releases may contain breaking changes.

Releases are tagged `astro@X.Y.Z` and publish to npm from CI. The
npm-style tag stays invisible to the Go toolchain, unlike a
`sdk/astro/vX.Y.Z` tag naming the directory as a module. Versions
through 0.8.0 shipped in step with Gophenberg under its `vX.Y.Z` tags.

## [Unreleased]

### Added

- A content type's fields carry the settings the operator set on them.
- `mediaFields` and `mediaItems` read the library files a media field names.
- `mediaUrl` addresses a library file from a theme serving its own origin.
- `MediaValue` and `MediaRendition` type what a media field serves.

### Changed

- The kit is versioned on its own line, released on `astro@X.Y.Z` tags,
  no longer in step with Gophenberg.
