// SPDX-License-Identifier: Apache-2.0

// A stub theme server that stays alive without ever listening, so it never reports ready.

process.stderr.write('stub alive but never listening\n')
setInterval(() => {}, 1000)
