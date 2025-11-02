export class IPFSWebAppClient {
    constructor() {
        this.ws = null;
        this._peerID = null;
        this.requestID = 0;
        this.pending = new Map();
        this.protocolListeners = new Map(); // key: protocol
        this.topicListeners = new Map();
        // Message queuing for sequential processing
        this.messageQueue = [];
        this.processingMessage = false;
    }
    /**
     * Connect to the WebSocket server
     */
    async connect(url) {
        const wsUrl = url || this.getDefaultWSUrl();
        return new Promise((resolve, reject) => {
            this.ws = new WebSocket(wsUrl);
            this.ws.onopen = () => resolve();
            this.ws.onerror = (error) => reject(new Error('WebSocket connection failed'));
            this.ws.onmessage = (event) => this.handleMessage(event);
            this.ws.onclose = () => this.handleClose();
        });
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
     * Initialize or retrieve peer ID
     */
    async peer(peerID) {
        const result = await this.sendRequest('peer', peerID ? { peerid: peerID } : {});
        const response = result;
        this._peerID = response.value;
        return this._peerID;
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
     */
    async send(peer, protocol, data) {
        if (!this.protocolListeners.has(protocol)) {
            throw new Error(`Cannot send on protocol '${protocol}': protocol not started. Call start() first.`);
        }
        await this.sendRequest('send', { peer, protocol, data });
    }
    /**
     * Subscribe to a topic with data listener
     */
    async subscribe(topic, onData) {
        this.topicListeners.set(topic, onData);
        await this.sendRequest('subscribe', { topic });
    }
    /**
     * Publish data to a topic
     */
    async publish(topic, data) {
        await this.sendRequest('publish', { topic, data });
    }
    /**
     * Unsubscribe from a topic
     */
    async unsubscribe(topic) {
        this.topicListeners.delete(topic);
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
                        listener(req.peerid, req.data);
                    }
                }
                break;
        }
    }
    handleClose() {
        // Clean up all listeners on disconnect
        this.protocolListeners.clear();
        this.topicListeners.clear();
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
