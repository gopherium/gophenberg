// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"net/http"
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

// uncachedControl is what a failed answer carries, so no cache keeps it.
const uncachedControl = "no-store"

// cacheStamp is a writer stamping the Cache-Control an answer earns once its status is known.
type cacheStamp struct {
	http.ResponseWriter
	header  string
	stamped bool
}

// WriteHeader stamps the window on a success and no-store on a failure, then sends the status.
func (c *cacheStamp) WriteHeader(status int) {
	if !c.stamped {
		c.stamped = true
		control := c.header
		if status >= http.StatusBadRequest {
			control = uncachedControl
		}
		c.Header().Set("Cache-Control", control)
	}
	c.ResponseWriter.WriteHeader(status)
}

// Write sends the body, stamping a success first when no status went out.
func (c *cacheStamp) Write(body []byte) (int, error) {
	if !c.stamped {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(body)
}

// cacheStamped returns next answering through a writer that stamps the header the answer earns.
func cacheStamped(next http.Handler, header string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&cacheStamp{ResponseWriter: w, header: header}, r)
	})
}

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
