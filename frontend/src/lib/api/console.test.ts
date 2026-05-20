import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { buildWebSocketURL } from "./console";

describe("buildWebSocketURL", () => {
  beforeEach(() => {
    vi.stubGlobal("window", {
      location: {
        host: "pvmss.example.com",
        protocol: "https:",
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses an opaque console token when provided", () => {
    const url = buildWebSocketURL(
      100,
      "PVEVNC:secret",
      5901,
      "pve-a",
      "opaque-token",
    );
    expect(url).toBe(
      "wss://pvmss.example.com/api/v1/vms/100/console/websocket?token=opaque-token",
    );
    expect(url).not.toContain("vncticket");
    expect(url).not.toContain("PVEVNC");
  });

  it("preserves the legacy query when no console token is provided", () => {
    const url = buildWebSocketURL(100, "PVEVNC:secret", 5901, "pve-a");
    expect(url).toContain("/api/v1/vms/100/console/websocket?port=5901");
    expect(url).toContain("node=pve-a");
    expect(url).toContain("vncticket=PVEVNC%3Asecret");
  });

  it("falls back to the legacy query when console token is an empty string", () => {
    const url = buildWebSocketURL(100, "PVEVNC:secret", 5901, "pve-a", "");
    expect(url).toContain("/api/v1/vms/100/console/websocket?port=5901");
    expect(url).toContain("vncticket=PVEVNC%3Asecret");
    expect(url).not.toContain("token=");
  });

  it("falls back to the legacy query when console token is whitespace", () => {
    const url = buildWebSocketURL(100, "PVEVNC:secret", 5901, "pve-a", "   ");
    expect(url).toContain("vncticket=PVEVNC%3Asecret");
    expect(url).not.toContain("token=");
  });
});
