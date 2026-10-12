import { useEffect, useRef, useState, useCallback } from 'react';
import { WSMessage } from '../types';

interface UseWebSocketOptions {
  onMessage?: (message: WSMessage) => void;
  reconnectInterval?: number;
}

export function useWebSocket(options: UseWebSocketOptions = {}) {
  const { onMessage, reconnectInterval = 5000 } = options;
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number>();

  // Closing a socket fires onclose asynchronously, after the cleanup has
  // already run. Without this flag that late onclose scheduled a reconnect
  // nobody would ever clear, so an unmounted hook kept re-opening sockets.
  const teardownRef = useRef(false);

  // Held in a ref so a caller passing an inline callback does not change the
  // identity of connect() and tear the socket down on every render.
  const onMessageRef = useRef(onMessage);
  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  const connect = useCallback(() => {
    if (teardownRef.current) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const basePath = new URL('.', window.location.href).pathname.replace(/\/+$/, '');
    const wsUrl = `${protocol}//${window.location.host}${basePath}/ws`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WebSocket connected');
      setIsConnected(true);
    };

    ws.onclose = () => {
      // Under StrictMode (and on any fast remount) the first socket's close
      // arrives after a second one is already open. That stale close must
      // neither flip isConnected nor start a reconnect, or two sockets end
      // up delivering every message twice.
      if (wsRef.current !== ws) return;

      console.log('WebSocket disconnected');
      setIsConnected(false);

      if (teardownRef.current) return;

      // Reconnect after delay
      reconnectTimeoutRef.current = window.setTimeout(() => {
        connect();
      }, reconnectInterval);
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WSMessage;
        onMessageRef.current?.(message);
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };
  }, [reconnectInterval]);

  useEffect(() => {
    teardownRef.current = false;
    connect();

    return () => {
      teardownRef.current = true;

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = undefined;
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect]);

  const sendCommand = useCallback((target: string, action: string, params: Record<string, unknown> = {}) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      const message = {
        type: 'command',
        target,
        action,
        params,
      };
      wsRef.current.send(JSON.stringify(message));
    }
  }, []);

  return {
    isConnected,
    sendCommand,
  };
}
