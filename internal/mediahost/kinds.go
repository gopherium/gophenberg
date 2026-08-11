// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"net/http"
	"path"
	"slices"
	"strings"
)

// kind describes one allowed upload type.
type kind struct {
	mime   string
	ext    string
	image  bool
	strict bool
	extra  []string
}

// allowedKinds maps a file extension to the upload type it stands for. A kind
// whose signature the sniffer knows is strict, the rest tolerate unknown
// content because their container zoo defeats sniffing.
var allowedKinds = map[string]kind{
	"jpg":  {mime: "image/jpeg", ext: "jpg", image: true},
	"jpeg": {mime: "image/jpeg", ext: "jpg", image: true},
	"png":  {mime: "image/png", ext: "png", image: true},
	"gif":  {mime: "image/gif", ext: "gif", image: true},
	"webp": {mime: "image/webp", ext: "webp", image: true},
	"mp4":  {mime: "video/mp4", ext: "mp4"},
	"webm": {mime: "video/webm", ext: "webm"},
	"mp3":  {mime: "audio/mpeg", ext: "mp3"},
	"m4a":  {mime: "audio/mp4", ext: "m4a", extra: []string{"video/mp4"}},
	"wav":  {mime: "audio/wav", ext: "wav", extra: []string{"audio/wave"}},
	"ogg":  {mime: "audio/ogg", ext: "ogg", extra: []string{"application/ogg"}},
	"flac": {mime: "audio/flac", ext: "flac"},
	"pdf":  {mime: "application/pdf", ext: "pdf", strict: true},
	"zip":  {mime: "application/zip", ext: "zip", strict: true},
}

// detect returns the upload type name and data agree on, or the refusal they earn.
func detect(name string, data []byte) (kind, error) {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(baseName(name)), "."))
	k, allowed := allowedKinds[ext]
	if !allowed {
		return kind{}, refuse("the file type is not allowed", "extension %q is not in the allowed set", ext)
	}
	sniffed, _, _ := strings.Cut(http.DetectContentType(data), ";")
	if !k.accepts(strings.TrimSpace(sniffed)) {
		return kind{}, refuse("the content does not match", "%s content in a %q file", sniffed, ext)
	}
	return k, nil
}

// accepts reports whether sniffed content may live in a file of this kind.
func (k kind) accepts(sniffed string) bool {
	if sniffed == k.mime {
		return true
	}
	if k.image || k.strict {
		return false
	}
	return sniffed == "application/octet-stream" || slices.Contains(k.extra, sniffed)
}
