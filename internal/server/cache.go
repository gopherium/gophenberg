// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"time"
)

// DefaultAssetCacheMaxAge is how long a client may keep a site asset when nothing else applies.
const DefaultAssetCacheMaxAge = time.Hour

// DefaultMediaCacheMaxAge is how long a client may keep a served upload when nothing else applies.
const DefaultMediaCacheMaxAge = time.Hour

// DefaultContentSharedMaxAge is how long a shared cache may serve a public read by default.
const DefaultContentSharedMaxAge = time.Minute

// DefaultContentStaleWhileRevalidate is how long a shared cache may serve a stale read by default.
const DefaultContentStaleWhileRevalidate = 5 * time.Minute

// readerCacheControl is what an answer resolved for one reader carries, so no cache it does not belong to holds it.
const readerCacheControl = "private, no-store"

// CachePolicy is how long each kind of public answer may be kept. Zero applies the default.
type CachePolicy struct {
	// AssetMaxAge is how long a client may keep a site asset.
	AssetMaxAge time.Duration
	// MediaMaxAge is how long a client may keep a served upload.
	MediaMaxAge time.Duration
	// ContentSharedMaxAge is how long a shared cache may serve a public read.
	ContentSharedMaxAge time.Duration
	// ContentStaleWhileRevalidate is how long a shared cache may serve a stale read.
	ContentStaleWhileRevalidate time.Duration
}

// cacheHeaders are the Cache-Control values each kind of public answer carries.
type cacheHeaders struct {
	asset   string
	media   string
	content string
}

// headersFor returns the Cache-Control values the policy asks for, each default standing in for a zero.
func headersFor(policy CachePolicy) cacheHeaders {
	return cacheHeaders{
		asset: fmt.Sprintf("public, max-age=%d", wholeSeconds(policy.AssetMaxAge, DefaultAssetCacheMaxAge)),
		media: fmt.Sprintf("public, max-age=%d", wholeSeconds(policy.MediaMaxAge, DefaultMediaCacheMaxAge)),
		content: fmt.Sprintf("public, s-maxage=%d, stale-while-revalidate=%d",
			wholeSeconds(policy.ContentSharedMaxAge, DefaultContentSharedMaxAge),
			wholeSeconds(policy.ContentStaleWhileRevalidate, DefaultContentStaleWhileRevalidate)),
	}
}

// wholeSeconds returns the seconds the window holds, or the fallback when it holds none.
func wholeSeconds(window, fallback time.Duration) int64 {
	if window <= 0 {
		window = fallback
	}
	return int64(window / time.Second)
}
