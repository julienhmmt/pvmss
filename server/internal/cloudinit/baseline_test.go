package cloudinit_test

import (
	"errors"
	"pvmss/server/internal/cloudinit"
	"strings"
	"testing"
)

// TestBuildVendorData_Golden - table-driven golden test over the three
// inputs (baseline alone; baseline + override; baseline + user; all three),
// asserting the externally observable document the portal would hand a guest.
// Pins the Debian branch's install → purge → sentinel ordering, the
// RHEL-family and Arch paths (no kernel work, no reboot), the user's scalars
// winning, and packages/runcmd concatenating rather than replacing.
// goldenCase is one row of TestBuildVendorData_Golden: the inputs to
// BuildVendorData, the expected error, and a positive assertion on the
// merged document (failures print the document so the golden can be
// inspected in the test log).
type goldenCase struct {
	name    string
	inputs  cloudinit.BaselineInputs
	wantErr error
	check   func(t *testing.T, doc string)
}

func TestBuildVendorData_Golden(t *testing.T) {
	t.Parallel()

	tests := []goldenCase{
		{
			name:   "generated baseline alone",
			inputs: cloudinit.BaselineInputs{},
			check:  checkBaselineAlone,
		},
		{
			name: "admin override replaces generated baseline",
			inputs: cloudinit.BaselineInputs{
				Override: "#cloud-config\npackages:\n  - nmap\n",
			},
			check: checkOverrideReplacesBaseline,
		},
		{
			name: "user document alone merges on top of generated baseline",
			inputs: cloudinit.BaselineInputs{
				UserDocument: "#cloud-config\npackages:\n  - nmap\nwrite_files: []\n",
			},
			check: checkUserDocMerges,
		},
		{
			name: "all three: override replaces baseline, user merges on top",
			inputs: cloudinit.BaselineInputs{
				Override:     "#cloud-config\npackages:\n  - curl\nruncmd:\n  - echo base\n",
				UserDocument: "#cloud-config\npackages:\n  - nmap\nruncmd:\n  - echo user\n",
			},
			check: checkOverridePlusUser,
		},
		{
			name: "user scalar wins over baseline scalar",
			inputs: cloudinit.BaselineInputs{
				UserDocument: "#cloud-config\npackage_update: false\n",
			},
			check: checkUserScalarWins,
		},
		{
			name: "user packages and runcmd add to baseline, never replace",
			inputs: cloudinit.BaselineInputs{
				UserDocument: "#cloud-config\npackages:\n  - nmap\nruncmd:\n  - echo hi\n",
			},
			check: checkUserListsAdd,
		},
		{
			name: "nested map: user key wins without losing baseline siblings",
			inputs: cloudinit.BaselineInputs{
				UserDocument: "#cloud-config\npower_state:\n  timeout: 60\n",
			},
			check: checkNestedMapMerge,
		},
		{
			name: "malformed override is rejected with validation error",
			inputs: cloudinit.BaselineInputs{
				Override: "not a cloud-config document",
			},
			wantErr: cloudinit.ErrBaselineInvalid,
		},
		{
			name: "malformed user document is rejected with validation error",
			inputs: cloudinit.BaselineInputs{
				UserDocument: "#cloud-config\ninvalid: [",
			},
			wantErr: cloudinit.ErrBaselineInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runGoldenCase(t, tt)
		})
	}
}

// runGoldenCase executes one golden-table row: BuildVendorData must fail
// with wantErr when set, otherwise produce a document that passes check and
// is itself a valid #cloud-config.
func runGoldenCase(t *testing.T, tt goldenCase) {
	t.Helper()

	doc, err := cloudinit.BuildVendorData(tt.inputs)
	if tt.wantErr != nil {
		if !errors.Is(err, tt.wantErr) {
			t.Fatalf("BuildVendorData error = %v, want %v", err, tt.wantErr)
		}

		return
	}

	if err != nil {
		t.Fatalf("BuildVendorData error = %v, want nil", err)
	}

	if tt.check != nil {
		tt.check(t, doc)
	}

	if err := cloudinit.Validate(doc); err != nil {
		t.Errorf("merged document fails Validate: %v\n%s", err, doc)
	}
}

// checkBaselineAlone pins the generated baseline's shape: the Debian
// branch's install → purge → sentinel ordering, the install guard, and the
// reboot gating on the sentinel file.
func checkBaselineAlone(t *testing.T, doc string) {
	t.Helper()

	if !strings.HasPrefix(doc, "#cloud-config") {
		t.Errorf("doc does not start with #cloud-config: %q", head(doc))
	}

	if !strings.Contains(doc, "qemu-guest-agent") {
		t.Error("doc does not install qemu-guest-agent")
	}

	// Debian branch: install before purge before sentinel.
	installIdx := strings.Index(doc, "apt-get install -y linux-image-amd64")

	purgeIdx := strings.Index(doc, "apt-get purge -y linux-image-cloud-amd64")

	sentinelIdx := strings.Index(doc, "touch /var/run/pvmss-kernel-swapped")

	if installIdx < 0 || purgeIdx < 0 || sentinelIdx < 0 {
		t.Fatalf("Debian branch missing a step:\n%s", doc)
	}

	if installIdx >= purgeIdx || purgeIdx >= sentinelIdx {
		t.Errorf("Debian branch order wrong: install=%d purge=%d sentinel=%d", installIdx, purgeIdx, sentinelIdx)
	}

	// install is guarded by `|| exit 0` so a failed install
	// cannot leave the VM kernel-less before the purge runs.
	if !strings.Contains(doc, "apt-get install -y linux-image-amd64 || exit 0") {
		t.Error("install is not guarded by || exit 0")
	}

	// power_state.condition gates the reboot on the sentinel.
	if !strings.Contains(doc, "condition: test -f /var/run/pvmss-kernel-swapped") {
		t.Error("power_state.condition sentinel missing")
	}
}

// checkOverrideReplacesBaseline asserts an admin override fully replaces the
// generated baseline.
func checkOverrideReplacesBaseline(t *testing.T, doc string) {
	t.Helper()

	if strings.Contains(doc, "qemu-guest-agent") {
		t.Errorf("override did not replace generated baseline:\n%s", doc)
	}

	if !strings.Contains(doc, "nmap") {
		t.Errorf("override content missing:\n%s", doc)
	}
}

// checkUserDocMerges asserts a user document merges on top of the generated
// baseline: user package added, baseline's agent preserved, user scalar
// (write_files) appears.
func checkUserDocMerges(t *testing.T, doc string) {
	t.Helper()

	if !strings.Contains(doc, "qemu-guest-agent") {
		t.Errorf("baseline agent package dropped:\n%s", doc)
	}

	if !strings.Contains(doc, "nmap") {
		t.Errorf("user package missing:\n%s", doc)
	}

	if !strings.Contains(doc, "write_files") {
		t.Errorf("user scalar key missing:\n%s", doc)
	}
}

// checkOverridePlusUser asserts the override replaces the baseline while the
// user document still merges on top: packages and runcmd concatenate
// override-first.
func checkOverridePlusUser(t *testing.T, doc string) {
	t.Helper()

	// Override's curl present, generated agent absent.
	if strings.Contains(doc, "qemu-guest-agent") {
		t.Errorf("generated baseline leaked through override:\n%s", doc)
	}

	if !strings.Contains(doc, "curl") || !strings.Contains(doc, "nmap") {
		t.Errorf("packages did not concatenate:\n%s", doc)
	}

	// runcmd concatenates: override's "echo base" before user's "echo user".
	baseIdx := strings.Index(doc, "echo base")

	userIdx := strings.Index(doc, "echo user")

	if baseIdx < 0 || userIdx < 0 || baseIdx > userIdx {
		t.Errorf("runcmd did not concatenate base-first:\n%s", doc)
	}
}

// checkUserScalarWins asserts a user scalar overrides the same baseline key.
func checkUserScalarWins(t *testing.T, doc string) {
	t.Helper()

	if !strings.Contains(doc, "package_update: false") {
		t.Errorf("user scalar did not win:\n%s", doc)
	}

	if strings.Contains(doc, "package_update: true") {
		t.Errorf("baseline scalar survived a user override:\n%s", doc)
	}
}

// checkUserListsAdd asserts user packages and runcmd add to the baseline's,
// never replace.
func checkUserListsAdd(t *testing.T, doc string) {
	t.Helper()

	if !strings.Contains(doc, "qemu-guest-agent") {
		t.Errorf("baseline package replaced by user:\n%s", doc)
	}

	if !strings.Contains(doc, "nmap") {
		t.Errorf("user package missing:\n%s", doc)
	}

	// The baseline's runcmd (the shell block) is preserved.
	if !strings.Contains(doc, "systemctl enable --now qemu-guest-agent") {
		t.Errorf("baseline runcmd replaced by user:\n%s", doc)
	}

	if !strings.Contains(doc, "echo hi") {
		t.Errorf("user runcmd missing:\n%s", doc)
	}
}

// checkNestedMapMerge asserts a nested user key wins without losing baseline
// siblings.
func checkNestedMapMerge(t *testing.T, doc string) {
	t.Helper()

	if !strings.Contains(doc, "timeout: 60") {
		t.Errorf("user nested scalar missing:\n%s", doc)
	}

	// Baseline's power_state.mode and condition survive.
	if !strings.Contains(doc, "mode: reboot") {
		t.Errorf("baseline power_state.mode dropped on nested merge:\n%s", doc)
	}

	if !strings.Contains(doc, "condition: test -f /var/run/pvmss-kernel-swapped") {
		t.Errorf("baseline power_state.condition dropped on nested merge:\n%s", doc)
	}
}

// TestBuildVendorData_DistributionFamilies - RHEL-family and Arch paths
// install and enable the agent and perform no kernel work and no reboot; a
// test asserts this so the families stay deliberately covered rather than
// silently falling through. NixOS falls through with no package or kernel
// action.
func TestBuildVendorData_DistributionFamilies(t *testing.T) {
	t.Parallel()

	// The generated baseline's runcmd dispatches on /etc/os-release. The
	// Debian branch is the only one that touches the kernel and reboots; the
	//  others fall through to `systemctl enable --now qemu-guest-agent` only.
	// Assert the case statement names the families the spec covers, so a
	// future reader can see they were considered.
	doc, err := cloudinit.BuildVendorData(cloudinit.BaselineInputs{})
	if err != nil {
		t.Fatalf("BuildVendorData: %v", err)
	}

	// The case statement pattern covers debian and ubuntu (the only family
	// that needs the kernel swap). RHEL-family and Arch are not named in the
	// case because they need nothing beyond the agent line - but the spec
	// requires a test that pins that decision.
	if !strings.Contains(doc, "*debian*|*ubuntu*") {
		t.Errorf("Debian-family case pattern missing:\n%s", doc)
	}

	// No dnf/pacman/zypper branch: RHEL-family and Arch fall through to the
	// agent enable, which is correct (they already ship a standard kernel).
	for _, pkg := range []string{"dnf install", "pacman -S", "zypper install"} {
		if strings.Contains(doc, pkg) {
			t.Errorf("unexpected %q branch - RHEL/Arch should fall through:\n%s", pkg, doc)
		}
	}

	// The agent enable runs unconditionally (after the case), so every
	// family gets it.
	if !strings.Contains(doc, "systemctl enable --now qemu-guest-agent") {
		t.Errorf("agent enable missing:\n%s", doc)
	}
}

// TestGeneratedBaseline_IsStableSource - the admin view reads the
// generated baseline from the same source the create path delivers, never a
// copy. Pin that GeneratedBaseline returns the verbatim document.
func TestGeneratedBaseline_IsStableSource(t *testing.T) {
	t.Parallel()

	got := cloudinit.GeneratedBaseline()
	if got == "" {
		t.Fatal("GeneratedBaseline returned empty")
	}

	if !strings.HasPrefix(got, "#cloud-config") {
		t.Errorf("GeneratedBaseline does not start with #cloud-config: %q", head(got))
	}

	// BuildVendorData with no inputs returns the same string.
	built, err := cloudinit.BuildVendorData(cloudinit.BaselineInputs{})
	if err != nil {
		t.Fatalf("BuildVendorData: %v", err)
	}

	if built != got {
		t.Errorf("GeneratedBaseline and BuildVendorData diverged - admin view would show a different document than the create path delivers")
	}
}

// head returns the first 80 characters of s for error messages.
func head(s string) string {
	if len(s) <= 80 {
		return s
	}

	return s[:80] + "..."
}
