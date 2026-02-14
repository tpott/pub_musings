/**
 * WebSocket observer for PBT tests.
 *
 * Injected via page.addInitScript(). Wraps the WebSocket constructor
 * to intercept incoming messages. Stores parsed JSON messages in
 * window.__PBT_WS_MESSAGES__ for assertion methods.
 */

/** Returns a function suitable for page.addInitScript(). */
export function getWSObserverScript(): () => void {
  return () => {
    const w = window as any;
    w.__PBT_WS_MESSAGES__ = [] as any[];
    w.__PBT_CLEAR_MESSAGES__ = () => {
      w.__PBT_WS_MESSAGES__ = [];
    };

    const OriginalWebSocket = window.WebSocket;

    class ObservedWebSocket extends OriginalWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        super(url, protocols);
        this.addEventListener('message', (event: MessageEvent) => {
          if (typeof event.data === 'string') {
            try {
              const msg = JSON.parse(event.data);
              w.__PBT_WS_MESSAGES__.push(msg);
            } catch {
              // Non-JSON text message, ignore
            }
          }
        });
      }
    }

    // Preserve static properties
    Object.defineProperty(ObservedWebSocket, 'CONNECTING', { value: 0 });
    Object.defineProperty(ObservedWebSocket, 'OPEN', { value: 1 });
    Object.defineProperty(ObservedWebSocket, 'CLOSING', { value: 2 });
    Object.defineProperty(ObservedWebSocket, 'CLOSED', { value: 3 });

    (window as any).WebSocket = ObservedWebSocket;
  };
}
