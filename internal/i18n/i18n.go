// SPDX-License-Identifier: Apache-2.0

// Package i18n answers the words and dates a language reads on the public site.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"time"
)

// DefaultLocale is the language the sources are written in.
const DefaultLocale = "en-US"

// Msgid marks a message for extraction and returns it unchanged.
func Msgid(s string) string { return s }

// datePattern is the order a date reads in, taking the day, the month and the year.
var datePattern = Msgid("%[1]d %[2]s %[3]d")

// months holds each month's name in the order the calendar counts them.
var months = [...]string{
	Msgid("January"), Msgid("February"), Msgid("March"), Msgid("April"),
	Msgid("May"), Msgid("June"), Msgid("July"), Msgid("August"),
	Msgid("September"), Msgid("October"), Msgid("November"), Msgid("December"),
}

//go:embed catalogs/*.json
var catalogs embed.FS

// Translator answers the words one language reads.
type Translator struct {
	messages map[string]string
}

// loaded holds the translator each language was read into.
var loaded sync.Map

// For returns the translator a language reads with.
func For(locale string) *Translator {
	if held, ok := loaded.Load(locale); ok {
		return held.(*Translator) //nolint:forcetypeassert // only translators are stored.
	}
	held := read(locale)
	loaded.Store(locale, held)
	return held
}

// read returns the translator a language's shipped catalog compiles to.
func read(locale string) *Translator {
	raw, err := catalogs.ReadFile(path.Join("catalogs", locale+".json"))
	if err != nil {
		return &Translator{messages: map[string]string{}}
	}
	return parse(raw)
}

// parse returns the translator a compiled catalog's bytes hold.
func parse(raw []byte) *Translator {
	held := &Translator{messages: map[string]string{}}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return held
	}
	for key, entry := range entries {
		var msgstr []string
		if err := json.Unmarshal(entry, &msgstr); err != nil {
			continue
		}
		if len(msgstr) > 0 && msgstr[0] != "" {
			held.messages[key] = msgstr[0]
		}
	}
	return held
}

// Get returns the message in the translator's language, the source word when it carries none.
func (t *Translator) Get(msgid string) string {
	if held, ok := t.messages[msgid]; ok {
		return held
	}
	return msgid
}

// Date returns the date as the translator's language writes it.
func (t *Translator) Date(at time.Time) string {
	return fmt.Sprintf(t.Get(datePattern), at.Day(), t.Get(months[at.Month()-1]), at.Year())
}
