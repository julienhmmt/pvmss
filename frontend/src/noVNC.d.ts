declare module "/noVNC-1.6.0/core/rfb.js" {
  export interface RFB {
    connect(url: string, credentials?: { password?: string }): void;
    disconnect(): void;
    sendKey(key: number, code?: string): void;
    sendCredentials(credentials: {
      username?: string;
      password?: string;
    }): void;
    focus(): void;
    blur(): void;
    clipboardPasteFrom(text: string): void;
    clipboardPasteTo(callback: (text: string) => void): void;
    sendCtrlAltDel(): void;
    sendText(text: string): void;
    resize(width: number, height: number): void;
    getDisplay(): { width: number; height: number };
    addEventListener(
      event: string,
      callback: (...args: unknown[]) => void,
    ): void;
    removeEventListener(
      event: string,
      callback: (...args: unknown[]) => void,
    ): void;
  }

  export const RFB: RFB;
  export default RFB;
}
