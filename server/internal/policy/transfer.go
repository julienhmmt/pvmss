package policy

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
)

// Import outcomes reported per cluster (policy import) or per node (node
// capacity import).
const (
	ImportUpdated   = "updated"
	ImportUnchanged = "unchanged"
	ImportSkipped   = "skipped"
	ImportInvalid   = "invalid"
	ImportNotFound  = "not_found"
	ImportFailed    = "failed"
)

// ImportOutcome reports what happened to one cluster or one node of an import.
type ImportOutcome struct {
	Cluster string
	Node    string
	Result  string
	Error   string
}

// NodeCapacityImport is one node's caps read from an export file.
type NodeCapacityImport struct {
	Node     string
	Capacity Capacity
}

// ImportPolicy writes the given policy to every cluster, in the order given,
// and reports one outcome per cluster. Invalid settings and a lost concurrency
// race are reported per cluster and do not stop the others.
func (service *Policy) ImportPolicy(ctx context.Context, clusters []string, settings Settings) ([]ImportOutcome, error) {
	out := make([]ImportOutcome, 0, len(clusters))

	for _, clusterName := range clusters {
		outcome, err := service.importPolicyOne(ctx, clusterName, settings)
		if err != nil {
			return out, err
		}

		out = append(out, outcome)
	}

	return out, nil
}

func (service *Policy) importPolicyOne(ctx context.Context, clusterName string, settings Settings) (ImportOutcome, error) {
	outcome := ImportOutcome{Cluster: clusterName}

	update, err := service.UpdatePolicy(ctx, clusterName, func(Settings) Settings { return settings })

	switch {
	case err == nil && update.Changed:
		outcome.Result = ImportUpdated
	case err == nil:
		outcome.Result = ImportUnchanged
	case errors.Is(err, ErrInvalidPolicy):
		outcome.Result, outcome.Error = ImportInvalid, err.Error()
	case errors.Is(err, ErrConcurrentUpdate):
		outcome.Result, outcome.Error = ImportFailed, err.Error()
	default:
		return outcome, err
	}

	return outcome, nil
}

// ImportNodeCapacities writes node caps to one cluster, in file order. A node
// that already carries caps is skipped unless overwrite is set; a node the
// cluster does not report, or caps the node cannot honour, are reported per
// node and do not stop the others.
func (service *Policy) ImportNodeCapacities(ctx context.Context, clusterName string, items []NodeCapacityImport, overwrite bool) ([]ImportOutcome, error) {
	out := make([]ImportOutcome, 0, len(items))

	for _, item := range items {
		outcome, err := service.importNodeCapacityOne(ctx, clusterName, item, overwrite)
		if err != nil {
			return out, err
		}

		out = append(out, outcome)
	}

	return out, nil
}

func (service *Policy) importNodeCapacityOne(ctx context.Context, clusterName string, item NodeCapacityImport, overwrite bool) (ImportOutcome, error) {
	outcome := ImportOutcome{Cluster: clusterName, Node: item.Node}

	// NodeCapacity answers a node the cluster does not report with zero caps,
	// so existence has to be checked up front: an all-zero row would
	// otherwise read "unchanged" instead of "not_found".
	if _, err := service.discoveredNode(ctx, clusterName, item.Node); err != nil {
		if errors.Is(err, cluster.ErrNotFound) {
			outcome.Result = ImportNotFound

			return outcome, nil
		}

		return outcome, err
	}

	current, err := service.NodeCapacity(ctx, clusterName, item.Node)
	if err != nil {
		return outcome, err
	}

	if !overwrite && hasCapacityCaps(current) {
		outcome.Result = ImportSkipped

		return outcome, nil
	}

	if sameCapacityCaps(current, item.Capacity) {
		outcome.Result = ImportUnchanged

		return outcome, nil
	}

	if err := service.SetNodeCapacity(ctx, clusterName, item.Node, item.Capacity); err != nil {
		result, ok := nodeImportFailure(err)
		if !ok {
			return outcome, err
		}

		outcome.Result, outcome.Error = result, err.Error()

		return outcome, nil
	}

	outcome.Result = ImportUpdated

	return outcome, nil
}

// nodeImportFailure maps a refused write to its reported outcome; ok is false
// for errors that must abort the import.
func nodeImportFailure(err error) (string, bool) {
	switch {
	case errors.Is(err, cluster.ErrNotFound):
		return ImportNotFound, true
	case errors.Is(err, ErrInvalidPolicy), errors.Is(err, ErrBelowCurrentUsage), errors.Is(err, ErrAboveNodeCapacity):
		return ImportInvalid, true
	default:
		return "", false
	}
}

// hasCapacityCaps reports whether the node already carries any configured cap.
func hasCapacityCaps(capacity Capacity) bool {
	return capacity.MaxVMs != 0 || capacity.MaxVCPUs != 0 || capacity.MaxRAMGB != 0 || capacity.MaxDiskGB != 0
}

// sameCapacityCaps reports whether the node already holds exactly these caps.
func sameCapacityCaps(current, wanted Capacity) bool {
	return current.MaxVMs == wanted.MaxVMs && current.MaxVCPUs == wanted.MaxVCPUs &&
		current.MaxRAMGB == wanted.MaxRAMGB && current.MaxDiskGB == wanted.MaxDiskGB
}
