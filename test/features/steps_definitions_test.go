// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
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

// exportedBySite returns the definitions the site downloads right now.
func exportedBySite(ctx context.Context) (*world, definitions.Envelope, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, definitions.Envelope{}, err
	}
	if err := w.get("/api/definitions"); err != nil {
		return nil, definitions.Envelope{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return nil, definitions.Envelope{}, err
	}
	var envelope definitions.Envelope
	if err := w.answer.decode(&envelope); err != nil {
		return nil, definitions.Envelope{}, err
	}
	return w, envelope, nil
}

// applying performs the import against the running site.
func applying(w *world, asked definitions.Import) error {
	if err := postingImport(w, asked); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// postingImport sends the import to the running site, whatever it answers.
func postingImport(w *world, asked definitions.Import) error {
	body, err := json.Marshal(asked)
	if err != nil {
		return fmt.Errorf("writing the import: %w", err)
	}
	return w.postJSON("/api/definitions/apply", string(body))
}

// theAdministratorAppliesAFileRenamingTheGroup applies the site's own definitions with one group retitled.
func theAdministratorAppliesAFileRenamingTheGroup(ctx context.Context, title, renamed string) error {
	w, envelope, err := exportedBySite(ctx)
	if err != nil {
		return err
	}
	for i := range envelope.Groups {
		if envelope.Groups[i].Title == title {
			envelope.Groups[i].Title = renamed
		}
	}
	return applying(w, definitions.Import{Envelope: envelope})
}

// theAdministratorAppliesAFileHoldingNoGroups applies the site's own definitions with every group left out.
func theAdministratorAppliesAFileHoldingNoGroups(ctx context.Context) error {
	w, envelope, err := exportedBySite(ctx)
	if err != nil {
		return err
	}
	envelope.Groups = nil
	return applying(w, definitions.Import{Envelope: envelope})
}

// theAdministratorAppliesAFileGivingUpTheGroup applies the file with every group left out and one loss confirmed.
func theAdministratorAppliesAFileGivingUpTheGroup(ctx context.Context, key string) error {
	w, envelope, err := exportedBySite(ctx)
	if err != nil {
		return err
	}
	envelope.Groups = nil
	return applying(w, definitions.Import{
		Envelope: envelope,
		Confirm:  []definitions.Confirmed{{Subject: "group", Key: key}},
	})
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

// siteFile returns the world holding the file the scenario builds, starting from what the site downloads.
func siteFile(ctx context.Context) (*world, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, err
	}
	if w.file != nil {
		return w, nil
	}
	_, envelope, err := exportedBySite(ctx)
	if err != nil {
		return nil, err
	}
	w.file = &definitions.Import{Envelope: envelope}
	return w, nil
}

// groupInFile returns the file's group the stored title names, or the one the file itself titles so.
func groupInFile(w *world, title string) (*definitions.GroupDefinition, error) {
	key := ""
	if stored, err := groupNamed(w, title); err == nil {
		key = stored.Key
	}
	for i := range w.file.Groups {
		held := &w.file.Groups[i]
		if (key != "" && held.Key == key) || held.Title == title {
			return held, nil
		}
	}
	return nil, fmt.Errorf("the file holds no group titled %q", title)
}

// fieldInFile returns the field the file's group holds under the key.
func fieldInFile(ctx context.Context, key, title string) (*definitions.FieldDefinition, error) {
	w, err := siteFile(ctx)
	if err != nil {
		return nil, err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return nil, err
	}
	for i := range group.Fields {
		if group.Fields[i].Key == key {
			return &group.Fields[i], nil
		}
	}
	return nil, fmt.Errorf("the file's group %q holds no field %q", title, key)
}

// theSitesFileLeavesOut takes one field out of the file's group.
func theSitesFileLeavesOut(ctx context.Context, key, title string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return err
	}
	held := len(group.Fields)
	group.Fields = slices.DeleteFunc(group.Fields, func(f definitions.FieldDefinition) bool {
		return f.Key == key
	})
	if len(group.Fields) == held {
		return fmt.Errorf("the file's group %q holds no field %q", title, key)
	}
	return nil
}

// theSitesFileMoves takes one field out of the file's group and stands it in a new group placed alike.
func theSitesFileMoves(ctx context.Context, key, from, title string) error {
	moved, err := fieldInFile(ctx, key, from)
	if err != nil {
		return err
	}
	carried := *moved
	if err := theSitesFileLeavesOut(ctx, key, from); err != nil {
		return err
	}
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	source, err := groupInFile(w, from)
	if err != nil {
		return err
	}
	w.file.Groups = append(w.file.Groups, definitions.GroupDefinition{
		Key: strings.ToLower(strings.ReplaceAll(title, " ", "-")), Title: title,
		Location: source.Location, Active: true, Fields: []definitions.FieldDefinition{carried},
	})
	return nil
}

// theSitesFileClearsTheConditionsOf takes the rules off one field of the file.
func theSitesFileClearsTheConditionsOf(ctx context.Context, key, title string) error {
	held, err := fieldInFile(ctx, key, title)
	if err != nil {
		return err
	}
	delete(held.Settings, content.SettingConditions)
	return nil
}

// theSitesFileTurnsInto gives one field of the file another kind.
func theSitesFileTurnsInto(ctx context.Context, key, title, kind string) error {
	held, err := fieldInFile(ctx, key, title)
	if err != nil {
		return err
	}
	held.Kind = kind
	return nil
}

// theSitesFileMakesHoldOne turns one relation of the file into a single link.
func theSitesFileMakesHoldOne(ctx context.Context, key, title string) error {
	held, err := fieldInFile(ctx, key, title)
	if err != nil {
		return err
	}
	held.Many = false
	return nil
}

// theSitesFileRetitles gives the file's group another title.
func theSitesFileRetitles(ctx context.Context, title, renamed string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return err
	}
	group.Title = renamed
	return nil
}

// theSitesFilePointsAt names another relation as the one a backlinks field of the file reads.
func theSitesFilePointsAt(ctx context.Context, key, title, source, sourceTitle string) error {
	held, err := fieldInFile(ctx, key, title)
	if err != nil {
		return err
	}
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, sourceTitle)
	if err != nil {
		return err
	}
	settings := maps.Clone(held.Settings)
	settings[content.SettingSourceGroup] = group.Key
	settings[content.SettingSourceField] = []any{source}
	held.Settings = settings
	return nil
}

// theSitesFileAddsTheFieldWithSettings stands a new field carrying the settings in the file's group.
func theSitesFileAddsTheFieldWithSettings(
	ctx context.Context, kind, key, title string, settings *godog.DocString,
) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return err
	}
	var held map[string]any
	if err := json.Unmarshal([]byte(settings.Content), &held); err != nil {
		return fmt.Errorf("reading the settings: %w", err)
	}
	group.Fields = append(group.Fields, definitions.FieldDefinition{Key: key, Label: key, Kind: kind, Settings: held})
	return nil
}

// theSitesFileLeavesOutTheType takes one type out of the file.
func theSitesFileLeavesOutTheType(ctx context.Context, key string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	held := len(w.file.Types)
	w.file.Types = slices.DeleteFunc(w.file.Types, func(d definitions.TypeDefinition) bool {
		return d.Key == key
	})
	if len(w.file.Types) == held {
		return fmt.Errorf("the file holds no type %q", key)
	}
	return nil
}

// theSitesFileLeavesOutTheGroup takes one group out of the file.
func theSitesFileLeavesOutTheGroup(ctx context.Context, title string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return err
	}
	key := group.Key
	w.file.Groups = slices.DeleteFunc(w.file.Groups, func(d definitions.GroupDefinition) bool {
		return d.Key == key
	})
	return nil
}

// theSitesFileMakesPointAt names another type as the one a relation of the file points at.
func theSitesFileMakesPointAt(ctx context.Context, key, title, typeKey string) error {
	held, err := fieldInFile(ctx, key, title)
	if err != nil {
		return err
	}
	held.RelatesTo = typeKey
	return nil
}

// theSitesFilePlacesOn places the file's group on the named type alone.
func theSitesFilePlacesOn(ctx context.Context, title, typeKey string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return err
	}
	group.Location = content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: typeKey}}}
	return nil
}

// theSitesFileStopsTheTypeNesting marks one type of the file as flat.
func theSitesFileStopsTheTypeNesting(ctx context.Context, key string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	for i := range w.file.Types {
		if w.file.Types[i].Key == key {
			w.file.Types[i].Hierarchical = false
			return nil
		}
	}
	return fmt.Errorf("the file holds no type %q", key)
}

// theAdministratorPlansTheFile asks what the file the scenario built would change, whatever the site answers.
func theAdministratorPlansTheFile(ctx context.Context) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(w.file.Envelope)
	if err != nil {
		return fmt.Errorf("writing the file: %w", err)
	}
	return w.postJSON("/api/definitions/plan", string(body))
}

// thePlanWarnsThatKeepsNesting asserts the plan warns the type keeps nesting.
func thePlanWarnsThatKeepsNesting(ctx context.Context, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	var planned definitions.Plan
	if err := w.answer.decode(&planned); err != nil {
		return err
	}
	for _, held := range planned.Warnings {
		if held.Code == "nesting_kept" && held.Key == key {
			return nil
		}
	}
	return fmt.Errorf("the plan warns %v, want %q kept nesting", planned.Warnings, key)
}

// theAdministratorConfirmsTheLossOf names one field of a stored group the import may take away.
func theAdministratorConfirmsTheLossOf(ctx context.Context, key, title string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	stored, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	w.file.Confirm = append(w.file.Confirm, definitions.Confirmed{
		Subject: definitions.SubjectField, Key: key, Group: stored.Key,
	})
	return nil
}

// theAdministratorConfirmsTheLossOfTheType names one stored type the import may take away.
func theAdministratorConfirmsTheLossOfTheType(ctx context.Context, key string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	w.file.Confirm = append(w.file.Confirm, definitions.Confirmed{Subject: definitions.SubjectType, Key: key})
	return nil
}

// theAdministratorConfirmsTheLossOfTheGroup names one stored group the import may take away whole.
func theAdministratorConfirmsTheLossOfTheGroup(ctx context.Context, title string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	stored, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	w.file.Confirm = append(w.file.Confirm, definitions.Confirmed{Subject: definitions.SubjectGroup, Key: stored.Key})
	return nil
}

// theAdministratorImportsTheFile applies the file the scenario built, whatever the site answers.
func theAdministratorImportsTheFile(ctx context.Context) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	if err := postingImport(w, *w.file); err != nil {
		return err
	}
	w.imported = w.answer
	return nil
}

// leftUndone returns what the last import said it left undone.
func leftUndone(w *world) ([]definitions.Change, error) {
	if w.imported == nil {
		return nil, fmt.Errorf("the scenario imported no file")
	}
	var outcome definitions.Outcome
	if err := w.imported.decode(&outcome); err != nil {
		return nil, err
	}
	return outcome.Skipped, nil
}

// theImportLeftTheGroupUndone asserts the last import names the file's group among what it left undone.
func theImportLeftTheGroupUndone(ctx context.Context, title string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return err
	}
	skipped, err := leftUndone(w)
	if err != nil {
		return err
	}
	for _, held := range skipped {
		if held.Subject == definitions.SubjectGroup && held.Key == group.Key {
			return nil
		}
	}
	return fmt.Errorf("the import left %+v undone, want the group %q among them", skipped, group.Key)
}

// theImportLeftTheFieldUndone asserts the last import names the field it would have added to the group as undone.
func theImportLeftTheFieldUndone(ctx context.Context, key, title string) error {
	w, err := siteFile(ctx)
	if err != nil {
		return err
	}
	group, err := groupInFile(w, title)
	if err != nil {
		return err
	}
	skipped, err := leftUndone(w)
	if err != nil {
		return err
	}
	for _, held := range skipped {
		if held.Subject == definitions.SubjectField && held.Key == key && held.Group == group.Key {
			return nil
		}
	}
	return fmt.Errorf("the import left %+v undone, want the field %q in %q among them", skipped, key, group.Key)
}

// theImportIsApplied asserts the site took the whole import.
func theImportIsApplied(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// noGroupIsTitled asserts the site holds no group under the title.
func noGroupIsTitled(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := listGroups(w)
	if err != nil {
		return err
	}
	for _, held := range listed.Items {
		if held.Title == title {
			return fmt.Errorf("a group is titled %q, want none", title)
		}
	}
	return nil
}

// theGroupHolds asserts the stored group carrying the title holds the field.
func theGroupHolds(ctx context.Context, title, key string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	stored, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	for _, f := range stored.Fields {
		if f.Key == key {
			return nil
		}
	}
	return fmt.Errorf("the group %q holds no field %q", title, key)
}

// initializeImportFile binds the steps building a changed copy of the site's file and importing it.
func initializeImportFile(sc *godog.ScenarioContext) {
	sc.Given(`^the site's file leaves out "([^"]*)" in "([^"]*)"$`, theSitesFileLeavesOut)
	sc.Given(`^the site's file moves "([^"]*)" from "([^"]*)" into a new group "([^"]*)"$`, theSitesFileMoves)
	sc.Given(`^the site's file clears the conditions of "([^"]*)" in "([^"]*)"$`, theSitesFileClearsTheConditionsOf)
	sc.Given(`^the site's file turns "([^"]*)" in "([^"]*)" into a "([^"]*)" field$`, theSitesFileTurnsInto)
	sc.Given(`^the site's file makes "([^"]*)" in "([^"]*)" hold one$`, theSitesFileMakesHoldOne)
	sc.Given(`^the site's file retitles "([^"]*)" as "([^"]*)"$`, theSitesFileRetitles)
	sc.Given(`^the site's file points "([^"]*)" in "([^"]*)" at "([^"]*)" in "([^"]*)"$`, theSitesFilePointsAt)
	sc.Given(`^the administrator confirms the loss of "([^"]*)" in "([^"]*)"$`, theAdministratorConfirmsTheLossOf)
	sc.Given(`^the administrator confirms the loss of the type "([^"]*)"$`, theAdministratorConfirmsTheLossOfTheType)
	sc.Given(`^the administrator confirms the loss of the group "([^"]*)"$`, theAdministratorConfirmsTheLossOfTheGroup)
	sc.Given(`^the site's file stops the type "([^"]*)" nesting$`, theSitesFileStopsTheTypeNesting)
	sc.Given(
		`^the site's file adds the "([^"]*)" field "([^"]*)" to "([^"]*)" with settings:$`,
		theSitesFileAddsTheFieldWithSettings,
	)
	sc.Given(`^the site's file leaves out the type "([^"]*)"$`, theSitesFileLeavesOutTheType)
	sc.Given(`^the site's file leaves out the group "([^"]*)"$`, theSitesFileLeavesOutTheGroup)
	sc.Given(`^the site's file makes "([^"]*)" in "([^"]*)" point at "([^"]*)"$`, theSitesFileMakesPointAt)
	sc.Given(`^the site's file places "([^"]*)" on "([^"]*)"$`, theSitesFilePlacesOn)
	sc.When(`^the administrator plans the file$`, theAdministratorPlansTheFile)
	sc.When(`^the administrator imports the file$`, theAdministratorImportsTheFile)
	sc.Then(`^the plan warns that "([^"]*)" keeps nesting$`, thePlanWarnsThatKeepsNesting)
	sc.Then(`^the import is applied$`, theImportIsApplied)
	sc.Then(`^the import left the group "([^"]*)" undone$`, theImportLeftTheGroupUndone)
	sc.Then(`^the import left the field "([^"]*)" in "([^"]*)" undone$`, theImportLeftTheFieldUndone)
	sc.Then(`^no group is titled "([^"]*)"$`, noGroupIsTitled)
	sc.Then(`^the group "([^"]*)" holds "([^"]*)"$`, theGroupHolds)
}
