package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

type fakeMetricsHistoryReader struct {
	samples []cluster.MetricsSample
	err     error
	gotNode string
	gotVMID int
	gotTF   cluster.MetricsTimeframe
}

func (f *fakeMetricsHistoryReader) GetMetricsHistory(_ context.Context, node string, vmid int, tf cluster.MetricsTimeframe) ([]cluster.MetricsSample, error) {
	f.gotNode = node
	f.gotVMID = vmid
	f.gotTF = tf

	return f.samples, f.err
}

func TestGetMetricsHistory_HappyPath(t *testing.T) {
	t.Parallel()

	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	want := []cluster.MetricsSample{
		{Timestamp: time.Now(), CPUPercent: 12.5, MemoryUsed: 1024, MemoryMax: 4096},
		{Timestamp: time.Now().Add(time.Minute), CPUPercent: 25.0, MemoryUsed: 2048, MemoryMax: 4096},
	}

	reader := &fakeMetricsHistoryReader{samples: want}

	got, err := vm.GetMetricsHistory(context.Background(), vm.MetricsDependencies{
		Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Reader: reader,
	}, cluster.MetricsTimeframeHour)
	if err != nil {
		t.Fatalf("GetMetricsHistory: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("samples count = %d, want %d", len(got), len(want))
	}

	if reader.gotNode != cluster.FakeNode01 {
		t.Fatalf("reader node = %q, want %q", reader.gotNode, cluster.FakeNode01)
	}

	if reader.gotVMID != 100 {
		t.Fatalf("reader vmid = %d, want 100", reader.gotVMID)
	}

	if reader.gotTF != cluster.MetricsTimeframeHour {
		t.Fatalf("reader timeframe = %q, want %q", reader.gotTF, cluster.MetricsTimeframeHour)
	}
}

func assertGetMetricsHistoryError(t *testing.T, actor auth.Identity, vmid int, tf cluster.MetricsTimeframe, wantErr error) {
	t.Helper()

	idx := buildResolveIndex(t)
	reader := &fakeMetricsHistoryReader{samples: []cluster.MetricsSample{{}}}

	_, err := vm.GetMetricsHistory(context.Background(), vm.MetricsDependencies{
		Index: idx, Actor: actor, ClusterName: testClusterName, VMID: vmid, Reader: reader,
	}, tf)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	if reader.gotNode != "" || reader.gotVMID != 0 {
		t.Fatalf("reader was called despite Resolve failure: %+v", reader)
	}
}

func TestGetMetricsHistory_NonOwnerForbidden(t *testing.T) {
	t.Parallel()

	bob := auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}
	assertGetMetricsHistoryError(t, bob, 100, cluster.MetricsTimeframeHour, vm.ErrForbidden)
}

func TestGetMetricsHistory_VMNotFound(t *testing.T) {
	t.Parallel()

	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	assertGetMetricsHistoryError(t, alice, 999, cluster.MetricsTimeframeDay, vm.ErrNotFound)
}

func TestGetMetricsHistory_NilIndexReturnsNotFound(t *testing.T) {
	t.Parallel()

	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	reader := &fakeMetricsHistoryReader{samples: []cluster.MetricsSample{{}}}

	_, err := vm.GetMetricsHistory(context.Background(), vm.MetricsDependencies{
		Index: nil, Actor: alice, ClusterName: testClusterName, VMID: 100, Reader: reader,
	}, cluster.MetricsTimeframeWeek)
	if !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetMetricsHistory_ReaderErrorWraps(t *testing.T) {
	t.Parallel()

	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	readerErr := errors.New("proxmox timeout")
	reader := &fakeMetricsHistoryReader{err: readerErr}

	_, err := vm.GetMetricsHistory(context.Background(), vm.MetricsDependencies{
		Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Reader: reader,
	}, cluster.MetricsTimeframeHour)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, readerErr) {
		t.Fatalf("err = %v, want it to wrap %v", err, readerErr)
	}
}

func TestGetMetricsHistory_TableDriven(t *testing.T) {
	t.Parallel()

	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	bob := auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}

	cases := []struct {
		name      string
		actor     auth.Identity
		vmid      int
		timeframe cluster.MetricsTimeframe
		wantErr   error
	}{
		{"owner hour", alice, 100, cluster.MetricsTimeframeHour, nil},
		{"owner day", alice, 100, cluster.MetricsTimeframeDay, nil},
		{"owner week", alice, 100, cluster.MetricsTimeframeWeek, nil},
		{"non-owner forbidden", bob, 100, cluster.MetricsTimeframeHour, vm.ErrForbidden},
		{"not found", alice, 999, cluster.MetricsTimeframeHour, vm.ErrNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader := &fakeMetricsHistoryReader{samples: []cluster.MetricsSample{{CPUPercent: 1}}}

			_, err := vm.GetMetricsHistory(context.Background(), vm.MetricsDependencies{
				Index: idx, Actor: tc.actor, ClusterName: testClusterName, VMID: tc.vmid, Reader: reader,
			}, tc.timeframe)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if reader.gotTF != tc.timeframe {
				t.Fatalf("timeframe = %q, want %q", reader.gotTF, tc.timeframe)
			}
		})
	}
}
