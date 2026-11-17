package catalog_test

import (
	"context"
	"pvmss/server/internal/catalog"
	"slices"
	"testing"
)

func TestReorderProfiles_AdminAndUserOrder(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	want := []string{mediumProfileID, "large", testProfileID}
	if err := catalog.ReorderProfiles(ctx, st, "default", want); err != nil {
		t.Fatalf("ReorderProfiles: %v", err)
	}

	admin, err := catalog.ListAdminProfiles(ctx, st, "default")
	if err != nil {
		t.Fatal(err)
	}

	user, err := catalog.Profiles(ctx, st, "default")
	if err != nil {
		t.Fatal(err)
	}

	var adminIDs, userIDs []string
	for _, p := range admin {
		adminIDs = append(adminIDs, p.ID)
	}

	for _, p := range user {
		userIDs = append(userIDs, p.ID)
	}

	if !slices.Equal(adminIDs, want) || !slices.Equal(userIDs, want) {
		t.Fatalf("order admin=%v user=%v, want %v", adminIDs, userIDs, want)
	}

	// A new profile lands at the end.
	created, err := catalog.CreateProfile(ctx, st, "default", catalog.ProfileSpec{Label: "Tiny", CPUCores: 1, MemoryMB: 512, DiskGB: 5, Bus: testProfileBus})
	if err != nil {
		t.Fatal(err)
	}

	admin, _ = catalog.ListAdminProfiles(ctx, st, "default")
	if last := admin[len(admin)-1].ID; last != created.ID {
		t.Fatalf("last profile = %q, want %q", last, created.ID)
	}
}

func TestImportProfiles_Outcomes(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()

	items := []catalog.ImportItem{
		{Spec: catalog.ProfileSpec{Label: "Small", CPUCores: 3, MemoryMB: 2048, DiskGB: 20, Bus: testProfileBus}, Enabled: true},
		{Spec: catalog.ProfileSpec{Label: "Huge", CPUCores: 16, MemoryMB: 65536, DiskGB: 500, Bus: testProfileBus}, Enabled: false},
		{Spec: catalog.ProfileSpec{Label: "Broken", CPUCores: 0, MemoryMB: 2048, DiskGB: 20, Bus: testProfileBus}, Enabled: true},
	}

	out, err := catalog.ImportProfiles(ctx, st, []string{"default", "other"}, items, false)
	if err != nil {
		t.Fatalf("ImportProfiles: %v", err)
	}

	got := make([]string, len(out))
	for i, o := range out {
		got[i] = o.Cluster + ":" + o.ID + ":" + o.Result
	}

	want := []string{
		"default:" + testProfileID + ":skipped", "default:huge:created", "default:broken:invalid",
		"other:" + testProfileID + ":created", "other:huge:created", "other:broken:invalid",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}

	profiles, _ := catalog.ListAdminProfiles(ctx, st, "default")
	for _, p := range profiles {
		if p.ID == "huge" && p.Enabled {
			t.Error("huge should be imported disabled")
		}

		if p.ID == testProfileID && p.CPUCores != 1 {
			t.Error("small must not change without overwrite")
		}
	}

	out, err = catalog.ImportProfiles(ctx, st, []string{"default"}, items[:1], true)
	if err != nil || out[0].Result != catalog.ImportUpdated {
		t.Fatalf("overwrite: %v %v", out, err)
	}

	profiles, _ = catalog.ListAdminProfiles(ctx, st, "default")
	if i := slices.IndexFunc(profiles, func(p catalog.AdminProfile) bool { return p.ID == testProfileID }); profiles[i].CPUCores != 3 {
		t.Error("small should be overwritten to 3 cores")
	}
}

func TestImportProfiles_UsesRequestedID(t *testing.T) {
	t.Parallel()

	st := openAdminStore(t)
	ctx := context.Background()
	spec := catalog.ProfileSpec{Label: "Imported", CPUCores: 1, MemoryMB: 512, DiskGB: 5, Bus: testProfileBus}

	cases := []struct {
		newID, wantID, wantResult string
	}{
		{"custom-id", "custom-id", catalog.ImportCreated},
		{"", "imported", catalog.ImportCreated},
		{"Bad ID", "Bad ID", catalog.ImportInvalid},
	}
	for _, tc := range cases {
		s := spec
		s.NewID = tc.newID

		out, err := catalog.ImportProfiles(ctx, st, []string{"default"}, []catalog.ImportItem{{Spec: s, Enabled: true}}, false)
		if err != nil {
			t.Fatal(err)
		}

		if out[0].ID != tc.wantID || out[0].Result != tc.wantResult {
			t.Fatalf("newID %q: got %s/%s, want %s/%s", tc.newID, out[0].ID, out[0].Result, tc.wantID, tc.wantResult)
		}
	}
}
