export type AppError = Error & { status?: number; code?: string };

export function toError(err: unknown): AppError {
  return err instanceof Error ? err : new Error(String(err));
}
