// SPDX-License-Identifier: Apache-2.0

// A stub theme server that dies before it ever listens.

process.stderr.write('stub crashing on boot\n')
process.exit(1)
