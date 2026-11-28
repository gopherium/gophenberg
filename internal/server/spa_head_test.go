// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gopherium/gophenberg/internal/server"
)

func TestAdminPageCarriesTheLimitInTheHeadOfUnusualPages(t *testing.T) {
	t.Parallel()

	meta := dialogLimitMeta("1000")
	mark := string(rune(0xFEFF))
	cases := []struct {
		name, page, want string
	}{
		{
			name: "a head written inside a title",
			page: `<!doctype html><title><head></title><head></head>`,
			want: `<!doctype html>` + meta + `<title><head></title><head></head>`,
		},
		{
			name: "a head written inside a script",
			page: `<!doctype html><script>let tag = "<head>"</script><head></head>`,
			want: `<!doctype html>` + meta + `<script>let tag = "<head>"</script><head></head>`,
		},
		{
			name: "a head written inside a template",
			page: `<!doctype html><template><head/></template><title>Gophenberg</title>`,
			want: `<!doctype html>` + meta + `<template><head/></template><title>Gophenberg</title>`,
		},
		{
			name: "an html tag with attributes and no head",
			page: `<!doctype html><html lang="en"><title>Gophenberg</title>`,
			want: `<!doctype html><html lang="en">` + meta + `<title>Gophenberg</title>`,
		},
		{
			name: "a head written inside an attribute",
			page: `<!doctype html><html data-note="<head>"><head></head>`,
			want: `<!doctype html><html data-note="<head>"><head>` + meta + `</head>`,
		},
		{
			name: "a self-closing head",
			page: `<!doctype html><head/><title>Gophenberg</title>`,
			want: `<!doctype html><head/>` + meta + `<title>Gophenberg</title>`,
		},
		{
			name: "an empty comment closed at once",
			page: `<!doctype html><!--><head></head>`,
			want: `<!doctype html><!--><head>` + meta + `</head>`,
		},
		{
			name: "a processing instruction before the doctype",
			page: `<?xml version="1.0"?><!doctype html><title>Gophenberg</title>`,
			want: `<?xml version="1.0"?><!doctype html>` + meta + `<title>Gophenberg</title>`,
		},
		{
			name: "a doctype that never closes",
			page: `<!doctype html`,
			want: meta + `<!doctype html`,
		},
		{
			name: "a byte order mark before the doctype",
			page: mark + `<!doctype html><title>Gophenberg</title>`,
			want: mark + `<!doctype html>` + meta + `<title>Gophenberg</title>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := server.NewServer(server.Config{
				Users: newFakeUserStore(),
				Web:   fstest.MapFS{"index.html": {Data: []byte(tc.page)}},
				Lists: server.ListSettings{DialogWriteTimeout: time.Second},
			})
			recorder := doRequest(t, handler, http.MethodGet, "/admin/", "")

			if recorder.Code != http.StatusOK || recorder.Body.String() != tc.want {
				t.Errorf("GET /admin/ = %d %q, want %q", recorder.Code, recorder.Body.String(), tc.want)
			}
		})
	}
}
