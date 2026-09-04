// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// downloadedGroup is one field group as the definitions download carries it.
type downloadedGroup struct {
	Title  string `json:"title"`
	Fields []struct {
		Key string `json:"key"`
	} `json:"fields"`
}

// theAdministratorDownloadsTheDefinitions asks for the site's definitions file.
func theAdministratorDownloadsTheDefinitions(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get("/api/definitions"); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	if got := w.answer.header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		return fmt.Errorf("Content-Disposition = %q, want an attachment", got)
	}
	return nil
}

// downloadedGroups returns the groups the last download carries.
func downloadedGroups(w *world) ([]downloadedGroup, error) {
	var envelope struct {
		Groups []downloadedGroup `json:"groups"`
	}
	if err := w.answer.decode(&envelope); err != nil {
		return nil, err
	}
	return envelope.Groups, nil
}

// theDownloadHoldsTheGroupWithTheField asserts the download carries the group and the field inside it.
func theDownloadHoldsTheGroupWithTheField(ctx context.Context, title, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groups, err := downloadedGroups(w)
	if err != nil {
		return err
	}
	for _, held := range groups {
		if held.Title != title {
			continue
		}
		for _, f := range held.Fields {
			if f.Key == key {
				return nil
			}
		}
		return fmt.Errorf("the downloaded group %q holds no field %q", title, key)
	}
	return fmt.Errorf("the download holds no group %q", title)
}

// theAdministratorPlansTheFileTheSiteExports downloads the site's definitions and plans them straight back.
func theAdministratorPlansTheFileTheSiteExports(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.get("/api/definitions"); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	return w.postJSON("/api/definitions/plan", string(w.answer.body))
}

// theAdministratorPlansAFileWrittenInFormat plans an otherwise empty definitions file under the named format.
func theAdministratorPlansAFileWrittenInFormat(ctx context.Context, format string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.postJSON("/api/definitions/plan", `{"format":"`+format+`","types":[],"groups":[]}`)
}

// thePlanHoldsNoChanges asserts the plan asks for nothing at all.
func thePlanHoldsNoChanges(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	var planned struct {
		Changes  []map[string]any `json:"changes"`
		Warnings []map[string]any `json:"warnings"`
	}
	if err := w.answer.decode(&planned); err != nil {
		return err
	}
	if len(planned.Changes) != 0 || len(planned.Warnings) != 0 {
		return fmt.Errorf("the plan holds %v and warns %v, want nothing at all", planned.Changes, planned.Warnings)
	}
	return nil
}

// theDownloadLeavesOutTheGroup asserts the download carries no group under the title.
func theDownloadLeavesOutTheGroup(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	groups, err := downloadedGroups(w)
	if err != nil {
		return err
	}
	for _, held := range groups {
		if held.Title == title {
			return fmt.Errorf("the download holds the group %q, want it left out", title)
		}
	}
	return nil
}
