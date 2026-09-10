// SPDX-License-Identifier: Apache-2.0

// Package served shapes a stored item the way every public seam answers it.
package served

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
)

// MediaPrefix is the URL prefix uploads are served under.
const MediaPrefix = "/media"

// Target is one item a relation points at, as a public reader sees it.
type Target struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// Pointer is one item pointing at another, named as a relation target and typed besides.
type Pointer struct {
	Target
	Type string `json:"type"`
}

// Rendition is one stored rendition as a public reader sees it.
type Rendition struct {
	Src      string `json:"src"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	MimeType string `json:"mime_type"`
}

// File is one library file as a public reader sees it.
type File struct {
	ID       int64                `json:"id"`
	Src      string               `json:"src"`
	Title    string               `json:"title"`
	AltText  string               `json:"alt_text"`
	Caption  string               `json:"caption"`
	MimeType string               `json:"mime_type"`
	Width    int                  `json:"width"`
	Height   int                  `json:"height"`
	Sizes    map[string]Rendition `json:"sizes"`
}

// Library reads the stored files a media value names.
type Library interface {
	ByIDs(ctx context.Context, ids []int64) ([]media.Media, error)
}

// Groups reads the field groups a backlinks source resolves through.
type Groups interface {
	Groups(ctx context.Context) ([]content.Group, error)
	Params(ctx context.Context) *content.ParamRegistry
}

// NamedTargets returns the targets a public answer carries under one relation field.
func NamedTargets(held []content.Target) []Target {
	named := make([]Target, len(held))
	for i, target := range held {
		named[i] = Target{ID: target.ID.String(), Title: target.Title, Path: target.Path}
	}
	return named
}

// NamedPointers returns the pointing items a public answer carries under one backlinks field.
func NamedPointers(held []content.Pointer) []Pointer {
	named := make([]Pointer, len(held))
	for i, pointer := range held {
		named[i] = Pointer{
			Target: Target{ID: pointer.ID.String(), Title: pointer.Title, Path: pointer.Path},
			Type:   pointer.Type,
		}
	}
	return named
}

// FileOf returns the public view of one stored file.
func FileOf(m media.Media) File {
	sizes := make(map[string]Rendition, len(m.Sizes))
	for slug, held := range m.Sizes {
		sizes[slug] = Rendition{
			Src:      MediaPrefix + "/" + held.File,
			Width:    held.Width,
			Height:   held.Height,
			MimeType: held.MimeType,
		}
	}
	return File{
		ID:       m.ID,
		Src:      MediaPrefix + "/" + m.File,
		Title:    m.Title,
		AltText:  m.AltText,
		Caption:  m.Caption,
		MimeType: m.MimeType,
		Width:    m.Width,
		Height:   m.Height,
		Sizes:    sizes,
	}
}
