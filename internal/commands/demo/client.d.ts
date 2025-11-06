import { ProtocolDataCallback, TopicDataCallback, PeerChangeCallback, AckCallback } from './types.js';
export declare class P2PWebAppClient {
    private ws;
    private _peerID;
    private _peerKey;
    private requestID;
    private pending;
    private protocolListeners;
    private topicListeners;
    private peerChangeListeners;
    private messageQueue;
    private processingMessage;
    private nextAckNumber;
    private ackCallbacks;
    /**
     * Connect to the WebSocket server and initialize peer identity
     * @param peerKey Optional peer key to restore previous identity
     * @returns Promise resolving to [peerID, peerKey] tuple
     */
    connect(peerKey?: string): Promise<[string, string]>;
    /**
     * Close the WebSocket connection
     */
    close(): void;
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
     * @param peer Target peer ID
     * @param protocol Protocol name
     * @param data Data to send
     * @param onAck Optional callback invoked when delivery is confirmed
     */
    send(peer: string, protocol: string, data: any, onAck?: AckCallback): Promise<void>;
    /**
     * Subscribe to a topic with data listener and optional peer change listener
     * Automatically monitors the topic for peer join/leave events if onPeerChange is provided
     */
    subscribe(topic: string, onData: TopicDataCallback, onPeerChange?: PeerChangeCallback): Promise<void>;
    /**
     * Publish data to a topic
     */
    publish(topic: string, data: any): Promise<void>;
    /**
     * Unsubscribe from a topic and stop monitoring peer changes
     */
    unsubscribe(topic: string): Promise<void>;
    /**
     * List peers subscribed to a topic
     */
    listPeers(topic: string): Promise<string[]>;
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
/**
 * Convenience function to create and connect a P2PWebAppClient in one call
 * @param peerKey Optional peer key to restore previous identity
 * @returns Promise resolving to connected P2PWebAppClient instance
 */
export declare function connect(peerKey?: string): Promise<P2PWebAppClient>;
