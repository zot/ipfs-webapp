import { ProtocolDataCallback, TopicDataCallback, TopicJoinedCallback, TopicLeftCallback } from './types.js';
export declare class IPFSWebAppClient {
    private ws;
    private _peerID;
    private _peerKey;
    private requestID;
    private pending;
    private protocolListeners;
    private topicListeners;
    private topicJoinedListeners;
    private topicLeftListeners;
    private messageQueue;
    private processingMessage;
    /**
     * Connect to the WebSocket server
     */
    connect(url?: string): Promise<void>;
    /**
     * Close the WebSocket connection
     */
    close(): void;
    /**
     * Initialize or retrieve peer ID
     * Returns an array [peerID, peerKey]
     */
    peer(peerKey?: string): Promise<[string, string]>;
    /**
     * Start a protocol with a data listener (required before sending)
     * The listener receives (peer, data) for all messages on this protocol
     */
    start(protocol: string, onData: ProtocolDataCallback): Promise<void>;
    /**
     * Stop a protocol and remove its listener
     */
    stop(protocol: string): Promise<void>;
    /**
     * Send data to a peer on a protocol
     */
    send(peer: string, protocol: string, data: any): Promise<void>;
    /**
     * Subscribe to a topic with data listener
     */
    subscribe(topic: string, onData: TopicDataCallback): Promise<void>;
    /**
     * Publish data to a topic
     */
    publish(topic: string, data: any): Promise<void>;
    /**
     * Unsubscribe from a topic
     */
    unsubscribe(topic: string): Promise<void>;
    /**
     * List peers subscribed to a topic
     */
    listPeers(topic: string): Promise<string[]>;
    /**
     * Start monitoring a topic for peer join/leave events
     */
    monitor(topic: string, onJoined: TopicJoinedCallback, onLeft: TopicLeftCallback): Promise<void>;
    /**
     * Stop monitoring a topic for peer join/leave events
     */
    stopMonitor(topic: string): Promise<void>;
    /**
     * Get the current peer ID
     */
    get peerID(): string | null;
    /**
     * Get the current peer key
     */
    get peerKey(): string | null;
    private getDefaultWSUrl;
    private handleMessage;
    private processMessageQueue;
    private handleServerRequest;
    private handleClose;
    private sendRequest;
}
