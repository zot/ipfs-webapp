export class P2PWebAppClient {
    constructor() {
        this.ws = null;
        this._peerID = null;
        this._peerKey = null;
        this.requestID = 0;
        this.pending = new Map();
        this.protocolListeners = new Map(); // key: protocol
        this.topicListeners = new Map();
        this.peerChangeListeners = new Map(); // key: topic
        // Message queuing for sequential processing
        this.messageQueue = [];
        this.processingMessage = false;
        // Ack tracking
        this.nextAckNumber = 0;
        this.ackCallbacks = new Map(); // key: ack number
    }
    /**
     * Connect to the WebSocket server and initialize peer identity
     * @param peerKey Optional peer key to restore previous identity
     * @returns Promise resolving to [peerID, peerKey] tuple
     */
    async connect(peerKey) {
        const wsUrl = this.getDefaultWSUrl();
        // First, establish WebSocket connection
        await new Promise((resolve, reject) => {
            this.ws = new WebSocket(wsUrl);
            this.ws.onopen = () => resolve();
            this.ws.onerror = (error) => reject(new Error('WebSocket connection failed'));
            this.ws.onmessage = (event) => this.handleMessage(event);
            this.ws.onclose = () => this.handleClose();
        });
        // Then, initialize peer identity
        const result = await this.sendRequest('peer', peerKey ? { peerkey: peerKey } : {});
        const response = result;
        this._peerID = response.peerid;
        this._peerKey = response.peerkey;
        return [this._peerID, this._peerKey];
    }
    /**
     * Close the WebSocket connection
     */
    close() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }
    /**
     * Start a protocol with a data listener (required before sending)
     * The listener receives (peer, data) for all messages on this protocol
     */
    async start(protocol, onData) {
        if (this.protocolListeners.has(protocol)) {
            throw new Error(`Protocol '${protocol}' already started`);
        }
        await this.sendRequest('start', { protocol });
        this.protocolListeners.set(protocol, onData);
    }
    /**
     * Stop a protocol and remove its listener
     */
    async stop(protocol) {
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
    async send(peer, protocol, data, onAck) {
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
    async subscribe(topic, onData, onPeerChange) {
        this.topicListeners.set(topic, onData);
        if (onPeerChange) {
            this.peerChangeListeners.set(topic, onPeerChange);
        }
        await this.sendRequest('subscribe', { topic });
    }
    /**
     * Publish data to a topic
     */
    async publish(topic, data) {
        await this.sendRequest('publish', { topic, data });
    }
    /**
     * Unsubscribe from a topic and stop monitoring peer changes
     */
    async unsubscribe(topic) {
        this.topicListeners.delete(topic);
        this.peerChangeListeners.delete(topic);
        await this.sendRequest('unsubscribe', { topic });
    }
    /**
     * List peers subscribed to a topic
     */
    async listPeers(topic) {
        const result = await this.sendRequest('listpeers', { topic });
        const response = result;
        return response.peers || [];
    }
    /**
     * Get the current peer ID
     */
    get peerID() {
        return this._peerID;
    }
    /**
     * Get the current peer key
     */
    get peerKey() {
        return this._peerKey;
    }
    // Private methods
    getDefaultWSUrl() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        return `${protocol}//${window.location.host}/ws`;
    }
    handleMessage(event) {
        try {
            const msg = JSON.parse(event.data);
            if (msg.isresponse) {
                // Handle response to our request (not queued, processed immediately)
                const pending = this.pending.get(msg.requestid);
                if (pending) {
                    this.pending.delete(msg.requestid);
                    if (msg.error) {
                        pending.reject(new Error(msg.error.message));
                    }
                    else {
                        pending.resolve(msg.result);
                    }
                }
            }
            else {
                // Queue server-initiated requests for sequential processing
                this.messageQueue.push(msg);
                this.processMessageQueue();
            }
        }
        catch (error) {
            console.error('Failed to handle message:', error);
        }
    }
    async processMessageQueue() {
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
                    }
                    catch (error) {
                        console.error('Error processing server message:', error);
                        // Continue processing next message despite error
                    }
                }
            }
        }
        finally {
            this.processingMessage = false;
        }
    }
    async handleServerRequest(msg) {
        switch (msg.method) {
            case 'peerData':
                if (msg.params) {
                    const req = msg.params;
                    const listener = this.protocolListeners.get(req.protocol);
                    if (listener) {
                        try {
                            await listener(req.peer, req.data);
                        }
                        catch (error) {
                            console.error('Error in peerData listener:', error);
                        }
                    }
                }
                break;
            case 'topicData':
                if (msg.params) {
                    const req = msg.params;
                    const listener = this.topicListeners.get(req.topic);
                    if (listener) {
                        try {
                            await listener(req.peerid, req.data);
                        }
                        catch (error) {
                            console.error('Error in topicData listener:', error);
                        }
                    }
                }
                break;
            case 'peerChange':
                if (msg.params) {
                    const req = msg.params;
                    const listener = this.peerChangeListeners.get(req.topic);
                    if (listener) {
                        try {
                            await listener(req.peerid, req.joined);
                        }
                        catch (error) {
                            console.error('Error in peerChange listener:', error);
                        }
                    }
                }
                break;
            case 'ack':
                if (msg.params) {
                    const req = msg.params;
                    const callback = this.ackCallbacks.get(req.ack);
                    if (callback) {
                        this.ackCallbacks.delete(req.ack); // Remove callback after use
                        try {
                            await callback();
                        }
                        catch (error) {
                            console.error('Error in ack callback:', error);
                        }
                    }
                }
                break;
        }
    }
    handleClose() {
        // Clean up all listeners on disconnect
        this.protocolListeners.clear();
        this.topicListeners.clear();
        this.peerChangeListeners.clear();
        this.ackCallbacks.clear();
        this.messageQueue.length = 0;
        this.processingMessage = false;
    }
    sendRequest(method, params) {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            return Promise.reject(new Error('WebSocket not connected'));
        }
        return new Promise((resolve, reject) => {
            const id = this.requestID++;
            this.pending.set(id, { resolve, reject });
            const msg = {
                requestid: id,
                method,
                params,
                isresponse: false,
            };
            this.ws.send(JSON.stringify(msg));
        });
    }
}
/**
 * Convenience function to create and connect a P2PWebAppClient in one call
 * @param peerKey Optional peer key to restore previous identity
 * @returns Promise resolving to connected P2PWebAppClient instance
 */
export async function connect(peerKey) {
    const client = new P2PWebAppClient();
    await client.connect(peerKey);
    return client;
}
