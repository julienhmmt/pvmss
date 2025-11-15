package proxmox

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"pvmss/logger"
)

// FetchAllNodeDetailsResty retrieves node details concurrently for all nodes and
// returns cached-friendly structs. Offline/unreachable nodes are represented to
// avoid blocking callers.
func FetchAllNodeDetailsResty(ctx context.Context, restyClient *RestyClient) ([]*NodeDetails, error) {
	nodes, err := GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get Proxmox node names: %w", err)
	}

	if len(nodes) == 0 {
		return []*NodeDetails{}, nil
	}

	const maxConcurrent = 8
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	detailsChan := make(chan *NodeDetails, len(nodes))

	for _, nodeName := range nodes {
		wg.Add(1)
		name := nodeName
		go func() {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			nodeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()

			details, detailErr := GetNodeDetailsResty(nodeCtx, restyClient, name)
			if detailErr != nil {
				logger.Get().Warn().Err(detailErr).Str("node", name).Msg("Failed to refresh node details for cache")
				detailsChan <- &NodeDetails{Node: name, Status: "offline"}
				return
			}

			detailsChan <- details
		}()
	}

	go func() {
		wg.Wait()
		close(detailsChan)
	}()

	collected := make([]*NodeDetails, 0, len(nodes))
	for detail := range detailsChan {
		collected = append(collected, detail)
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Node < collected[j].Node
	})

	return collected, nil
}
