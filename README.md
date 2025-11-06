# Build Peer-to-Peer Web Apps in Minutes

**p2p-webapp** lets you build real-time, peer-to-peer web applications with just JavaScript. No backend servers. No hosting costs. No signaling servers. No complex setup.

```typescript
// Connect to a peer-to-peer chat room in 3 lines
const client = await connect();
await client.subscribe('my-chat-topic', (peerID, data) => showMessage(data.text));
await client.publish('my-chat-topic', { text: 'Hello, P2P world!' });
```

## Why Build P2P Web Apps?

- ✨ **No Backend Required** - Your users connect directly to each other
- 💰 **Zero Hosting Costs** - No servers to maintain or pay for
- 🔌 **No Signaling Servers** - Native peers, not browser-based WebRTC
- 🚀 **Real-Time by Default** - Messages arrive instantly, no polling needed
- 🔒 **Privacy-First** - Data travels directly between peers
- 🌍 **Resilient** - No single point of failure
- 📦 **Simple** - Just JavaScript, works like any other web app

## What Can You Build?

- **Chat Applications** - Public rooms and private messages
- **Collaborative Tools** - Real-time editors, whiteboards, kanban boards
- **Multiplayer Games** - Card games, turn-based games, party games
- **Social Apps** - Micro-communities, forums, social networks
- **File Sharing** - Direct file transfers between users
- **Live Feeds** - News, updates, real-time dashboards
- **Voting Systems** - Polls, surveys, decision-making tools

See it in action: Run `./p2p-webapp` to try a working chatroom with direct messaging!

## Quick Start

### 1. Get the Tool

Download the latest release or build from source:

```bash
git clone https://github.com/zot/p2p-webapp
cd p2p-webapp
make build
```

### 2. Try the Demo

```bash
./p2p-webapp
```

The bundled binary ships with a complete chatroom demo. Open the URL in multiple browser tabs or windows to see peers connecting to each other. Try the group chat and direct messages!

### 3. Build Your Own App

Extract the client library:

```bash
./p2p-webapp cp 'client.*' my-app/
```

Create your `index.html`:

```html
<!DOCTYPE html>
<html>
<head>
  <title>My P2P App</title>
</head>
<body>
  <div id="messages"></div>
  <input id="input" placeholder="Type a message...">
  <button id="send">Send</button>

  <script type="module">
    import { connect } from './client.js';

    const messages = document.getElementById('messages');
    const input = document.getElementById('input');
    const send = document.getElementById('send');

    // Connect to server
    const client = await connect();

    // Subscribe to chat topic with peer tracking
    await client.subscribe('my-chat-topic',
      // Message callback
      (peerID, data) => {
        const msg = document.createElement('div');
        msg.textContent = `${peerID.slice(0, 8)}: ${data.text}`;
        messages.appendChild(msg);
      },
      // Peer join/leave callback
      (peerID, joined) => {
        console.log(`${peerID} ${joined ? 'joined' : 'left'}`);
      }
    );

    // Send messages
    send.onclick = async () => {
      if (input.value.trim()) {
        await client.publish('my-chat-topic', { text: input.value });
        input.value = '';
      }
    };
  </script>
</body>
</html>
```

Set up your project structure:

```bash
mkdir -p html ipfs storage
mv index.html html/
mv client.* html/
```

Run your app:

```bash
./p2p-webapp --dir .
```

That's it! Your P2P app is running. Open multiple browser tabs to see peers connecting.

**Tip:** You can also bundle your app into a standalone binary with `./p2p-webapp bundle . -o my-app`, then users can just run `./my-app` without needing the directory structure.

## API Overview

### Connection & Setup

```typescript
import { connect } from './client.js';

// Connect to server and initialize peer
const client = await connect();

// Access peer identity
console.log(client.peerID, client.peerKey);

// Save peerKey to maintain identity across sessions
localStorage.setItem('peerKey', client.peerKey);
```

### Group Chat (Pub/Sub)

```typescript
// Join a chat room
await client.subscribe('my-room-name',
  // Receive messages
  (peerID, data) => console.log(`${peerID}: ${data.message}`),
  // Track who's online (optional)
  (peerID, joined) => console.log(`${peerID} ${joined ? 'joined' : 'left'}`)
);

// Send to everyone in the room
await client.publish('my-room-name', { message: 'Hello everyone!' });

// See who's in the room
const peers = await client.listPeers('my-room-name');
```

### Direct Messages (Peer-to-Peer)

```typescript
// Listen for direct messages
await client.start('/my-app/dm/1.0.0', (fromPeerID, data) => {
  console.log(`DM from ${fromPeerID}:`, data.message);
});

// Send to a specific peer
await client.send(targetPeerID, '/my-app/dm/1.0.0', {
  message: 'Hi there!'
});

// Get delivery confirmation (optional)
await client.send(targetPeerID, '/my-app/dm/1.0.0', data,
  () => console.log('Message delivered!')
);
```

### Cleaning Up

```typescript
// Leave a chat room
await client.unsubscribe('my-room-name');

// Stop receiving direct messages
await client.stop('/my-app/dm/1.0.0');

// Disconnect
client.close();
```

## Complete Examples

### Example 1: Simple Chat Room

```html
<!DOCTYPE html>
<html>
<head><title>P2P Chat</title></head>
<body>
  <div id="messages"></div>
  <input id="input" type="text">
  <button id="send">Send</button>

  <script type="module">
    import { connect } from './client.js';

    const messagesDiv = document.getElementById('messages');
    const input = document.getElementById('input');

    const client = await connect();

    await client.subscribe('my-chat-topic', (peerID, data) => {
      const msg = document.createElement('div');
      msg.className = peerID === client.peerID ? 'own' : 'other';
      msg.textContent = data.text;
      messagesDiv.appendChild(msg);
    });

    document.getElementById('send').onclick = async () => {
      if (input.value.trim()) {
        await client.publish('my-chat-topic', { text: input.value });
        input.value = '';
      }
    };
  </script>
</body>
</html>
```

### Example 2: Private Messaging

```typescript
const PROTOCOL = '/my-app/1.0.0';

// Listen for incoming messages
await client.start(PROTOCOL, (fromPeer, data) => {
  showNotification(`Message from ${fromPeer}: ${data.text}`);
});

// Send private message
async function sendPrivateMessage(toPeerID, text) {
  await client.send(
    toPeerID,
    PROTOCOL,
    { text },
    () => showStatus('Delivered ✓')
  );
}
```

### Example 3: Presence & User List

```typescript
const onlinePeers = new Set();

await client.subscribe('my-app',
  // Handle messages
  (peerID, data) => handleMessage(peerID, data),
  // Track online users
  (peerID, joined) => {
    if (joined) {
      onlinePeers.add(peerID);
      showUserJoined(peerID);
    } else {
      onlinePeers.delete(peerID);
      showUserLeft(peerID);
    }
    updateUserList(Array.from(onlinePeers));
  }
);

// Get current users
const currentPeers = await client.listPeers('my-app');
currentPeers.forEach(p => onlinePeers.add(p));
updateUserList(currentPeers);
```

## Project Structure

Your P2P web app needs this structure:

```
my-app/
├── html/              # Your web application
│   ├── index.html     # Main page
│   ├── client.js      # P2P client library (copy from demo)
│   ├── client.d.ts    # TypeScript definitions
│   └── ...            # Your other web files
├── ipfs/              # Optional: IPFS content
└── storage/           # Created automatically: peer data
```

## Commands

### Default: Run Your P2P App

Running `p2p-webapp` without a subcommand starts the server. Two modes:

**1. Bundled mode (default):**
```bash
./p2p-webapp [--noopen] [-p PORT] [-v]
```

Serves directly from the bundled site without extraction. Efficient - no filesystem needed!
- Requires bundled binary (the built binary ships bundled by default)
- Creates `.p2p-webapp-storage` in current directory for peer data
- Perfect for quick testing and distribution

**2. Directory mode:**
```bash
./p2p-webapp --dir PATH [--noopen] [-p PORT] [-v]
```

Serves from a filesystem directory. Use this for:
- Development with your own site
- After extracting with `extract` command
- Directory must contain `html/` with `index.html`

Both modes automatically:
- Initialize P2P networking
- Open your default browser
- Support client-side routing (SPA)
- Serve on localhost (default port 10000)

**Options:**
- `--dir PATH` - Serve from directory instead of bundle
- `--noopen` - Don't open browser
- `-p 8080` - Use specific port
- `-v` - Show connection logs (use `-vv` or `-vvv` for more detail)

### `extract` - Extract Bundled Site

```bash
mkdir test && cd test
../p2p-webapp extract
```

Extracts the bundled site to the current directory (must be empty). The default bundle contains a chatroom demo. After extraction, run `./p2p-webapp --dir .` to start the server.

### `bundle` - Create Standalone App

```bash
./p2p-webapp bundle my-site -o my-app
```

Bundles your site into a standalone binary that can be distributed. No compilation tools needed! The bundled binary contains everything users need to run your P2P app.

**Requirements:**
- Site directory with `html/` containing `index.html`
- Optional `ipfs/` and `storage/` directories

Users can run your bundled app directly:
- `./my-app` - Run immediately from bundle
- `./my-app extract` - Extract to filesystem first (optional)

### `cp` - Copy Client Library

```bash
./p2p-webapp cp 'client.*' my-app/html/
```

Copies files from the bundled site to your project. Works directly on the bundled content without needing extraction. Supports glob patterns:
- `client.*` - Both .js and .d.ts files
- `*.js` - All JavaScript files
- `index.html` - Specific files

### `ls` - List Available Files

```bash
./p2p-webapp ls
```

Shows all files available in the bundled site. Reads directly from the bundle without extraction.

## How It Works

When you run `p2p-webapp`:

1. The bundled site is served directly from the binary (or from filesystem with `--dir`)
2. A local server starts on your machine
3. Your web app connects via WebSocket
4. The server manages IPFS and libp2p networking
5. Your JavaScript code uses simple APIs
6. Messages flow directly between users' browsers

```
Browser A ←→ Local Server A ←→ P2P Network ←→ Local Server B ←→ Browser B
```

Each user runs their own local server (or shares one). No central server needed!

**Want to see how it works?** Extract the demo with `./p2p-webapp extract` to examine the source code.

## Advanced Features

### Persistent Identity

Save the peer key to keep the same identity across sessions:

```typescript
const savedKey = localStorage.getItem('peerKey');
const client = await connect(savedKey);
if (!savedKey) {
  localStorage.setItem('peerKey', client.peerKey);
}
```

### Protocol Versioning

Use semantic versioning in your protocol names:

```typescript
const PROTOCOL_V1 = '/my-app/messages/1.0.0';
const PROTOCOL_V2 = '/my-app/messages/2.0.0';

// Support both versions
await client.start(PROTOCOL_V1, handleV1Message);
await client.start(PROTOCOL_V2, handleV2Message);
```

### Error Handling

All methods return Promises that reject on error:

```typescript
try {
  await client.publish('my-topic', data);
} catch (error) {
  console.error('Failed to send:', error);
  showRetryButton();
}
```

### Async Callbacks

Callbacks can be async if you need to do work:

```typescript
await client.subscribe('my-events', async (peerID, data) => {
  // Do async work like saving to IndexedDB
  await database.save(data);
  updateUI();
});
```

## TypeScript Support

Full TypeScript definitions included:

```typescript
import { connect } from './client.js';

const client = await connect();

// All methods are fully typed
await client.subscribe('my-topic',
  (peerID: string, data: any) => {
    // peerID is typed as string
  },
  (peerID: string, joined: boolean) => {
    // joined is typed as boolean
  }
);
```

## FAQ

**Q: Do my users need to install anything?**
A: Each user runs `p2p-webapp` on their own computer.

**Q: Why don't I need signaling servers like browser WebRTC apps?**
A: Browser-only P2P apps using WebRTC require [signaling servers to exchange SDP connection information](https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API/Signaling_and_video_calling) before peers can connect. This means "peer-to-peer" browser apps still depend on a central server infrastructure.

p2p-webapp solves this by running native libp2p peers outside the browser. These peers connect directly using protocols like mDNS (local discovery) and DHT (global discovery) without needing a signaling intermediary. Your browser talks to its local peer via WebSocket, and the local peer handles all P2P networking natively—using the same battle-hardened code that powers systems like the Ethereum blockchain.

**Q: How do users discover each other?**
A: The IPFS/libp2p network handles peer discovery automatically. Users on the same topic find each other.

**Q: Can I deploy this to production?**
A: Currently, `p2p-webapp` is designed for local development and small-scale deployments. Each user needs to run their own instance or connect to a trusted instance.

**Q: What browsers are supported?**
A: All modern browsers with WebSocket support (Chrome, Firefox, Safari, Edge).

**Q: Is the connection secure?**
A: libp2p handles encryption between peers. The WebSocket connection to localhost is not encrypted by default.

**Q: Can I use this with React/Vue/Svelte?**
A: Yes! The client library is framework-agnostic. Just import it and use it in your components.

**Q: How many peers can connect?**
A: This depends on network conditions and the IPFS/libp2p configuration. Small to medium groups work well.

## API Reference

### Connection

| Function | Description |
|----------|-------------|
| `connect(peerKey?)` | Create and connect a client (returns P2PWebAppClient) |

### Client Methods

| Method | Description |
|--------|-------------|
| `subscribe(topic, onData, onPeerChange?)` | Join chat room with optional presence |
| `publish(topic, data)` | Send to all room members |
| `listPeers(topic)` | Get list of room members |
| `start(protocol, onData)` | Listen for direct messages |
| `send(peer, protocol, data, onAck?)` | Send direct message with optional confirmation |
| `unsubscribe(topic)` | Leave chat room |
| `stop(protocol)` | Stop listening for direct messages |
| `close()` | Disconnect |

### Client Properties

| Property | Description |
|----------|-------------|
| `peerID` | Your peer ID (read-only) |
| `peerKey` | Your peer key (read-only, save this!) |

## Building from Source

### Prerequisites

- Go 1.25+ (only for building)
- Node.js 18+ (only for building client library)

### Build Steps

```bash
git clone https://github.com/zot/p2p-webapp
cd p2p-webapp
make build
```

This builds:
1. TypeScript client library (`pkg/client/`)
2. Temporary Go server binary
3. Final bundled binary (`./p2p-webapp`) with demo site included

The final binary ships with the demo bundled and ready to extract.

### Development

```bash
# Build just the client library
make client

# Clean build artifacts
make clean

# Run tests
go test ./...
```

## Protocol Details

For those interested in the underlying protocol, see [CLAUDE.md](CLAUDE.md) for the complete specification.

The WebSocket protocol uses JSON-RPC style messages. The TypeScript client handles all protocol details automatically.

## License

MIT

## Contributing

Contributions welcome! This project follows SOLID principles and emphasizes simplicity. Please:

1. Keep code minimal and focused
2. Add tests for new features
3. Update documentation
4. Follow the existing code style

## Links

- GitHub: https://github.com/zot/p2p-webapp
- IPFS: https://ipfs.io
- libp2p: https://libp2p.io

---

**Start building your P2P web app today!** 🚀

```bash
./p2p-webapp                    # Try the demo (runs from bundle)
./p2p-webapp extract            # Extract demo to examine code
./p2p-webapp --dir .            # Run your own site
./p2p-webapp bundle . -o my-app # Create standalone binary
```
