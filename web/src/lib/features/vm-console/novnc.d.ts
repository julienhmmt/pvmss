// Ambient module declaration for @novnc/novnc — the package ships JS only, no
// bundled .d.ts. This declares the subset of the RFB class the console feature
// uses: constructor, runtime properties, methods, and the events dispatched.
// See noVNC docs/API.md for the full surface; only what PVMSS touches is here.
declare module '@novnc/novnc' {
	export type RFBEventType =
		| 'connect'
		| 'disconnect'
		| 'credentialsrequired'
		| 'securityfailure'
		| 'clipboard'
		| 'capabilities'
		| 'bell'
		| 'desktopname';

	export interface RFBEvent extends Event {
		readonly detail: {
			clean?: boolean;
			reason?: string;
			text?: string;
			types?: string[];
			capabilities?: Record<string, boolean>;
			name?: string;
		};
	}

	export interface RFBEventListener extends EventListener {
		(event: RFBEvent): void;
	}

	/**
	 * RFB — a VNC client. Constructed with a target HTMLElement and a WebSocket
	 * URL; attaches the remote framebuffer to the target as a canvas.
	 */
	export default class RFB extends EventTarget {
		constructor(target: HTMLElement, urlOrChannel: string | WebSocket, options?: {
			shared?: boolean;
			credentials?: Record<string, string>;
			wsProtocols?: string[];
			repeaterID?: string;
		});

		// Connection lifecycle.
		disconnect(): void;
		sendCredentials(creds: Record<string, string>): void;
		sendCtrlAltDel(): void;
		sendKey(keysym: number, code: string, down: boolean): void;
		focus(options?: { focusOnClick?: boolean }): void;
		blur(): void;

		// Clipboard — paste from the local clipboard into the remote VM.
		clipboardPasteFrom(text: string): void;

		// Runtime properties.
		get viewOnly(): boolean;
		set viewOnly(value: boolean);
		get scaleViewport(): boolean;
		set scaleViewport(value: boolean);
		get clipViewport(): boolean;
		set clipViewport(value: boolean);
		get resizeSession(): boolean;
		set resizeSession(value: boolean);
		get qualityLevel(): number;
		set qualityLevel(value: number);
		get compressionLevel(): number;
		set compressionLevel(value: number);

		// Machine control (requires server-side power capabilities).
		machineShutdown(): void;
		machineReboot(): void;
		machineReset(): void;

		addEventListener(type: RFBEventType, listener: RFBEventListener): void;
		removeEventListener(type: RFBEventType, listener: RFBEventListener): void;
	}
}
