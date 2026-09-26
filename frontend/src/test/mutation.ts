// SPDX-License-Identifier: Apache-2.0

/** Whether a mutation run rewrote the sources the tests read. */
export const MUTATION_RUN = process.env.STRYKER_MUTATOR_WORKER !== undefined
