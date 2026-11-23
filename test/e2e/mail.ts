// SPDX-License-Identifier: Apache-2.0

import { Buffer } from 'node:buffer'

import { SMTPServer } from 'smtp-server'

import { mailURL, smtpPort } from './env'

/** How long a test waits for a mail before it fails. */
const MAIL_WAIT_MS = 10_000

/** How often the sink looks again for a mail it waits for. */
const MAIL_POLL_MS = 50

/**
 * The mail sink that collects the messages the server under test delivers.
 */
export interface MailSink {
	/** Answers the decoded body of the first mail addressed to one recipient. */
	waitFor(recipient: string): Promise<string>
	/** Forgets every mail collected so far. */
	clear(): void
	/** Stops listening once no other holder needs the sink. */
	close(): Promise<void>
}

/** A mail the sink collected, with the one recipient it went to. */
interface HeldMail {
	to: string
	body: string
}

/**
 * Decodes a quoted-printable body the way a mail client does.
 * @param encoded - The encoded body.
 * @returns The decoded text.
 */
function decodeQuotedPrintable(encoded: string): string {
	return encoded
		.replace(/=\r?\n/g, '')
		.replace(/(?:=[0-9A-Fa-f]{2})+/g, (run) => Buffer.from(run.replaceAll('=', ''), 'hex').toString('utf8'))
}

/**
 * Returns the decoded body of one raw message.
 * @param raw - The message as the sink received it.
 * @returns The decoded body.
 */
function bodyOf(raw: string): string {
	const separator = raw.indexOf('\r\n\r\n')
	return decodeQuotedPrintable(separator === -1 ? raw : raw.slice(separator + 4))
}

/** The sink this process shares, with the number of holders. */
let running: { sink: MailSink; holders: number } | undefined

/**
 * Starts the shared mail sink, or joins the one already listening.
 * @returns The listening sink, released by its own close.
 */
export async function startMailSink(): Promise<MailSink> {
	if (running) {
		running.holders += 1
		return running.sink
	}
	const sink = await listen()
	running = { sink, holders: 1 }
	return sink
}

/**
 * Starts one mail sink on the port the mail server relays to, with neither STARTTLS nor AUTH.
 * @returns The listening sink.
 */
async function listen(): Promise<MailSink> {
	const held: HeldMail[] = []
	const server = new SMTPServer({
		authOptional: true,
		disabledCommands: ['STARTTLS', 'AUTH'],
		onData(stream, session, callback) {
			const chunks: Buffer[] = []
			stream.on('data', (chunk: Buffer) => chunks.push(chunk))
			stream.on('end', () => {
				const body = bodyOf(Buffer.concat(chunks).toString('utf8'))
				for (const recipient of session.envelope.rcptTo) {
					held.push({ to: recipient.address, body })
				}
				callback()
			})
		},
	})
	await new Promise<void>((resolve, reject) => {
		server.once('error', reject)
		server.listen(smtpPort, '127.0.0.1', resolve)
	})

	return {
		async waitFor(recipient: string): Promise<string> {
			const deadline = Date.now() + MAIL_WAIT_MS
			while (Date.now() < deadline) {
				const found = held.find((mail) => mail.to === recipient)
				if (found) {
					return found.body
				}
				await new Promise((resolve) => setTimeout(resolve, MAIL_POLL_MS))
			}
			throw new Error(`no mail arrived for ${recipient}, the sink holds ${held.length}`)
		},
		clear: () => {
			held.length = 0
		},
		close: async () => {
			if (running && running.holders > 1) {
				running.holders -= 1
				return
			}
			running = undefined
			await new Promise<void>((resolve) => {
				server.close(() => resolve())
			})
		},
	}
}

/**
 * Returns the path and query of the link a mail body carries under the mail server's public address.
 * @param body - The decoded mail body.
 * @param path - The path of the link to find.
 * @returns The link, relative to the site.
 */
export function linkFrom(body: string, path: string): string {
	const marker = `${mailURL}${path}?token=`
	const at = body.indexOf(marker)
	if (at === -1) {
		throw new Error(`the mail carries no ${marker} link: ${body}`)
	}
	const token = body.slice(at + marker.length).split(/\s/)[0]
	return `${path}?token=${token}`
}
