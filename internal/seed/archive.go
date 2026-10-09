// SPDX-License-Identifier: Apache-2.0

package seed

import "github.com/gopherium/gophenberg/internal/content"

// archiveDaySpan is how many days apart the archive dates its posts, the first one that many days before the seeding.
const archiveDaySpan = 18

// archiveEntry is one post of the archive, before its identity, words and date are filled in.
type archiveEntry struct {
	title    string
	excerpt  string
	status   content.Status
	author   string
	featured bool
}

// archiveEntries returns the posts the archive spreads over the eighteen months before the seeding, newest first.
func archiveEntries() []archiveEntry {
	return []archiveEntry{
		{"Planning the Spring Release", "What the next release holds.", content.StatusPublished, AdminEmail, true},
		{"How We Review Pull Requests", "Two readers for every change.", content.StatusPublished, EditorEmail, false},
		{"Keyboard Shortcuts Worth Learning", "Five keys that save hours.", content.StatusPublished, AuthorEmail, true},
		{"Draft Ideas for the Newsletter", "Topics to pick from next month.", content.StatusDraft, AuthorEmail, false},
		{"Choosing a Theme for a Small Site", "Fewer choices, better pages.", content.StatusPublished, EditorEmail, false},
		{"Quiet Hours in the Office", "A note for the team only.", content.StatusPrivate, AdminEmail, false},
		{"Release Notes for the Winter Update", "Every change in one place.", content.StatusPublished, AdminEmail, false},
		{"Tables Without Tears", "Laying out numbers people can read.", content.StatusPublished, AuthorEmail, false},
		{"A Second Look at Image Sizes", "Waiting for a review of the numbers.", content.StatusPending, EditorEmail, false},
		{"Moving a Blog in One Afternoon", "A checklist for the move.", content.StatusPublished, EditorEmail, true},
		{"Lessons from the First Hundred Posts", "What the archive taught us.", content.StatusPublished, AuthorEmail, false},
		{"Questions Readers Ask Most", "Answers still being written.", content.StatusDraft, EditorEmail, false},
		{"Writing Headlines That Age Well", "Titles that read well for years.", content.StatusPublished, AdminEmail, false},
		{"Backups You Can Actually Restore", "Practice restoring them first.", content.StatusPublished, EditorEmail, true},
		{"Interview Notes, Unedited", "Raw notes kept for the writer.", content.StatusPrivate, AuthorEmail, false},
		{"Accessible Color Choices", "Contrast that works for every reader.", content.StatusPublished, AuthorEmail, false},
		{"Year in Review", "Twelve months in twelve paragraphs.", content.StatusPublished, AdminEmail, false},
		{"Unfinished Thoughts on Comments", "Not sure where this one goes yet.", content.StatusDraft, AdminEmail, false},
		{"Garden Club Meeting Recap", "Seeds, soil and a long tea break.", content.StatusPublished, EditorEmail, false},
		{"Old Announcement Kept for Reference", "Superseded by a newer post.", content.StatusTrash, AuthorEmail, false},
		{"Zero Downtime Updates", "Updating the site while readers stay.", content.StatusPublished, AdminEmail, true},
		{"Notes from the Community Call", "Who said what and what comes next.", content.StatusPublished, AuthorEmail, false},
		{"Proposal for a Style Guide", "One voice across every page.", content.StatusPending, AuthorEmail, false},
		{"Every Block Explained", "A tour of the blocks the editor offers.", content.StatusPublished, EditorEmail, false},
		{"Mobile Editing on a Train", "Writing between two stations.", content.StatusPublished, AuthorEmail, false},
		{"Weekly Digest Template", "The skeleton of the weekly email.", content.StatusDraft, EditorEmail, false},
		{"Search Tips for Long Archives", "Finding the post you half remember.", content.StatusPublished, AdminEmail, false},
		{"Volunteer Thank You Note", "Thanks to everyone who helped this year.", content.StatusPublished, EditorEmail, false},
		{"Event Photos Needing Captions", "Pictures waiting for their words.", content.StatusPending, AdminEmail, false},
		{"Launch Day Checklist", "Everything to check before going live.", content.StatusPublished, AuthorEmail, false},
	}
}

// archivePosts returns the archive as scripted posts, each with a fixed identity and a date further back.
func archivePosts() []demoPost {
	entries := archiveEntries()
	posts := make([]demoPost, len(entries))
	for i, entry := range entries {
		posts[i] = demoPost{
			id:       archiveIDs[i],
			title:    entry.title,
			excerpt:  entry.excerpt,
			content:  "<!-- wp:paragraph -->\n<p>" + entry.excerpt + "</p>\n<!-- /wp:paragraph -->",
			status:   entry.status,
			author:   entry.author,
			daysAgo:  archiveDaySpan * (i + 1),
			featured: entry.featured,
		}
	}
	return posts
}

// archiveIDs holds the fixed identity of each archive post, in the order the archive lists them.
var archiveIDs = []string{
	"019fb000-0000-7000-8000-000000000101", "019fb000-0000-7000-8000-000000000102",
	"019fb000-0000-7000-8000-000000000103", "019fb000-0000-7000-8000-000000000104",
	"019fb000-0000-7000-8000-000000000105", "019fb000-0000-7000-8000-000000000106",
	"019fb000-0000-7000-8000-000000000107", "019fb000-0000-7000-8000-000000000108",
	"019fb000-0000-7000-8000-000000000109", "019fb000-0000-7000-8000-000000000110",
	"019fb000-0000-7000-8000-000000000111", "019fb000-0000-7000-8000-000000000112",
	"019fb000-0000-7000-8000-000000000113", "019fb000-0000-7000-8000-000000000114",
	"019fb000-0000-7000-8000-000000000115", "019fb000-0000-7000-8000-000000000116",
	"019fb000-0000-7000-8000-000000000117", "019fb000-0000-7000-8000-000000000118",
	"019fb000-0000-7000-8000-000000000119", "019fb000-0000-7000-8000-000000000120",
	"019fb000-0000-7000-8000-000000000121", "019fb000-0000-7000-8000-000000000122",
	"019fb000-0000-7000-8000-000000000123", "019fb000-0000-7000-8000-000000000124",
	"019fb000-0000-7000-8000-000000000125", "019fb000-0000-7000-8000-000000000126",
	"019fb000-0000-7000-8000-000000000127", "019fb000-0000-7000-8000-000000000128",
	"019fb000-0000-7000-8000-000000000129", "019fb000-0000-7000-8000-000000000130",
}
