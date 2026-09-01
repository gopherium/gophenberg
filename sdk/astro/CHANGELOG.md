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

## [0.11.0] - 2026-09-01

### Added

- `heldSection` reads the values a section holds under its key.
- `heldRows` reads the rows a repeater holds, leaving out what is not one.
- `heldValue` reads the value a path of keys and row numbers addresses.

## [0.10.0] - 2026-08-30

### Added

- A content type's fields carry the settings the operator set on them.
- `mediaFields` and `mediaItems` read the library files a media field names.
- `mediaUrl` addresses a library file from a theme serving its own origin.
- `MediaValue` and `MediaRendition` type what a media field serves.

## [0.9.0] - 2026-08-25

### Changed

- The kit is versioned on its own line, released on `astro@X.Y.Z` tags,
  no longer in step with Gophenberg.
