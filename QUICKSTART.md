# Quick Start Guide

## Try the Demo

The fastest way to see ipfs-webapp in action:

```bash
# Build the application
go build -o ipfs-webapp ./cmd/ipfs-webapp

# Create a test directory
mkdir demo-test
cd demo-test

# Run the demo
../ipfs-webapp demo
```

This will:
1. Extract the chatroom demo application
2. Start the server on a random port
3. Open your browser automatically
4. Display your peer ID

## Test the Chat

1. Open the same URL in multiple browser windows or tabs
2. Each window gets its own peer ID
3. Type messages - they appear in all connected peers
4. Messages are distributed using libp2p pub/sub (gossipsub)

## Create Your Own Application

### 1. Set Up Directory Structure

```bash
mkdir my-p2p-app
cd my-p2p-app
mkdir html ipfs storage
```

### 2. Create HTML File

Create `html/index.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>My P2P App</title>
</head>
<body>
    <h1>My P2P Application</h1>
    <div id="peer-id"></div>
    <div id="messages"></div>

    <script>
        class P2PClient {
            constructor() {
                this.ws = null;
                this.peerID = null;
                this.requestID = 0;
                this.pending = new Map();
                this.protocolListeners = new Map();
                this.topicListeners = new Map();
            }

            async connect() {
                const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
                this.ws = new WebSocket(`${protocol}//${location.host}/ws`);

                return new Promise((resolve, reject) => {
                    this.ws.onopen = () => resolve();
                    this.ws.onerror = reject;
                    this.ws.onmessage = (e) => this.handleMessage(e);
                });
            }

            handleMessage(event) {
                const msg = JSON.parse(event.data);

                if (msg.isresponse) {
                    const resolver = this.pending.get(msg.requestid);
                    if (resolver) {
                        this.pending.delete(msg.requestid);
                        if (msg.error) {
                            resolver.reject(new Error(msg.error.message));
                        } else {
                            resolver.resolve(msg.result);
                        }
                    }
                } else if (msg.method === 'peerData') {
                    const data = JSON.parse(msg.params);
                    const listener = this.protocolListeners.get(data.protocol);
                    if (listener) {
                        listener(data.peer, data.data);
                    }
                } else if (msg.method === 'topicData') {
                    const data = JSON.parse(msg.params);
                    const listener = this.topicListeners.get(data.topic);
                    if (listener) {
                        listener(data.peerid, data.data);
                    }
                }
            }

            send(method, params) {
                return new Promise((resolve, reject) => {
                    const id = this.requestID++;
                    this.pending.set(id, { resolve, reject });
                    this.ws.send(JSON.stringify({
                        requestid: id,
                        method,
                        params,
                        isresponse: false
                    }));
                });
            }

            async peer(id) {
                const result = await this.send('peer', id ? { peerid: id } : {});
                const parsed = JSON.parse(result);
                this.peerID = parsed.value;
                return this.peerID;
            }

            async start(protocol, onData) {
                this.protocolListeners.set(protocol, onData);
                await this.send('start', { protocol });
            }

            async sendToPeer(peer, protocol, data) {
                await this.send('send', { peer, protocol, data });
            }

            async subscribe(topic, onData) {
                this.topicListeners.set(topic, onData);
                await this.send('subscribe', { topic });
            }

            async publish(topic, data) {
                await this.send('publish', { topic, data });
            }
        }

        // Initialize
        const client = new P2PClient();

        (async () => {
            await client.connect();
            const peerID = await client.peer();

            document.getElementById('peer-id').textContent = `Peer ID: ${peerID}`;

            await client.subscribe('test', (from, data) => {
                const div = document.createElement('div');
                div.textContent = `${from}: ${JSON.stringify(data)}`;
                document.getElementById('messages').appendChild(div);
            });

            // Publish test message
            await client.publish('test', { hello: 'world' });
        })();
    </script>
</body>
</html>
```

### 3. Run Your Application

```bash
../ipfs-webapp serve
```

## Protocol Reference

### Client → Server Messages

```javascript
// Initialize peer
await client.send('peer', {});
// Response: { value: "peer-id-string" }

// Start protocol (required before sending to peers)
await client.send('start', { protocol: '/my-protocol/1.0.0' });
// Response: null

// Send data to peer on protocol
await client.send('send', {
    peer: 'target-peer-id',
    protocol: '/my-protocol/1.0.0',
    data: { any: 'data' }
});
// Response: null

// Stop protocol
await client.send('stop', { protocol: '/my-protocol/1.0.0' });
// Response: null

// Subscribe to topic
await client.send('subscribe', { topic: 'my-topic' });
// Response: null

// Publish to topic
await client.send('publish', {
    topic: 'my-topic',
    data: { your: 'data' }
});
// Response: null
```

### Server → Client Messages

```javascript
// Peer data (received from peer on protocol)
{
    method: 'peerData',
    params: {
        peer: 'sender-peer-id',
        protocol: '/my-protocol/1.0.0',
        data: { the: 'data' }
    }
}

// Topic data (received via subscription)
{
    method: 'topicData',
    params: {
        topic: 'my-topic',
        peerid: 'sender-peer-id',
        data: { the: 'data' }
    }
}
```

## Next Steps

- See [README.md](README.md) for complete API reference
- See [CLAUDE.md](CLAUDE.md) for protocol specification
- Check [plan.md](plan.md) for implementation details
- Read TypeScript client in `pkg/client/src/` for full client API

## Troubleshooting

**Browser won't open automatically**
- Manually navigate to the URL shown in terminal (e.g., `http://localhost:8080`)

**Connection fails**
- Check firewall settings
- Ensure port is not already in use
- Check browser console for errors

**Peers can't connect**
- On same machine: They should connect automatically via localhost
- Different machines: You'll need to configure libp2p addresses and potentially NAT traversal

## Architecture Overview

```
Browser (TypeScript)     Go Server              libp2p Network
     |                       |                        |
     |--WebSocket----------->|                        |
     |   peer request        |                        |
     |<--peer ID-------------|                        |
     |                       |                        |
     |--start protocol------>|                        |
     |<--success-------------|                        |
     |                       |                        |
     |--send to peer-------->|                        |
     |   (peer, protocol)    |--Open/reuse stream---->|
     |<--success-------------|   send data            |
     |                       |                        |
     |<--peerData------------|<--Receive on stream----|
     |   (peer, data)        |   from peer            |
     |                       |                        |
     |--subscribe----------->|                        |
     |   to topic            |--Join gossipsub------->|
     |<--success-------------|        topic           |
     |                       |                        |
     |--publish------------->|                        |
     |   data                |--Broadcast------------>|
     |                       |   to peers             |
     |                       |                        |
     |<--topicData-----------|<--Receive message------|
     |   from peer           |   from network         |
```
