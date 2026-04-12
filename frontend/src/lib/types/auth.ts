/**
 * User information.
 */
export interface User {
  /** Username */
  username: string;
  /** Whether the user has admin privileges */
  isAdmin: boolean;
  /** Optional: User's assigned pool (if applicable) */
  pool?: string;
  /** Optional: Number of VMs owned by the user */
  vmCount?: number;
}

/**
 * Login request payload.
 */
export interface LoginRequest {
  /** Username */
  username: string;
  /** User password */
  password: string;
}

/**
 * Login response from the backend.
 */
export interface LoginResponse {
  /** Username */
  username: string;
  /** Whether the user has admin privileges */
  isAdmin: boolean;
}
