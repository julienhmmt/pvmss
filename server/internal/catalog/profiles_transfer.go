package catalog

import (
	"context"
	"errors"
	"pvmss/server/internal/store"
)

// ReorderProfiles stores the admin display order of a cluster's profiles. The
// same order is used by the user-side VM create picker.
func ReorderProfiles(ctx context.Context, st *store.Store, cluster string, ids []string) error {
	return st.ReorderProfiles(ctx, cluster, ids)
}

// ImportItem is one profile read from an export file.
type ImportItem struct {
	Spec    ProfileSpec
	Enabled bool
}

// Import outcomes reported per profile and cluster.
const (
	ImportCreated = "created"
	ImportUpdated = "updated"
	ImportSkipped = "skipped"
	ImportInvalid = "invalid"
)

// ImportOutcome reports what happened to one profile on one cluster.
type ImportOutcome struct {
	Cluster string
	ID      string
	Label   string
	Result  string
	Error   string
}

// ImportProfiles writes items to every target cluster, in file order, so new
// profiles land at the end in the exported order. An existing id is skipped
// unless overwrite is set; an overwritten profile keeps its position. Invalid
// items are reported and do not stop the others.
func ImportProfiles(ctx context.Context, st *store.Store, clusters []string, items []ImportItem, overwrite bool) ([]ImportOutcome, error) {
	out := make([]ImportOutcome, 0, len(clusters)*len(items))

	for _, cluster := range clusters {
		for _, item := range items {
			outcome, err := importOne(ctx, st, cluster, item, overwrite)
			if err != nil {
				return out, err
			}

			out = append(out, outcome)
		}
	}

	return out, nil
}

func importOne(ctx context.Context, st *store.Store, cluster string, item ImportItem, overwrite bool) (ImportOutcome, error) {
	id, err := createProfileID(item.Spec.Label, item.Spec.NewID)
	if err != nil {
		return invalidOrFail(ImportOutcome{Cluster: cluster, ID: item.Spec.NewID, Label: item.Spec.Label}, err)
	}

	outcome := ImportOutcome{Cluster: cluster, ID: id, Label: item.Spec.Label}

	profile, err := CreateProfile(ctx, st, cluster, item.Spec)

	switch {
	case err == nil:
		outcome.Result = ImportCreated
	case errors.Is(err, ErrDuplicateProfile) && !overwrite:
		outcome.Result = ImportSkipped

		return outcome, nil
	case errors.Is(err, ErrDuplicateProfile):
		spec := item.Spec
		spec.NewID = "" // id already chosen; do not rename on overwrite

		profile, err = UpdateProfile(ctx, st, cluster, id, spec)
		if err != nil {
			return invalidOrFail(outcome, err)
		}

		outcome.Result = ImportUpdated
	default:
		return invalidOrFail(outcome, err)
	}

	if profile.Enabled != item.Enabled {
		if err := SetProfileEnabled(ctx, st, cluster, profile.ID, item.Enabled); err != nil {
			return outcome, err
		}
	}

	return outcome, nil
}

func invalidOrFail(outcome ImportOutcome, err error) (ImportOutcome, error) {
	if errors.Is(err, ErrInvalidProfile) {
		outcome.Result = ImportInvalid
		outcome.Error = err.Error()

		return outcome, nil
	}

	return outcome, err
}
