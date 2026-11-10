// SPDX-License-Identifier: Apache-2.0

package feed_test

import (
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/sdk"
)

func TestTheFeedLinksItsChannelFromTheHeadOfEveryPage(t *testing.T) {
	t.Parallel()

	contributor, ok := mustRegister(t, &stubPosts{}, nil).(sdk.HeadContributor)
	if !ok {
		t.Fatal("the feed plugin adds no head tags, want it to link its channel")
	}

	page := sdk.PageMeta{Kind: sdk.PageItem, Path: "/a-post", SiteTitle: "Field Notes"}

	got, err := contributor.HeadTags(t.Context(), page)

	want := []sdk.HeadTag{{Tag: "link", Attrs: []sdk.Attr{
		{Key: "rel", Value: "alternate"}, {Key: "type", Value: "application/rss+xml"},
		{Key: "title", Value: "Field Notes"}, {Key: "href", Value: "/api/plugins/feed/rss.xml"},
	}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("HeadTags() = %+v, %v, want %+v", got, err, want)
	}
}
