// SPDX-License-Identifier: Apache-2.0

import { __ } from '@wordpress/i18n'

/** The text domain every Gophenberg owned string names. */
const DOMAIN = 'gophenberg'

/**
 * Returns the message each error code stands for, read fresh so the
 * catalogue the reader loaded is the one that answers.
 * @returns The messages, keyed by code.
 */
export function errorTemplates(): Record<string, string> {
	return {
		address_reserved: __(
			'Gophenberg keeps this address for itself, so your content cannot answer there. Give the item a different slug.',
			DOMAIN,
		),
		archive_entry_escapes: __(
			'The entry %(entry)s in this zip would be written outside the theme folder, so nothing was installed. Package the theme again.',
			DOMAIN,
		),
		archive_too_many_entries: __(
			'This zip holds %(entries)d files, more than the %(max)d a theme may carry. Package only the built theme.',
			DOMAIN,
		),
		archive_unreadable: __(
			'This file could not be opened as a zip. Package the theme again and upload the new file.',
			DOMAIN,
		),
		author_required: __('Every item needs an author who exists. Pick one from the list and save again.', DOMAIN),
		autosave_not_found: __(
			'Gophenberg has no autosave of yours for this item. Open the item to see the version that is stored.',
			DOMAIN,
		),
		body_field_required: __(
			'This request left out %(field)s, which Gophenberg needs. Reload the page and try again.',
			DOMAIN,
		),
		body_malformed: __(
			'Gophenberg could not read what your browser sent. Reload the page and try again, and tell whoever looks after your site if it keeps happening.',
			DOMAIN,
		),
		body_unknown_attribute: __(
			'This request carried %(attribute)s, which fields do not take. Reload the page so the admin and the server agree again.',
			DOMAIN,
		),
		client_assets_missing: __(
			'The theme %(name)s does not carry %(path)s as a folder, so its styles and scripts are missing. Package the built theme rather than its source.',
			DOMAIN,
		),
		content_already_trashed: __('This item is already in the trash. Restore it first to work on it again.', DOMAIN),
		content_holds_children: __('This item still holds items nested inside it. Move or delete those first.', DOMAIN),
		content_id_malformed: __(
			'That web address does not name an item Gophenberg can look up. Go back to the list and open the item from there.',
			DOMAIN,
		),
		content_not_found: __(
			'This item does not exist, and it may have been deleted. Go back to the list to see what is there.',
			DOMAIN,
		),
		content_stale_update: __(
			'Someone else saved changes to this item while you were editing, so yours were not saved. Copy your text somewhere safe, then reload the item and put your changes back in.',
			DOMAIN,
		),
		default_type_required: __(
			'Your site always needs one default content type, so this one cannot be turned off or deleted. Make another type the default first.',
			DOMAIN,
		),
		field_key_malformed: __(
			'A field key has to start with a lowercase letter and then hold only lowercase letters, numbers and hyphens. Rename the field so its key comes out in that shape.',
			DOMAIN,
		),
		field_kind_unknown: __('There is no kind of field called %(value)s. Pick one of these instead: %(allowed)s.', DOMAIN),
		field_label_required: __('A field needs a name. Type one and try again.', DOMAIN),
		field_not_found: __(
			'That field is not on this content type any more. Reload the page to see the fields it has now.',
			DOMAIN,
		),
		field_not_relational: __(
			'Only a relation field points at another content type or holds many items. Pick the Relation kind, or leave the target and the many option empty.',
			DOMAIN,
		),
		field_order_incomplete: __(
			'The new order has to name every field of this content type exactly once. Reload the page and move the fields again.',
			DOMAIN,
		),
		field_required: __(
			'The field %(field)s has to hold a value before this item can be published. Fill it in, or leave the item as a draft.',
			DOMAIN,
		),
		field_shape_identity: __(
			'One of the items picked for %(field)s was not named in a way Gophenberg understands. Reload the page and pick the items again.',
			DOMAIN,
		),
		field_shape_kind: __(
			'The field %(field)s cannot hold the value it was given. Check what kind of field it is and enter a value that matches.',
			DOMAIN,
		),
		field_shape_list: __(
			'The field %(field)s takes a list of items, and what arrived was not a list. Reload the page and pick the items again.',
			DOMAIN,
		),
		field_shape_value: __(
			'The field %(field)s points at other items, so it does not take a plain value. Pick the items it points at instead.',
			DOMAIN,
		),
		field_taken: __(
			'A field with that name already exists on the items this group reaches. Pick another name.',
			DOMAIN,
		),
		field_unknown: __(
			'This content type has no field called %(field)s. Check the name, or declare the field on the type first.',
			DOMAIN,
		),
		file_content_mismatch: __(
			'This file is named %(extension)s but it holds %(detected)s content. Upload it under a name that matches what is inside it.',
			DOMAIN,
		),
		file_too_large: __('This file is larger than the upload limit. Upload a smaller file.', DOMAIN),
		file_type_not_allowed: __(
			'Gophenberg does not accept %(extension)s files. Upload a picture, a video, an audio file, a PDF or a zip instead.',
			DOMAIN,
		),
		group_not_found: __(
			'That field group no longer exists. Reload the page to see the groups your site holds now.',
			DOMAIN,
		),
		group_order_incomplete: __(
			'The new order has to name every field group exactly once. Reload the page and move the groups again.',
			DOMAIN,
		),
		group_title_required: __('Every field group needs a title. Give it a name and save again.', DOMAIN),
		image_frame_too_large: __(
			'One frame of this animation holds more than %(max)d pixels, which is more than Gophenberg opens. Upload a smaller animation.',
			DOMAIN,
		),
		image_pixel_budget_exceeded: __(
			'This picture is %(width)d by %(height)d pixels, over the limit of %(max)d pixels in all. Make it smaller and upload it again.',
			DOMAIN,
		),
		image_unreadable: __(
			'This picture could not be read, so nothing was stored. The file may be damaged, so open it on your computer and upload it again.',
			DOMAIN,
		),
		internal: __(
			'Something went wrong inside Gophenberg. Try again in a moment, and tell whoever looks after your site if it keeps happening.',
			DOMAIN,
		),
		kit_missing: __(
			'The %(file)s inside %(name)s names its theme kit as %(declared)s, which is not a version number. Build the theme again, and the build will fill that in.',
			DOMAIN,
		),
		kit_unsupported: __(
			'The theme %(name)s was built on theme kit %(declared)s, and this site serves %(served)s. Build the theme again against a kit this site serves, then install it.',
			DOMAIN,
		),
		list_parameters_invalid: __(
			'One of the filters on this list is not one Gophenberg understands. Clear the filters and try again.',
			DOMAIN,
		),
		locale_unknown: __('Gophenberg does not answer in %(value)s. Pick one of the languages on the list.', DOMAIN),
		manifest_malformed: __(
			'The %(file)s inside %(name)s is not valid JSON, so it could not be read. Fix that file and package the theme again.',
			DOMAIN,
		),
		manifest_missing: __(
			'The theme %(name)s carries no %(file)s, so Gophenberg cannot tell what it is. Put that file at the top level of the zip and package the theme again.',
			DOMAIN,
		),
		manifest_name_mismatch: __(
			'The %(file)s calls this theme %(declared)s, but it is being installed as %(installed)s. Rename the zip file to match, or change the name inside %(file)s.',
			DOMAIN,
		),
		media_directory_unset: __(
			'This site has nowhere to store uploads, so nothing can be uploaded. Ask whoever looks after your site to point %(setting)s at a folder the server can write to.',
			DOMAIN,
		),
		media_id_malformed: __(
			'That web address does not name a media item Gophenberg can look up. Open the media library and pick the item from there.',
			DOMAIN,
		),
		media_not_found: __(
			'This media item is not in the library, and someone may have deleted it. Reload the page to see what is there now.',
			DOMAIN,
		),
		media_stale_update: __(
			'Someone else changed this media item while you were editing it. Reload the page and make your change again.',
			DOMAIN,
		),
		not_the_author: __(
			'Another account wrote this, and only its author, an editor or an administrator may change it. You can still read it.',
			DOMAIN,
		),
		nesting_limit_reached: __(
			'Content nests at most %(max)d levels deep, and this item would sit deeper. Pick a parent closer to the top.',
			DOMAIN,
		),
		page_kind_unknown: __('There is no page kind called %(value)s. Pick one of these instead: %(allowed)s.', DOMAIN),
		parent_cycle: __(
			'An item cannot sit inside itself, or inside one of the items nested under it. Pick a parent outside this branch.',
			DOMAIN,
		),
		parent_id_malformed: __(
			'Gophenberg could not read which item you picked as parent. Reload the page and pick it again.',
			DOMAIN,
		),
		param_taken: __(
			'Two rule sources are registered under the name %(param)s. A plugin is declaring a source your site already holds.',
			DOMAIN,
		),
		parent_not_found: __(
			'The item you picked as parent no longer exists. Reload the page and pick another parent.',
			DOMAIN,
		),
		parent_trashed: __('The item you picked as parent is in the trash. Restore it, or pick another parent.', DOMAIN),
		parent_wrong_type: __(
			'The item you picked as parent holds another kind of content. Pick a parent of the same content type as this item.',
			DOMAIN,
		),
		relation_target_key_malformed: __(
			'The content type this field points at is not named in a shape Gophenberg accepts. Give it a key that starts with a lowercase letter and then holds only lowercase letters, numbers and hyphens.',
			DOMAIN,
		),
		relation_target_required: __(
			'A relation field has to say which content type it points at. Pick that type and try again.',
			DOMAIN,
		),
		relation_target_unknown: __(
			'There is no content type called %(value)s. Pick a type your site holds, or register it first.',
			DOMAIN,
		),
		restore_not_trashed: __(
			'This item is not in the trash, so there is nothing to restore. Open it from the list to work on it as it is.',
			DOMAIN,
		),
		revision_cap_invalid: __(
			'The number of revisions to keep has to be a whole number, and it cannot be negative. Enter a different number.',
			DOMAIN,
		),
		revision_id_malformed: __(
			'That web address does not name a revision Gophenberg can look up. Open the item and pick a revision from its history.',
			DOMAIN,
		),
		revision_not_found: __(
			'That revision does not exist, and older ones drop off once a type holds more than it keeps. Open the item to see which revisions are left.',
			DOMAIN,
		),
		rollback_unavailable: __(
			'There is nothing to roll back to, because your site has not been through a theme change yet. Install a theme first, then a rollback has somewhere to go.',
			DOMAIN,
		),
		rule_any_negated: __(
			'A rule matching any %(source)s cannot be turned into an exclusion. Leave it as is, or name the ones to leave out instead.',
			DOMAIN,
		),
		rule_operator: __(
			'The source %(source)s does not offer %(operator)s. Pick one of the comparisons it does offer.',
			DOMAIN,
		),
		rule_source_unknown: __(
			'This rule reads a source called %(source)s, and nothing on your site declares one. A plugin that added it may have been removed.',
			DOMAIN,
		),
		rule_value_missing: __(
			'The rule on %(source)s names nothing to match. Pick a value, or remove the rule.',
			DOMAIN,
		),
		root_taken: __(
			'Another content type already lives at the root of your site, and only one can. Give this type a route word, or hand the root over with Make default.',
			DOMAIN,
		),
		route_not_found: __('There is nothing at this address. Check the link, or go back to the dashboard.', DOMAIN),
		route_word_malformed: __(
			'A route word has to start with a lowercase letter and then hold only lowercase letters, numbers and hyphens. Change it and try again.',
			DOMAIN,
		),
		route_word_required: __(
			'Only the content type at the root of your site may go without a route word. Give this type one and try again.',
			DOMAIN,
		),
		route_word_reserved: __(
			'Gophenberg keeps this word for itself, so no content type can answer under it. Pick a different route word.',
			DOMAIN,
		),
		route_word_taken: __('Another content type already answers under this route word. Pick a different word.', DOMAIN),
		server_entry_missing: __(
			'The theme %(name)s does not carry %(path)s as a file, so there is nothing to run. Package the built theme rather than its source.',
			DOMAIN,
		),
		setting_bounds: __(
			'The setting %(setting)s falls outside the others. Keep min below max and the default inside the bounds.',
			DOMAIN,
		),
		setting_shape: __(
			'The setting %(setting)s cannot hold the value it was given. Check what this setting takes and try again.',
			DOMAIN,
		),
		setting_unknown: __(
			'A %(kind)s field does not take a setting called %(setting)s. Remove it and try again.',
			DOMAIN,
		),
		slug_taken: __(
			'Another item of this content type already uses that address. Pick a different slug and save again.',
			DOMAIN,
		),
		status_transition_not_allowed: __(
			'This item cannot go straight to that status from the one it holds now. Save it as a draft first, then try again.',
			DOMAIN,
		),
		status_unknown: __('There is no status called %(value)s. Pick one of these instead: %(allowed)s.', DOMAIN),
		symlink_present: __(
			'The theme %(name)s holds a link to another file at %(path)s, and a theme may only carry real files. Replace it and package the theme again.',
			DOMAIN,
		),
		target_is_self: __('An item cannot point at itself. Pick a different item for %(field)s.', DOMAIN),
		target_not_found: __('One of the items picked no longer exists. Reload the page and pick again.', DOMAIN),
		target_repeated: __('The same item was picked twice for %(field)s. Pick each item once.', DOMAIN),
		target_wrong_type: __(
			'One of the items picked for %(field)s is not the kind of content that field points at. Reload the page and pick from the list the field offers.',
			DOMAIN,
		),
		theme_active: __(
			'The theme %(name)s is serving your site right now, so it cannot be replaced. Deactivate it first, or install this copy under another name.',
			DOMAIN,
		),
		theme_name_malformed: __(
			'%(name)s cannot be used as a theme name. Name the zip file after the theme, with no dot at the start and no slashes in it.',
			DOMAIN,
		),
		theme_not_installed: __(
			'The theme %(name)s is the one chosen, but it is not installed, so your site is served by the built-in renderer. Upload the theme again, or choose another one.',
			DOMAIN,
		),
		theme_pinned: __(
			'Your server pins the site to the theme %(name)s, so themes cannot be changed here. Ask whoever looks after your site to remove the pin.',
			DOMAIN,
		),
		theme_start_failed: __(
			'The theme %(name)s did not start, so your site is still served by the theme it had. Ask whoever looks after your site to check the server log for what the theme reported.',
			DOMAIN,
		),
		theme_too_large: __(
			'This theme is over the limit of %(max)d bytes, packed or unpacked. Leave out what the theme does not need and package it again.',
			DOMAIN,
		),
		theme_unreadable: __(
			'The theme %(name)s could not be read, so it cannot serve your site. Upload it again, or choose another theme.',
			DOMAIN,
		),
		themes_directory_unset: __(
			'This site has nowhere to keep themes, so no theme can be installed. Ask whoever looks after your site to point %(setting)s at a folder the server can write to.',
			DOMAIN,
		),
		too_many_targets: __('The field %(field)s points at one item only. Remove the extra choices and keep one.', DOMAIN),
		type_in_use: __(
			'This content type still holds content, so it cannot be deleted. Move or delete its items first.',
			DOMAIN,
		),
		type_inactive: __(
			'This content type is turned off, so its content is not served or edited. Activate it first.',
			DOMAIN,
		),
		type_key_malformed: __(
			'A type key has to start with a lowercase letter and then hold only lowercase letters, numbers and hyphens. Change the key and try again.',
			DOMAIN,
		),
		type_key_taken: __('Another content type already uses this key. Give this type a different name.', DOMAIN),
		type_label_required: __(
			'A content type needs a singular and a plural name, like Guide and Guides. Fill both in and try again.',
			DOMAIN,
		),
		type_not_found: __(
			'This content type does not exist any more. Reload the page to see the types your site holds now.',
			DOMAIN,
		),
		type_not_hierarchical: __(
			'This content type does not nest, so one of its items cannot sit inside another. Mark the type as Nests first.',
			DOMAIN,
		),
		type_targeted: __(
			'The field %(field)s in %(group)s still points at this content type. Delete that field first, then delete this type.',
			DOMAIN,
		),
		type_unknown: __(
			'There is no content type called %(value)s. Pick one of the types your site holds and try again.',
			DOMAIN,
		),
		upload_file_required: __('No file arrived with this upload. Choose a file and try again.', DOMAIN),
		upload_theme_required: __('No theme arrived with this upload. Choose a zip file and try again.', DOMAIN),
	}
}
