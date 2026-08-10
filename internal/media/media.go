// SPDX-License-Identifier: Apache-2.0

// Package media defines the files the media library holds.
package media

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidFile reports that a media item names no stored file.
var ErrInvalidFile = errors.New("media: invalid file")

// ErrInvalidMime reports that a media item carries no content type.
var ErrInvalidMime = errors.New("media: invalid mime type")

// ErrInvalidAuthor reports that a media item carries no author.
var ErrInvalidAuthor = errors.New("media: invalid author")

// ErrNotFound reports that no media item exists for the requested ID.
var ErrNotFound = errors.New("media: not found")

// Type sorts a media item into the kinds the library tells apart.
type Type string

// The kinds of media the library tells apart.
const (
	TypeImage Type = "image"
	TypeFile  Type = "file"
)

// Rendition is one derived copy of an image bounded to a size.
type Rendition struct {
	File     string `json:"file"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	MimeType string `json:"mime_type"`
	Filesize int64  `json:"filesize"`
}

// RenditionMap holds a media item's renditions keyed by size slug.
type RenditionMap map[string]Rendition

// Media is a stored upload with the descriptions the editor reads.
type Media struct {
	ID          int64
	Type        Type
	File        string
	Title       string
	AltText     string
	Caption     string
	Description string
	MimeType    string
	Width       int
	Height      int
	Filesize    int64
	Sizes       RenditionMap
	AuthorID    uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// New returns a [Media] item over a stored file, typed after its content type.
func New(file, title, mimeType string, authorID uuid.UUID) (Media, error) {
	if strings.TrimSpace(file) == "" {
		return Media{}, ErrInvalidFile
	}
	if strings.TrimSpace(mimeType) == "" {
		return Media{}, ErrInvalidMime
	}
	if authorID == uuid.Nil {
		return Media{}, ErrInvalidAuthor
	}
	now := time.Now().UTC()
	return Media{
		Type:      typeOf(mimeType),
		File:      file,
		Title:     title,
		MimeType:  mimeType,
		Sizes:     RenditionMap{},
		AuthorID:  authorID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// typeOf returns the kind of media a content type stands for.
func typeOf(mimeType string) Type {
	if strings.HasPrefix(mimeType, "image/") {
		return TypeImage
	}
	return TypeFile
}
