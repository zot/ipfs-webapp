import {
  Message,
  StringResponse,
  PeerResponse,
  ListPeersResponse,
  ProtocolDataCallback,
  TopicDataCallback,
  PeerChangeCallback,
  AckCallback,
  PeerDataRequest,
  TopicDataRequest,
  PeerChangeRequest,
  AckRequest,
} from './types.js';

interface PendingRequest {
  resolve: (value: any) => void;
  reject: (error: Error) => void;
}

export class IPFSWebAppClient {
  private ws: WebSocket | null = null;
  private _peerID: string | null = null;
  private _peerKey: string | null = null;
  private requestID: number = 0;
  private pending: Map<number, PendingRequest> = new Map();
  private protocolListeners: Map<string, ProtocolDataCallback> = new Map(); // key: protocol
  private topicListeners: Map<string, TopicDataCallback> = new Map();
  private peerChangeListeners: Map<string, PeerChangeCallback> = new Map(); // key: topic

  // Message queuing for sequential processing
  private messageQueue: Message[] = [];
  private processingMessage: boolean = false;

  // Ack tracking
  private nextAckNumber: number = 0;
  private ackCallbacks: Map<number, AckCallback> = new Map(); // key: ack number

  /**
   * Connect to the WebSocket server and initialize peer identity
   * @param peerKey Optional peer key to restore previous identity
   * @returns Promise resolving to [peerID, peerKey] tuple
   */
  async connect(peerKey?: string): Promise<[string, string]> {
    const wsUrl = this.getDefaultWSUrl();

    // First, establish WebSocket connection
    await new Promise<void>((resolve, reject) => {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => resolve();
      this.ws.onerror = (error) => reject(new Error('WebSocket connection failed'));
      this.ws.onmessage = (event) => this.handleMessage(event);
      this.ws.onclose = () => this.handleClose();
    });

    // Then, initialize peer identity
    const result = await this.sendRequest('peer', peerKey ? { peerkey: peerKey } : {});
    const response = result as PeerResponse;
    this._peerID = response.peerid;
    this._peerKey = response.peerkey;
    return [this._peerID, this._peerKey];
  }

  /**
   * Close the WebSocket connection
   */
  close(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  /**
   * Start a protocol with a data listener (required before sending)
   * The listener receives (peer, data) for all messages on this protocol
   */
  async start(protocol: string, onData: ProtocolDataCallback): Promise<void> {
    if (this.protocolListeners.has(protocol)) {
      throw new Error(`Protocol '${protocol}' already started`);
    }
    await this.sendRequest('start', { protocol });
    this.protocolListeners.set(protocol, onData);
  }

  /**
   * Stop a protocol and remove its listener
   */
  async stop(protocol: string): Promise<void> {
    if (!this.protocolListeners.has(protocol)) {
      throw new Error(`Protocol '${protocol}' not started`);
    }
    await this.sendRequest('stop', { protocol });
    this.protocolListeners.delete(protocol);
  }

  /**
   * Send data to a peer on a protocol
   * @param peer Target peer ID
   * @param protocol Protocol name
   * @param data Data to send
   * @param onAck Optional callback invoked when delivery is confirmed
   */
  async send(peer: string, protocol: string, data: any, onAck?: AckCallback): Promise<void> {
    if (!this.protocolListeners.has(protocol)) {
      throw new Error(`Cannot send on protocol '${protocol}': protocol not started. Call start() first.`);
    }

    let ackNum = -1; // Default: no ack requested
    if (onAck) {
      // Assign ack number and store callback
      ackNum = this.nextAckNumber++;
      this.ackCallbacks.set(ackNum, onAck);
    }

    await this.sendRequest('send', { peer, protocol, data, ack: ackNum });
  }

  /**
   * Subscribe to a topic with data listener and optional peer change listener
   * Automatically monitors the topic for peer join/leave events if onPeerChange is provided
   */
  async subscribe(topic: string, onData: TopicDataCallback, onPeerChange?: PeerChangeCallback): Promise<void> {
    this.topicListeners.set(topic, onData);
    if (onPeerChange) {
      this.peerChangeListeners.set(topic, onPeerChange);
    }
    await this.sendRequest('subscribe', { topic });
  }

  /**
   * Publish data to a topic
   */
  async publish(topic: string, data: any): Promise<void> {
    await this.sendRequest('publish', { topic, data });
  }

  /**
   * Unsubscribe from a topic and stop monitoring peer changes
   */
  async unsubscribe(topic: string): Promise<void> {
    this.topicListeners.delete(topic);
    this.peerChangeListeners.delete(topic);
    await this.sendRequest('unsubscribe', { topic });
  }

  /**
   * List peers subscribed to a topic
   */
  async listPeers(topic: string): Promise<string[]> {
    const result = await this.sendRequest('listpeers', { topic });
    const response = result as ListPeersResponse;
    return response.peers || [];
  }

  /**
   * Get the current peer ID
   */
  get peerID(): string | null {
    return this._peerID;
  }

  /**
   * Get the current peer key
   */
  get peerKey(): string | null {
    return this._peerKey;
  }

  // Private methods

  private getDefaultWSUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/ws`;
  }

  private handleMessage(event: MessageEvent): void {
    try {
      const msg: Message = JSON.parse(event.data);

      if (msg.isresponse) {
        // Handle response to our request (not queued, processed immediately)
        const pending = this.pending.get(msg.requestid);
        if (pending) {
          this.pending.delete(msg.requestid);
          if (msg.error) {
            pending.reject(new Error(msg.error.message));
          } else {
            pending.resolve(msg.result);
          }
        }
      } else {
        // Queue server-initiated requests for sequential processing
        this.messageQueue.push(msg);
        this.processMessageQueue();
      }
    } catch (error) {
      console.error('Failed to handle message:', error);
    }
  }

  private async processMessageQueue(): Promise<void> {
    // If already processing, return (next message will be processed when current finishes)
    if (this.processingMessage) {
      return;
    }

    this.processingMessage = true;

    try {
      while (this.messageQueue.length > 0) {
        const msg = this.messageQueue.shift();
        if (msg) {
          try {
            await this.handleServerRequest(msg);
          } catch (error) {
            console.error('Error processing server message:', error);
            // Continue processing next message despite error
          }
        }
      }
    } finally {
      this.processingMessage = false;
    }
  }

  private async handleServerRequest(msg: Message): Promise<void> {
    switch (msg.method) {
      case 'peerData':
        if (msg.params) {
          const req = msg.params as PeerDataRequest;
          const listener = this.protocolListeners.get(req.protocol);
          if (listener) {
            try {
              await listener(req.peer, req.data);
            } catch (error) {
              console.error('Error in peerData listener:', error);
            }
          }
        }
        break;

      case 'topicData':
        if (msg.params) {
          const req = msg.params as TopicDataRequest;
          const listener = this.topicListeners.get(req.topic);
          if (listener) {
            try {
              await listener(req.peerid, req.data);
            } catch (error) {
              console.error('Error in topicData listener:', error);
            }
          }
        }
        break;

      case 'peerChange':
        if (msg.params) {
          const req = msg.params as PeerChangeRequest;
          const listener = this.peerChangeListeners.get(req.topic);
          if (listener) {
            try {
              await listener(req.peerid, req.joined);
            } catch (error) {
              console.error('Error in peerChange listener:', error);
            }
          }
        }
        break;

      case 'ack':
        if (msg.params) {
          const req = msg.params as AckRequest;
          const callback = this.ackCallbacks.get(req.ack);
          if (callback) {
            this.ackCallbacks.delete(req.ack); // Remove callback after use
            try {
              await callback();
            } catch (error) {
              console.error('Error in ack callback:', error);
            }
          }
        }
        break;
    }
  }

  private handleClose(): void {
    // Clean up all listeners on disconnect
    this.protocolListeners.clear();
    this.topicListeners.clear();
    this.peerChangeListeners.clear();
    this.ackCallbacks.clear();
    this.messageQueue.length = 0;
    this.processingMessage = false;
  }

  private sendRequest(method: string, params: any): Promise<any> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return Promise.reject(new Error('WebSocket not connected'));
    }

    return new Promise((resolve, reject) => {
      const id = this.requestID++;
      this.pending.set(id, { resolve, reject });

      const msg: Message = {
        requestid: id,
        method,
        params,
        isresponse: false,
      };

      this.ws!.send(JSON.stringify(msg));
    });
  }
}
