// SPDX-License-Identifier: Apache-2.0

import {
	Button,
	InputControl,
	Notice,
	SelectControl,
	Stack,
	Tabs,
	Text,
	TextareaControl,
	ValidatedInputControl,
} from '@gophenberg/frontend-sdk'
import { ErrorNotice, useRejectedField } from '@gopherium/godmin'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { __, _x } from '@wordpress/i18n'
import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'

import { ChoicesEditor } from './ChoicesEditor'
import { ConditionsTabName, conditionSources, FieldConditions } from './FieldConditions'
import { TickBox } from './fieldControls'
import { fieldDraftOf, rebasedField, settingPart } from './fieldDraft'
import { fieldAt } from './fieldNotes'
import { changedElsewhere } from './groupDraft'
import { groupsQueryKey, pickOf, staleFieldMessage } from './groups'
import { chosenOf } from './select'
import { LISTABLE, settingAnswers, settingsOffered } from './settingControls'
import { pluralOf } from './typeText'
import { ownValue, pairsOf, pickedLabel, requirable } from './types'
import type { FieldWork } from './fieldControls'
import type { FieldDraft, FieldPart } from './fieldDraft'
import type { FieldNote, Rejection } from './fieldNotes'
import type { FieldGroup } from './groups'
import type { SettingControl, SettingTab } from './settingControls'
import type { ContentField, ContentType } from './types'

/** The shapes of the settings whose box shows the server's words under it. */
const BOXED: SettingControl['shape'][] = ['text', 'number']

/** The tab of an open field: one a setting shows in, or the one holding its rules. */
type FieldTab = SettingTab | 'conditions'

/** Where an open field stands, what it reads, and the parts the table hands it. */
interface OpenFieldProps {
	group: number
	groups: FieldGroup[]
	types: ContentType[]
	field: ContentField
	/** The dotted path of the field. */
	path: string
	/** The fields standing beside it, which its rules may read. */
	siblings: ContentField[]
	work: FieldWork
	/** The fields it holds and the form declaring another, none for a field holding none. */
	nested: ReactNode
	/** Closes the field, moving the focus to its label when the focus sat inside. */
	onClose: () => void
}

/**
 * Renders the open part of a field.
 * @param props - Where the field stands, what it reads, the page's work on its fields, and the parts it holds.
 * @returns What the last save left, the form in tabs, the fields it holds and the form declaring another, then the
 * buttons closing it.
 */
export function OpenField(props: OpenFieldProps) {
	const { field, path, work } = props
	const pageDraft = ownValue(work.drafts, path)
	const draft = pageDraft ?? fieldDraftOf(field)
	const note = ownValue(work.notes, path)
	return (
		<Stack direction="column" gap="md">
			<FieldNotices group={props.group} field={field} path={path} draft={draft} note={note} work={work} />
			<form
				onSubmit={(event) => {
					event.preventDefault()
					work.onSave()
				}}
			>
				<FieldTabs
					field={field}
					draft={draft}
					note={note}
					top={!path.includes('.')}
					types={props.types}
					groups={props.groups}
					siblings={props.siblings}
					readOnly={work.saving}
					onDraft={(next) => work.onDraft(path, next)}
				/>
				<button type="submit" hidden />
			</form>
			{props.nested}
			<FieldActions path={path} work={work} hasDraft={pageDraft !== undefined} onClose={props.onClose} />
		</Stack>
	)
}

/**
 * Renders the footer closing an open field: Close field, then Discard changes, under a line across the field.
 * @param props - The path of the field, the page's work on its fields, whether it holds a draft, and what closes it.
 * @returns The footer, Discard changes shown while a draft is held and off while the header Save runs.
 */
function FieldActions(props: Pick<OpenFieldProps, 'path' | 'work' | 'onClose'> & { hasDraft: boolean }) {
	const { path, work } = props
	return (
		<Stack direction="row" gap="sm" align="center" wrap="wrap" className="gophenberg-field__footer">
			<Button variant="outline" size="compact" onClick={props.onClose}>
				{__('Close field', 'gophenberg')}
			</Button>
			{props.hasDraft && (
				<Button
					variant="minimal"
					tone="neutral"
					size="compact"
					disabled={work.saving}
					onClick={() => {
						props.onClose()
						work.onDraft(path, null)
					}}
				>
					{__('Discard changes', 'gophenberg')}
				</Button>
			)}
		</Stack>
	)
}

/** What the notices of an open field read. */
interface NoticesProps {
	group: number
	field: ContentField
	path: string
	draft: FieldDraft
	note: FieldNote | undefined
	work: FieldWork
}

/**
 * Returns the box of the setting a rejected save named, when its control shows the server's words under it.
 * @param offered - The settings the field's kind takes.
 * @param rejected - Why the last save was rejected, none while no rejection stands.
 * @returns The setting, none when the words go in the field's own notice.
 */
function boxOf(offered: SettingControl[], rejected: Rejection | undefined): SettingControl | undefined {
	return offered.find((setting) => setting.name === rejected?.setting && BOXED.includes(setting.shape))
}

/**
 * Returns the server's message a rejected save leaves in the field's own notice, when no box takes it.
 * @param draft - The draft of the field.
 * @param note - What the field shows of its last save.
 * @returns The message, empty while it shows under a box or no rejection stands.
 */
function noticeMessage(draft: FieldDraft, note: FieldNote | undefined): string {
	const rejection = note?.rejected
	if (rejection === undefined || boxOf(settingsOffered(draft.base), rejection) !== undefined) {
		return ''
	}
	return rejection.words
}

/**
 * Returns what reads the groups again and moves the page's draft of a field onto the version saved elsewhere.
 * @param group - The id of the group of the field.
 * @param path - The dotted path of the field.
 * @param work - The page's work on its fields.
 * @returns The mutation, which moves the draft the page has once the read ends and leaves a field without one alone.
 */
function useLoadSaved(group: number, path: string, work: FieldWork) {
	const client = useQueryClient()
	return useMutation({
		mutationFn: async () => {
			await client.refetchQueries({ queryKey: groupsQueryKey }, { throwOnError: true })
			const savedGroup = client.getQueryData<FieldGroup[]>(groupsQueryKey)?.find((cachedGroup) => cachedGroup.id === group)
			return fieldAt(savedGroup?.fields ?? [], path)
		},
		onSuccess: (savedField) => {
			const pageDraft = ownValue(work.drafts, path)
			if (savedField !== undefined && pageDraft !== undefined) {
				work.onRebase(path, rebasedField(pageDraft, savedField))
			}
		},
	})
}

/**
 * Renders what the last save of a field left in its open part.
 * @param props - The group, the field, its path, its draft, what its last save left, and the page's work on it.
 * @returns The notice that someone saved it first, the server's message no box takes, or nothing while none stands.
 */
function FieldNotices(props: NoticesProps) {
	const loadSaved = useLoadSaved(props.group, props.path, props.work)
	const message = noticeMessage(props.draft, props.note)
	return (
		<>
			{props.note?.stale === true && (
				<Notice.Root intent="warning" role="alert">
					<Notice.Description>{staleFieldMessage(props.field.label)}</Notice.Description>
					<Notice.Actions>
						<Notice.ActionButton loading={loadSaved.isPending} onClick={() => loadSaved.mutate()}>
							{__('Load saved version', 'gophenberg')}
						</Notice.ActionButton>
					</Notice.Actions>
				</Notice.Root>
			)}
			{message !== '' && <ErrorNotice>{message}</ErrorNotice>}
		</>
	)
}

/** What the tabs of an open field show and edit. */
interface TabsProps {
	/** The field as last read, whose facts show. */
	field: ContentField
	draft: FieldDraft
	/** What the field shows of its last save. */
	note: FieldNote | undefined
	/** Whether the field stands at the top of its group, where the admin list reaches. */
	top: boolean
	types: ContentType[]
	groups: FieldGroup[]
	/** The fields standing beside it, which its rules may read. */
	siblings: ContentField[]
	readOnly: boolean
	onDraft: (next: FieldDraft) => void
}

/** The tabs of an open field and the settings its kind takes. */
type TabProps = TabsProps & { offered: SettingControl[] }

/**
 * Returns the tab an open field shows, moved to the tab of the setting a new rejection names, and to General once
 * Conditional logic goes.
 * @param offered - The settings the field's kind takes.
 * @param rejected - Why the last save was rejected, none while no rejection stands.
 * @param ruled - Whether the field offers its Conditional logic tab.
 * @returns The tab shown and what shows another.
 */
function useShownTab(offered: SettingControl[], rejected: Rejection | undefined, ruled: boolean) {
	const [tab, setTab] = useState<FieldTab>('general')
	const [seen, setSeen] = useState(rejected)
	if (seen !== rejected) {
		setSeen(rejected)
		const boxed = boxOf(offered, rejected)
		if (boxed !== undefined) {
			setTab(boxed.tab)
		}
	}
	if (tab === 'conditions' && !ruled) {
		setTab('general')
	}
	return { tab, show: (next: FieldTab) => setTab(next) }
}

/**
 * Renders the tabs of an open field, leaving out Validation when its kind holds nothing to check and Conditional logic
 * while no sibling can be read.
 * @param props - The field, its draft, what its last save left, whether it stands at the top, what it reads, its
 * siblings, and what changes the draft.
 * @returns The tabs.
 */
function FieldTabs(props: TabsProps) {
	const offered = settingsOffered(props.draft.base)
	const sources = conditionSources(props.siblings, props.draft.base)
	const ruled = sources.length > 0
	const shown = useShownTab(offered, props.note?.rejected, ruled)
	const validates = requirable(props.draft.base.kind) || offered.some((setting) => setting.tab === 'validation')
	return (
		<Tabs.Root value={shown.tab} onValueChange={(value: FieldTab) => shown.show(value)}>
			<Stack direction="column" gap="lg">
				<Tabs.List variant="minimal">
					<Tabs.Tab value="general">{__('General', 'gophenberg')}</Tabs.Tab>
					{validates && <Tabs.Tab value="validation">{__('Validation', 'gophenberg')}</Tabs.Tab>}
					<Tabs.Tab value="presentation">{__('Presentation', 'gophenberg')}</Tabs.Tab>
					{ruled && (
						<Tabs.Tab value="conditions">
							<ConditionsTabName draft={props.draft} />
						</Tabs.Tab>
					)}
				</Tabs.List>
				<Tabs.Panel value="general" className="godmin-form">
					<GeneralTab {...props} offered={offered} />
				</Tabs.Panel>
				{validates && (
					<Tabs.Panel value="validation" className="godmin-form">
						<ValidationTab {...props} offered={offered} />
					</Tabs.Panel>
				)}
				<Tabs.Panel value="presentation" className="godmin-form">
					<PresentationTab {...props} offered={offered} />
				</Tabs.Panel>
				{ruled && (
					<Tabs.Panel value="conditions" className="godmin-form">
						<FieldConditions
							draft={props.draft}
							sources={sources}
							readOnly={props.readOnly}
							onDraft={props.onDraft}
						/>
						<LostLine note={props.note} part={settingPart('conditions')} />
					</Tabs.Panel>
				)}
			</Stack>
		</Tabs.Root>
	)
}

/**
 * Returns the line saying the saved version won over the draft on one part, while it did.
 * @param note - What the field shows of its last save.
 * @param part - The part.
 * @returns The line, none while the draft kept its own value or never moved.
 */
function lostLine(note: FieldNote | undefined, part: FieldPart): string | undefined {
	return note?.lost.includes(part) === true ? changedElsewhere() : undefined
}

/**
 * Renders the line saying the saved version won over the draft on one part, under a control that takes no words.
 * @param props - What the field shows of its last save, and the part.
 * @returns The line, or nothing while the draft kept its own value.
 */
function LostLine(props: { note: FieldNote | undefined; part: FieldPart }) {
	const words = lostLine(props.note, props.part)
	return words === undefined ? null : <Text variant="body-sm">{words}</Text>
}

/**
 * Renders the General tab: the label, the facts fixed once the field exists, its choices and its general settings.
 * @param props - The field, its draft, what its last save left, what it reads, the settings its kind takes, and what
 * changes the draft.
 * @returns The tab's controls.
 */
function GeneralTab(props: TabProps) {
	const { draft } = props
	return (
		<>
			<InputControl
				label={__('Label', 'gophenberg')}
				description={
					lostLine(props.note, 'label') ?? __('The name changes. Nothing stored under this field moves.', 'gophenberg')
				}
				autoComplete="off"
				value={draft.label}
				readOnly={props.readOnly}
				onValueChange={(label) => props.onDraft({ ...draft, label })}
			/>
			<FieldFacts field={props.field} types={props.types} groups={props.groups} />
			{draft.base.kind === 'choice' && (
				<>
					<ChoicesEditor
						pairs={pairsOf(draft.settings)}
						readOnly={props.readOnly}
						onChange={(choices) => props.onDraft({ ...draft, settings: { ...draft.settings, choices } })}
					/>
					<LostLine note={props.note} part={settingPart('choices')} />
				</>
			)}
			<SettingBoxes {...props} tab="general" />
		</>
	)
}

/**
 * Renders the Validation tab: whether the field is required, then the bounds its kind takes.
 * @param props - The draft, what its last save left, the settings its kind takes, whether it is read only, and what
 * changes the draft.
 * @returns The tab's controls.
 */
function ValidationTab(props: TabProps) {
	const { draft } = props
	return (
		<>
			{requirable(draft.base.kind) && (
				<TickBox
					label={__('Required', 'gophenberg')}
					checked={draft.required}
					readOnly={props.readOnly}
					onChange={(required) => props.onDraft({ ...draft, required })}
				/>
			)}
			<SettingBoxes {...props} tab="validation" />
		</>
	)
}

/**
 * Renders the Presentation tab: how the field shows, then whether it stands as a column on the admin list.
 * @param props - The draft, what its last save left, whether it stands at the top, the settings its kind takes, and
 * what changes the draft.
 * @returns The tab's controls.
 */
function PresentationTab(props: TabProps) {
	const { draft } = props
	return (
		<>
			<SettingBoxes {...props} tab="presentation" />
			{props.top && LISTABLE.includes(draft.base.kind) && (
				<>
					<TickBox
						label={__('Show in the list', 'gophenberg')}
						checked={draft.settings.listed === true}
						readOnly={props.readOnly}
						onChange={(listed) => props.onDraft({ ...draft, settings: listedSettings(draft, listed) })}
					/>
					<LostLine note={props.note} part={settingPart('listed')} />
				</>
			)}
		</>
	)
}

/**
 * Returns the settings of a draft with the field in the admin list or out of it, counting no change once it is back.
 * @param draft - The draft held.
 * @param listed - Whether the field stands in the list.
 * @returns The settings.
 */
function listedSettings(draft: FieldDraft, listed: boolean): Record<string, unknown> {
	if (listed || draft.base.settings.listed !== undefined) {
		return { ...draft.settings, listed }
	}
	return Object.fromEntries(Object.entries(draft.settings).filter(([key]) => key !== 'listed'))
}

/**
 * Renders the controls of the settings one tab shows.
 * @param props - The draft, what its last save left, the settings its kind takes, the tab, whether they are read
 * only, and what changes them.
 * @returns The controls.
 */
function SettingBoxes(props: TabProps & { tab: SettingTab }) {
	const { draft, note } = props
	return props.offered
		.filter((setting) => setting.tab === props.tab)
		.map((setting) => (
			<SettingBox
				key={setting.name}
				setting={setting}
				typed={draft.typed[setting.name]}
				readOnly={props.readOnly}
				lost={lostLine(note, settingPart(setting.name))}
				rejected={note?.rejected?.setting === setting.name ? note.rejected : undefined}
				onType={(written) => props.onDraft({ ...draft, typed: { ...draft.typed, [setting.name]: written } })}
			/>
		))
}

/** What the control of one setting shows and edits. */
interface BoxProps {
	setting: SettingControl
	/** The text the control holds. */
	typed: string
	readOnly: boolean
	/** The line saying the saved version won over the draft on this setting, none while it did not. */
	lost: string | undefined
	/** Why the last save was rejected on this setting, none while no rejection names it. */
	rejected: Rejection | undefined
	onType: (written: string) => void
}

/**
 * Renders the control of one setting in the shape it takes.
 * @param props - The setting, the text its control holds, whether it is read only, the lines under it, and what to
 * call with new text.
 * @returns The control.
 */
function SettingBox(props: BoxProps) {
	const { setting } = props
	if (setting.shape === 'choice') {
		const answers = settingAnswers()
		return (
			<SelectControl
				label={setting.label}
				description={props.lost}
				items={answers}
				value={chosenOf({ value: props.typed }, answers, answers[0])}
				readOnly={props.readOnly}
				onValueChange={(item) => props.onType(chosenOf(item, answers, answers[0]).value)}
			/>
		)
	}
	if (setting.shape === 'textarea') {
		return (
			<TextareaControl
				label={setting.label}
				description={props.lost}
				value={props.typed}
				readOnly={props.readOnly}
				onValueChange={props.onType}
			/>
		)
	}
	return <SettingInput {...props} />
}

/**
 * Renders the box of a text or number setting, the server's words under it once a save the box sent was rejected.
 * @param props - The setting, the text its box holds, whether it is read only, the lines under it, and what to call
 * with new text.
 * @returns The box.
 */
function SettingInput(props: BoxProps) {
	const { ref, validity, reject } = useRejectedField(props.typed)
	const { rejected, readOnly } = props
	useEffect(() => {
		if (rejected !== undefined && !readOnly) {
			reject(rejected.words, rejected.sent)
		}
	}, [rejected, readOnly, reject])
	return (
		<ValidatedInputControl
			ref={ref}
			customValidity={validity}
			label={props.setting.label}
			description={props.lost}
			type={props.setting.shape}
			autoComplete="off"
			value={props.typed}
			readOnly={readOnly}
			onValueChange={props.onType}
		/>
	)
}

/** One fact about a field fixed once it exists: its name and what it reads. */
interface Fact {
	name: string
	value: string
}

/**
 * Renders the facts fixed once a field exists: its API name, its kind, and what a relation or a Linked from reads.
 * @param props - The field, the types a relation may point at, and the groups a Linked from field reads.
 * @returns The facts.
 */
function FieldFacts(props: { field: ContentField; types: ContentType[]; groups: FieldGroup[] }) {
	const { field } = props
	return (
		<>
			<FactLine name={__('API name', 'gophenberg')}>
				<code>{field.key}</code>
			</FactLine>
			<FactLine name={_x('Kind', 'field type', 'gophenberg')}>{pickedLabel(field)}</FactLine>
			{kindFacts(field, props.types, props.groups).map((fact) => (
				<FactLine key={fact.name} name={fact.name}>
					{fact.value}
				</FactLine>
			))}
		</>
	)
}

/**
 * Renders one fact as plain text under its name.
 * @param props - The name of the fact and what it reads.
 * @returns The fact.
 */
function FactLine(props: { name: string; children: ReactNode }) {
	return (
		<Stack direction="column" gap="xs">
			<Text variant="body-sm">{props.name}</Text>
			<Text>{props.children}</Text>
		</Stack>
	)
}

/**
 * Returns the facts a field's kind adds: where a relation points and how many it holds, or what a Linked from reads.
 * @param field - The field.
 * @param types - The types a relation may point at.
 * @param groups - The groups a Linked from field reads.
 * @returns The facts, none for a kind that adds none.
 */
function kindFacts(field: ContentField, types: ContentType[], groups: FieldGroup[]): Fact[] {
	if (field.kind === 'relation') {
		return relationFacts(field, types)
	}
	return field.kind === 'backlinks' ? linkFacts(field, groups) : []
}

/**
 * Returns where a relation points and how many targets it holds.
 * @param field - The relation.
 * @param types - The types it may point at.
 * @returns The facts, the type's key while no type read carries it.
 */
function relationFacts(field: ContentField, types: ContentType[]): Fact[] {
	const target = types.find((held) => held.key === field.relatesTo)
	return [
		{ name: __('Points at', 'gophenberg'), value: target === undefined ? field.relatesTo : pluralOf(target) },
		{
			name: __('Holds', 'gophenberg'),
			value: field.many ? __('Many targets', 'gophenberg') : __('One target', 'gophenberg'),
		},
	]
}

/**
 * Returns the group and the relation a Linked from field reads.
 * @param field - The Linked from field.
 * @param groups - The groups it may read.
 * @returns The facts, the keys while no group read carries them.
 */
function linkFacts(field: ContentField, groups: FieldGroup[]): Fact[] {
	const pick = pickOf(field)
	if (pick === null) {
		return [
			{
				name: __('Through', 'gophenberg'),
				value: __('It reads a relation inside a container, and only an import can point it elsewhere.', 'gophenberg'),
			},
		]
	}
	const source = groups.find((held) => held.key === pick.group)
	const relation = source?.fields.find((held) => held.key === pick.field)
	return [
		{ name: __('Reads from', 'gophenberg'), value: source?.title ?? pick.group },
		{ name: __('Through', 'gophenberg'), value: relation?.label ?? pick.field },
	]
}
