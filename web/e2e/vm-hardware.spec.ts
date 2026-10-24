import { test, expect, type APIRequestContext } from "@playwright/test";
import { csrfHeaders } from "./support/csrf";

async function signInAlice(request: APIRequestContext): Promise<void> {
  const response = await request.post("/api/v1/auth/login", {
    data: { username: "alice", password: "pvmss-alice", cluster: "default" },
  });
  expect(response.status()).toBe(200);
}

test.describe("T07 VM hardware (disks, CD-ROM, network, sockets/cores/RAM/tags)", () => {
  test("add a disk, resize it while running, refuse boot-disk delete, delete the added disk", async ({
    page,
  }) => {
    await signInAlice(page.request);
    // web-02 (VMID 101, node pve-node-01) - stopped, owned by alice, has a
    // pre-seeded boot disk scsi0 and a second disk scsi1.
    await page.goto("/vms/default/101");
    await page.getByTestId("vm-tab-configuration").click();
    await page.getByTestId("vm-tab-disks").click();

    // The boot disk cell renders the key and a separate "boot" pill.
    await expect(page.locator("tr", { hasText: "scsi0" }).first()).toContainText("boot");

    await page.getByTestId("vm-disk-add-open").click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByTestId("add-disk-storage").selectOption("local-lvm");
    await page.getByTestId("add-disk-size").fill("5");
    await page.getByTestId("add-disk-submit").click();
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(page.getByTestId("vm-disk-resize-open-scsi2")).toBeVisible();

    // V20: resize is allowed on a running VM (no stopped-VM guard).
    await page.getByTestId("vm-power-start").click();
    await expect(page.getByTestId("vm-status")).toContainText(/running/i);

    await page.getByTestId("vm-disk-resize-open-scsi2").click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByTestId("resize-disk-size").fill("8");
    await page.getByTestId("resize-disk-submit").click();
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(page.getByText("8 GB")).toBeVisible();

    await page.getByTestId("vm-power-shutdown").click();
    await page.getByTestId("vm-shutdown-confirm").click();
    await expect(page.getByTestId("vm-status")).toContainText(/stopped/i);

    // The boot disk's delete control is disabled - no destructive call possible.
    await expect(page.getByTestId("vm-disk-delete-open-scsi0")).toBeDisabled();
    await expect(page.getByTestId("vm-disk-delete-open-scsi0")).toHaveText(
      "Protected",
    );

    await page.getByTestId("vm-disk-delete-open-scsi2").click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByTestId("delete-disk-confirm").click();
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(page.getByTestId("vm-disk-resize-open-scsi2")).toHaveCount(0);
  });

  test("change a network interface bridge to a different approved one, persists on reload", async ({
    page,
  }) => {
    // vmbr1 (pve-node-01) is discovered but approved by no other spec, and a
    // bridge only becomes selectable once approved. Approve it here so this
    // test owns its precondition instead of depending on another spec.
    const admin = await page.request.post("/api/v1/auth/admin-login", {
      data: { password: "pvmss-e2e-admin" },
    });
    expect(admin.status()).toBe(200);
    const approve = await page.request.post("/api/v1/admin/bridges/toggle", {
      headers: await csrfHeaders(page.request),
      data: { cluster: "default", node: "pve-node-01", name: "vmbr1", enabled: true },
    });
    expect(approve.status()).toBe(200);

    await signInAlice(page.request);
    await page.goto("/vms/default/101");
    await page.getByTestId("vm-tab-configuration").click();
    await page.getByTestId("vm-tab-network").click();

    await expect(page.getByTestId("vm-nic-0")).toContainText("vmbr0");

    await page.getByTestId("vm-nic-edit-open-0").click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByTestId("edit-nic-bridge").selectOption("vmbr1");
    await page.getByTestId("edit-nic-submit").click();
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(page.getByTestId("vm-nic-0")).toContainText("vmbr1");

    await page.reload();
    await page.getByTestId("vm-tab-configuration").click();
    await page.getByTestId("vm-tab-network").click();
    await expect(page.getByTestId("vm-nic-0")).toContainText("vmbr1");
  });
});
