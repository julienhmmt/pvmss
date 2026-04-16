interface MigrateFromJSONResponse {
  message: string;
  nodes_count: number;
  storages_count: number;
  isos_count: number;
  vmbrs_count: number;
  tags_count: number;
  cloudinit_count: number;
  vm_profiles_count: number;
}

export async function exportDB(): Promise<void> {
  const res = await fetch("/api/v1/admin/db/export", { credentials: "same-origin" });
  if (!res.ok) throw new Error(`Export failed: ${res.statusText}`);
  const blob = await res.blob();
  const cd = res.headers.get("Content-Disposition") ?? "";
  const match = cd.match(/filename="([^"]+)"/);
  const filename = match ? match[1] : "pvmss-backup.db";
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export async function importDB(file: File): Promise<void> {
  const form = new FormData();
  form.append("db", file);
  const res = await fetch("/api/v1/admin/db/import", {
    method: "POST",
    credentials: "same-origin",
    body: form,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(err.message ?? res.statusText);
  }
}

export async function migrateFromJSON(file: File): Promise<MigrateFromJSONResponse> {
  const form = new FormData();
  form.append("settings", file);
  const res = await fetch("/api/v1/admin/migrate-from-json", {
    method: "POST",
    credentials: "same-origin",
    body: form,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(err.message ?? res.statusText);
  }
  return res.json();
}
