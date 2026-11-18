// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// stampWithNanoseconds is a stamp finer than the microsecond the file keeps, in a zone other than UTC.
var stampWithNanoseconds = time.Date(2026, time.March, 4, 10, 0, 0, 123456789, time.FixedZone("UTC+2", 2*60*60))

func TestAStampWithNanosecondsMatchesItselfOnceStored(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	trim := contenttest.DeclaredInside(t, site.types, specs, "trim", content.FieldKindText)
	mustExec(t, site.handle, "UPDATE core_content_fields SET updated_at = ?", dbkit.Time{Time: stampWithNanoseconds})
	car, err := site.types.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey(car) error = %v, want nil", err)
	}
	next := stampWithNanoseconds.Add(time.Hour)
	car.UpdatedAt, specs.UpdatedAt, trim.UpdatedAt = next, next, next

	for _, write := range []struct {
		name  string
		write func() (time.Time, error)
	}{
		{"the type", func() (time.Time, error) {
			stored, err := site.types.Update(t.Context(), car)
			return stored.UpdatedAt, err
		}},
		{"the top field over the stamp it read", func() (time.Time, error) {
			stored, err := site.types.UpdateFieldInGroup(t.Context(), specs.GroupID, specs, stampWithNanoseconds, nil)
			return stored.UpdatedAt, err
		}},
		{"the sub field over the stamp it read", func() (time.Time, error) {
			stored, err := site.types.UpdateSubField(t.Context(), trim.ID, trim, stampWithNanoseconds)
			return stored.UpdatedAt, err
		}},
	} {
		stored, err := write.write()

		want := next.Truncate(time.Microsecond).UTC()
		if err != nil || !stored.Equal(want) || stored.Location() != time.UTC {
			t.Errorf("writing %s = %v, %v, want %v, cut to the microsecond in UTC", write.name, stored, err, want)
		}
	}
}

func TestUpdateGroupWritesNothingOnceTheRowMovedPastTheStampItHolds(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	read := contenttest.GroupOn(t, site.types, "Cars", "car")
	mustExec(t, site.handle, "UPDATE core_field_groups SET title = 'Motors', updated_at = ? WHERE id = ?",
		dbkit.Time{Time: read.UpdatedAt.Add(time.Second)}, read.ID)
	read.Title = "Vehicles"

	_, err := site.types.UpdateGroup(t.Context(), read, nil, nil)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrConflict)
	}
	if held := contenttest.GroupAt(t, site.types, read.ID); held.Title != "Motors" {
		t.Errorf("title = %q, want the row the other write left", held.Title)
	}
}

func TestSiteDefinitionsStoreNoOriginInEachTable(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	specs := contenttest.DeclareSection(t, site.types, "specs")
	trim := contenttest.DeclaredInside(t, site.types, specs, "trim", content.FieldKindText)

	for _, row := range []struct {
		name, lookup string
		key          any
	}{
		{"the type", "SELECT origin IS NULL FROM core_content_types WHERE key = ?", "car"},
		{"the group", "SELECT origin IS NULL FROM core_field_groups WHERE id = ?", specs.GroupID},
		{"the field", "SELECT origin IS NULL FROM core_content_fields WHERE id = ?", specs.ID},
		{"the sub field", "SELECT origin IS NULL FROM core_content_fields WHERE id = ?", trim.ID},
	} {
		if none := count(t, site.handle, row.lookup, row.key); none != 1 {
			t.Errorf("origin IS NULL on %s = %d, want 1 for a definition the site made", row.name, none)
		}
	}
}

func TestAdoptingAGroupLeavesThePluginOriginOfAnotherGroupsFields(t *testing.T) {
	t.Parallel()

	site := newTypedSite(t)
	contenttest.StoreType(t, site.types, "car")
	declared := map[string]int{}
	for group, key := range map[string]string{"event-details": "starts", "venue-details": "address"} {
		stored, err := site.types.CreateGroup(t.Context(), content.Group{
			Key: group, Title: group, Location: contenttest.LocationOf("car"), Origin: "events",
		})
		if err != nil {
			t.Fatalf("CreateGroup(%s) error = %v, want nil", group, err)
		}
		field := contenttest.FieldOn(t, "", key, content.FieldKindText, "")
		field.Origin = "events"
		declared[group] = fieldInGroup(t, site.types, stored.ID, field).ID
	}

	if err := site.types.AdoptGroup(t.Context(), "event-details"); err != nil {
		t.Fatalf("AdoptGroup(event-details) error = %v, want nil", err)
	}

	lookup := "SELECT origin IS NULL FROM core_content_fields WHERE id = ?"
	for key, want := range map[string]int64{"event-details": 1, "venue-details": 0} {
		if none := count(t, site.handle, lookup, declared[key]); none != want {
			t.Errorf("origin IS NULL on the field of %s = %d, want %d", key, none, want)
		}
	}
}
