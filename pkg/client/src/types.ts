// Message types matching the Go protocol definitions

export interface Message {
  requestid: number;
  method?: string;
  params?: any;
  result?: any;
  error?: ErrorResponse;
  isresponse: boolean;
}

export interface ErrorResponse {
  code: number;
  message: string;
}

export interface StringResponse {
  value: string;
}

export interface PeerResponse {
  peerid: string;
  peerkey: string;
}

// Client request message types

export interface PeerRequest {
  peerkey?: string;
}

export interface StartRequest {
  protocol: string;
}

export interface StopRequest {
  protocol: string;
}

export interface SendRequest {
  peer: string;
  protocol: string;
  data: any;
}

export interface SubscribeRequest {
  topic: string;
}

export interface PublishRequest {
  topic: string;
  data: any;
}

export interface UnsubscribeRequest {
  topic: string;
}

export interface ListPeersRequest {
  topic: string;
}

export interface ListPeersResponse {
  peers: string[];
}

// Server request message types

export interface PeerDataRequest {
  peer: string;
  protocol: string;
  data: any;
}

export interface TopicDataRequest {
  topic: string;
  peerid: string;
  data: any;
}

// Callback types

// Callbacks can be sync or async for flexibility
export type ProtocolDataCallback = (peer: string, data: any) => void | Promise<void>;
export type TopicDataCallback = (peerID: string, data: any) => void | Promise<void>;
