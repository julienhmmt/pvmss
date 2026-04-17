/**
 * Transforms snake_case object keys to camelCase.
 * Handles consecutive underscores, numbers, and edge cases.
 */
function toCamelCase(str: string): string {
  return str
    .toLowerCase()
    .replace(/_+([a-z0-9])/g, (_, letter) => letter.toUpperCase())
    .replace(/_+$/g, "");
}

/**
 * Recursively transforms all keys in an object from snake_case to camelCase.
 * Preserves arrays, nested objects, Dates, and handles circular references.
 */
export function transformKeysToCamelCase<T>(obj: unknown, seen = new WeakMap<object, unknown>()): T {
  if (obj === null || typeof obj !== "object") {
    return obj as T;
  }

  if (obj instanceof Date) {
    return obj as T;
  }

  // Return already-transformed object for circular references
  if (seen.has(obj)) {
    return seen.get(obj) as T;
  }

  if (Array.isArray(obj)) {
    return obj.map((item) => transformKeysToCamelCase(item, seen)) as T;
  }

  // Pre-register empty object to handle circular references
  const result: Record<string, unknown> = {};
  seen.set(obj, result);

  for (const [key, value] of Object.entries(obj)) {
    const camelKey = toCamelCase(key);
    result[camelKey] = transformKeysToCamelCase(value, seen);
  }
  return result as T;
}

/**
 * Recursively transforms all keys in an object from camelCase to snake_case.
 * For API request payloads.
 */
function toSnakeCase(str: string): string {
  return str.replace(/[A-Z]/g, (letter) => `_${letter.toLowerCase()}`);
}

/**
 * Recursively transforms all keys in an object from camelCase to snake_case.
 * For sending data to the backend API. Preserves Dates and handles circular references.
 */
export function transformKeysToSnakeCase<T>(obj: unknown, seen = new WeakMap<object, unknown>()): T {
  if (obj === null || typeof obj !== "object") {
    return obj as T;
  }

  if (obj instanceof Date) {
    return obj as T;
  }

  // Return already-transformed object for circular references
  if (seen.has(obj)) {
    return seen.get(obj) as T;
  }

  if (Array.isArray(obj)) {
    return obj.map((item) => transformKeysToSnakeCase(item, seen)) as T;
  }

  // Pre-register empty object to handle circular references
  const result: Record<string, unknown> = {};
  seen.set(obj, result);

  for (const [key, value] of Object.entries(obj)) {
    const snakeKey = toSnakeCase(key);
    result[snakeKey] = transformKeysToSnakeCase(value, seen);
  }
  return result as T;
}
