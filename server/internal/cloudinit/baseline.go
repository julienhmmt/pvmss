package cloudinit

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Sentinel errors for the baseline document builder.
var (
	// ErrBaselineInvalid reports that an admin override or a user document
	// failed Validate. The cause is the underlying validation error.
	ErrBaselineInvalid = errors.New("baseline document invalid")
)

// generatedBaseline is the vendor-data document PVMSS writes for every
// cloud-image VM when no admin override is present. It is a decision, not an
// implementation detail. The Debian-family branch installs the standard kernel
//
// (the cloud kernel cannot drive the emulated VGA under UEFI), purges the
// cloud kernel, and touches a sentinel that gates the reboot; RHEL-family and
// Arch already ship a standard kernel, so they only install and enable the
// agent; NixOS is declarative and falls through deliberately (admin-override
// territory). The install→purge guard means a failed install can never leave a
// VM with no kernel; the power_state.condition means a VM with no egress is
// not rebooted for nothing.
const generatedBaseline = `#cloud-config
# PVMSS baseline — generated at create for cloud-image VMs.
# Replace it cluster-wide by placing <snippet_dir>/pvmss-baseline.yml.
package_update: true
packages:
  - qemu-guest-agent
runcmd:
  - |
    set -eu
    . /etc/os-release 2>/dev/null || true
    case "${ID:-} ${ID_LIKE:-}" in
      *debian*|*ubuntu*)
        apt-get install -y linux-image-amd64 || exit 0
        apt-get purge -y linux-image-cloud-amd64 linux-image-*-cloud-amd64 || true
        touch /var/run/pvmss-kernel-swapped
        ;;
    esac
    systemctl enable --now qemu-guest-agent || true
power_state:
  mode: reboot
  condition: test -f /var/run/pvmss-kernel-swapped
  message: "PVMSS: rebooting onto the standard kernel"
  timeout: 30
`

// GeneratedBaseline returns the verbatim baseline document the portal
// generates for cloud-image VMs. Exposed so the admin view reads
// from the same source the create path delivers, never a copy.
func GeneratedBaseline() string { return generatedBaseline }

// BaselineInputs are the three inputs to the vendor-data document builder.
// Each is optional; the precedence is generated baseline → admin override
// replaces it → user document merges on top.
type BaselineInputs struct {
	// Override is the cluster-wide admin override
	// (<snippet_dir>/pvmss-baseline.yml). When non-empty it replaces the
	// generated baseline entirely.
	Override string
	// UserDocument is the actor's own cloud-init document. When non-empty it
	// merges on top: scalar keys win over the baseline's, and packages/runcmd
	// concatenate.
	UserDocument string
}

// BuildVendorData builds the cloud-init vendor-data document for a cloud-image
// VM from the three inputs. It is pure: no I/O, no Proxmox, no logging. The
// precedence is generated baseline → admin override replaces it → user
// document merges on top (user scalars win, packages and runcmd concatenate).
//
// An empty Override selects the generated baseline; an empty UserDocument
// selects no merge. A malformed Override or UserDocument that fails Validate
// is rejected with ErrBaselineInvalid wrapping the cause — the create path
// surfaces the existing validation errors rather than silently dropping the
// document.
//
// Because every input is already validated as a #cloud-config YAML mapping,
// the merge is always YAML-into-YAML — no MIME multipart and no raw-script
// edge case.
func BuildVendorData(inputs BaselineInputs) (string, error) {
	base := generatedBaseline
	source := "generated"

	if inputs.Override != "" {
		if err := Validate(inputs.Override); err != nil {
			return "", fmt.Errorf("%w: admin override: %w", ErrBaselineInvalid, err)
		}

		base = inputs.Override
		source = "override"
	}

	if inputs.UserDocument == "" {
		return base, nil
	}

	if err := Validate(inputs.UserDocument); err != nil {
		return "", fmt.Errorf("%w: user document: %w", ErrBaselineInvalid, err)
	}

	return mergeVendorData(base, inputs.UserDocument, source)
}

// mergeVendorData merges userDoc on top of baseDoc. Scalar keys in userDoc win
// over baseDoc; packages and runcmd concatenate (baseDoc first, userDoc
// after). The result is re-serialized as a #cloud-config document. source
// labels the merge in the comment header.
func mergeVendorData(baseDoc, userDoc, source string) (string, error) {
	base, err := parseCloudConfigMap(baseDoc)
	if err != nil {
		return "", err
	}

	user, err := parseCloudConfigMap(userDoc)
	if err != nil {
		return "", err
	}

	merged := mergeMaps(base, user)

	out, err := yaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("%w: marshal merged document: %w", ErrBaselineInvalid, err)
	}

	return "#cloud-config\n# PVMSS vendor-data — " + source + " baseline + user document merged.\n" + string(out), nil
}

// parseCloudConfigMap parses a #cloud-config document into a generic map. The
// caller has already run Validate, so the root is a YAML mapping; an empty
// document parses to an empty map.
func parseCloudConfigMap(doc string) (map[string]any, error) {
	trimmed := strings.TrimLeft(doc, " \t\r\n")
	root := map[string]any{}

	if trimmed == "" {
		return root, nil
	}

	if err := yaml.Unmarshal([]byte(trimmed), &root); err != nil {
		return nil, fmt.Errorf("%w: parse document: %w", ErrBaselineInvalid, err)
	}

	return root, nil
}

// mergeMaps returns base with user merged on top. Scalar keys in user replace
// base; the list keys packages and runcmd concatenate (base first); other
// list keys replace (cloud-init's merger replaces lists by default, and only
// packages/runcmd are documented as additive in this portal's contract).
// Nested maps recurse so a user's power_state.timeout wins without losing the
// baseline's power_state.mode.
func mergeMaps(base, user map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(user))
	maps.Copy(merged, base)

	for k, uv := range user {
		bv, ok := merged[k]
		if !ok {
			merged[k] = uv

			continue
		}

		merged[k] = mergeValue(k, bv, uv)
	}

	return merged
}

// mergeValue merges one user value (uv) on top of one baseline value (bv)
// under key k. packages and runcmd concatenate; nested maps recurse; anything
// else (scalars, other lists) is replaced by the user value.
func mergeValue(key string, bv, uv any) any {
	if key == "packages" || key == "runcmd" {
		return concatLists(bv, uv)
	}

	bm, bIsMap := bv.(map[string]any)

	um, uIsMap := uv.(map[string]any)

	if bIsMap && uIsMap {
		return mergeMaps(bm, um)
	}

	return uv
}

// concatLists concatenates two list values (either []any or []string from
// yaml.v3) into a []any. A nil side yields the other side untouched.
func concatLists(bv, uv any) any {
	bl := toAnyList(bv)

	ul := toAnyList(uv)

	return slices.Concat(bl, ul)
}

// toAnyList normalises a yaml.v3 list value ([]any or []string) to []any. A
// non-list or nil becomes nil.
func toAnyList(v any) []any {
	switch l := v.(type) {
	case []any:
		return l
	case []string:
		out := make([]any, len(l))
		for i, s := range l {
			out[i] = s
		}

		return out
	default:
		return nil
	}
}
