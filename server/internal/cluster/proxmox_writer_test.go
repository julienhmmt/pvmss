package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestProxmox_Action_InvalidRejectedLocally(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("must not call proxmox for an invalid action")
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.Action(context.Background(), testNodeName, testVMID, "nonsense")
	if !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("err = %v, want ErrInvalidAction", err)
	}
}

func TestProxmox_Action_Valid(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu/101/status/start", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmstart:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.Action(context.Background(), testNodeName, testVMID, "start"); err != nil {
		t.Fatalf("Action: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/status/start" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestProxmox_Patch_OnlySendsSetFields(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.Patch(context.Background(), testNodeName, testVMID, "new-name", ""); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	if gotForm.Get("name") != "new-name" {
		t.Errorf("name = %q", gotForm.Get("name"))
	}

	if _, ok := gotForm["description"]; ok {
		t.Errorf("description should not be sent when empty, form = %v", gotForm)
	}
}

func TestProxmox_Patch_NoFieldsSkipsRequest(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("must not call proxmox when nothing changed")
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.Patch(context.Background(), testNodeName, testVMID, "", ""); err != nil {
		t.Fatalf("Patch: %v", err)
	}
}

func TestProxmox_AddDisk_PicksNextFreeSlot(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"scsi0":"local-lvm:vm-101-disk-0,size=32G"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	key, err := p.AddDisk(context.Background(), testNodeName, testVMID, "scsi", "local-lvm", 16)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}

	if key != "scsi1" {
		t.Fatalf("key = %q, want scsi1 (scsi0 already taken)", key)
	}

	if gotForm.Get("scsi1") != "local-lvm:16" {
		t.Errorf("scsi1 = %q, want local-lvm:16", gotForm.Get("scsi1"))
	}
}

func TestProxmox_AddDisk_NeverOffersReservedIDESlots(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			// ide0 and ide1 already taken; ide2 (cdrom) and ide3 (cloud-init)
			// must never be offered even though they're free-looking slots.
			writeJSONFixture(t, w, `{"data":{"ide0":"local:vm-101-disk-0,size=8G","ide1":"local:vm-101-disk-1,size=8G"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("must not attempt to write a disk when the bus is exhausted")
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if _, err := p.AddDisk(context.Background(), testNodeName, testVMID, "ide", "local", 8); err == nil {
		t.Fatal("expected an error: no free non-reserved ide slot")
	}
}

func TestProxmox_ResizeDisk(t *testing.T) {
	t.Parallel()

	var gotDisk, gotSize string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/resize", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotDisk = r.FormValue("disk")
			gotSize = r.FormValue("size")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.ResizeDisk(context.Background(), testNodeName, testVMID, diskKeySCSI0, 64); err != nil {
		t.Fatalf("ResizeDisk: %v", err)
	}

	if gotDisk != diskKeySCSI0 || gotSize != "64G" {
		t.Errorf("disk=%q size=%q", gotDisk, gotSize)
	}
}

func TestProxmox_DeleteDisk(t *testing.T) {
	t.Parallel()

	var gotIDList, gotForce string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/unlink", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotIDList = r.FormValue("idlist")
			gotForce = r.FormValue("force")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.DeleteDisk(context.Background(), testNodeName, testVMID, "scsi1"); err != nil {
		t.Fatalf("DeleteDisk: %v", err)
	}

	if gotIDList != "scsi1" || gotForce != "1" {
		t.Errorf("idlist=%q force=%q", gotIDList, gotForce)
	}
}

func TestProxmox_SetCDROM(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		state     CDROMState
		wantKey   string
		wantValue string
	}{
		{"mounted", CDROMState{State: CDROMMounted, ISOVolID: "local:iso/debian-12.iso"}, cdromDiskKey, cdromMountedValue},
		{"empty", CDROMState{State: CDROMEmpty}, cdromDiskKey, "none,media=cdrom"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotValue string

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Fatalf("parse form: %v", err)
					}

					gotValue = r.FormValue(tc.wantKey)

					writeJSONFixture(t, w, `{"data":null}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			if err := p.SetCDROM(context.Background(), testNodeName, testVMID, tc.state); err != nil {
				t.Fatalf("SetCDROM: %v", err)
			}

			if gotValue != tc.wantValue {
				t.Errorf("%s = %q, want %q", tc.wantKey, gotValue, tc.wantValue)
			}
		})
	}
}

func TestProxmox_SetCDROM_Absent(t *testing.T) {
	t.Parallel()

	var gotDelete string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotDelete = r.FormValue("delete")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.SetCDROM(context.Background(), testNodeName, testVMID, CDROMState{State: CDROMAbsent}); err != nil {
		t.Fatalf("SetCDROM: %v", err)
	}

	if gotDelete != "ide2" {
		t.Errorf("delete = %q, want ide2", gotDelete)
	}
}

func TestProxmox_UpdateNetwork_DeletesRemovedIndices(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"net0":"virtio=AA:BB,bridge=vmbr0","net1":"virtio=CC:DD,bridge=vmbr1"}}`)
		})
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	// Only net0 survives in the new set — net1 must be deleted.
	err := p.UpdateNetwork(context.Background(), testNodeName, testVMID, []NetworkInterface{
		{Index: 0, Model: string(DiskBusVirtio), MAC: "AA:BB", Bridge: FakeBridgeVMbr0},
	})
	if err != nil {
		t.Fatalf("UpdateNetwork: %v", err)
	}

	if gotForm.Get("net0") == "" {
		t.Error("net0 should be re-sent")
	}

	if gotForm.Get("delete") != "net1" {
		t.Errorf("delete = %q, want net1", gotForm.Get("delete"))
	}
}

func TestProxmox_UpdateHardware(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("PUT /api2/json/nodes/node01/qemu/101/config", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.UpdateHardware(context.Background(), testNodeName, testVMID, 2, 4, 4096, []string{FakeTagPvmss, "prod"})
	if err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	if gotForm.Get("sockets") != "2" || gotForm.Get("cores") != "4" || gotForm.Get("memory") != "4096" {
		t.Errorf("form = %v", gotForm)
	}

	if gotForm.Get("tags") != "pvmss;prod" {
		t.Errorf("tags = %q", gotForm.Get("tags"))
	}
}

func TestProxmox_Delete(t *testing.T) {
	t.Parallel()

	var gotPurge string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("DELETE /api2/json/nodes/node01/qemu/101", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotPurge = r.FormValue("purge")

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmdestroy:101:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.Delete(context.Background(), testNodeName, testVMID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if gotPurge != "1" {
		t.Errorf("purge = %q, want 1", gotPurge)
	}
}

func TestProxmox_Delete_RunningVMMappedToErrVMRunning(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("DELETE /api2/json/nodes/node01/qemu/101", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data":null,"message":"VM 101 is running - destroy failed\n"}`))
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.Delete(context.Background(), testNodeName, testVMID)
	if !errors.Is(err, ErrVMRunning) {
		t.Fatalf("err = %v, want ErrVMRunning", err)
	}
}
