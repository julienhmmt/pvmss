export interface ApiError {
	code: string;
	message: string;
}

export class ApiRequestError extends Error {
	constructor(
		public readonly status: number,
		public readonly error: ApiError
	) {
		super(error.message);
	}
}
