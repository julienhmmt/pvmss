export async function exportDB(): Promise<void> {
  const res = await fetch("/api/v1/admin/db/export", { credentials: "same-origin" });
  if (!res.ok) throw new Error(`Export failed: ${res.statusText}`);
  const blob = await res.blob();
  const cd = res.headers.get("Content-Disposition") ?? "";
  const match = cd.match(/filename="([^"]+)"/);
  const filename = match?.[1] ?? "pvmss-backup.db";
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
