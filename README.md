# ipfs-webapp

A Go application to host peer-to-peer applications using IPFS and libp2p with a TypeScript client library for browser communication.

## Overview

ipfs-webapp proxies opinionated IPFS and libp2p operations for managed peers, providing a WebSocket-based protocol for easy browser integration. It consists of:

- **Go Server**: Single executable that manages IPFS nodes, libp2p peers, and serves web applications
- **TypeScript Client**: Browser library for communication with the server via WebSocket
- **Demo Application**: Built-in chatroom example demonstrating P2P messaging

## Core Principles

- **SOLID Principles**: Clean architecture with separation of concerns
- **Comprehensive Testing**: Unit tests for all components
- **Minimal Code**: Focused, essential functionality only

## Installation

### Prerequisites

- (to build) Go 1.25 or higher -- or download release binary
- Modern web browser

### Building from Source

```bash
git clone https://github.com/zot/ipfs-webapp
cd ipfs-webapp
make build
```

This will:
1. Install TypeScript dependencies if needed
2. Build the TypeScript client library
3. Copy the library to the demo directory
4. Build the Go binary

The compiled client library (`client.js`, `client.d.ts`) will be available in `internal/commands/demo/` for use in your own projects.

## Commands

### `serve`

Serve a peer-to-peer application from the current directory.

**Directory Structure Required:**
```
.
├── html/          # Website files (must contain index.html)
├── ipfs/          # Content to serve via IPFS
└── storage/       # Server storage (peer keys, datastore)
```

**Usage:**
```bash
./ipfs-webapp serve [--noopen] [-p PORT] [-v | -vv | -vvv]
```

**Flags:**
- `--noopen`: Do not open browser automatically
- `-p, --port PORT`: Specify port to listen on (default: auto-select starting from 10000)
  - If the specified port is unavailable, automatically tries the next port (up to 100 attempts)
  - Example: `./ipfs-webapp serve -p 8080`
- `-v, --verbose`: Verbose output (can be specified multiple times)
  - `-v`: Log peer creation, connections, and messages
  - `-vv`: Additional debug information
  - `-vvv`: Maximum verbosity

This will:
1. Initialize an IPFS node with persistent peer ID
2. Start HTTP server on specified port (or auto-select starting from 10000) with SPA routing support
3. Open default browser to the application (unless --noopen is specified)
4. Expose WebSocket endpoint at `/ws`

**SPA Routing**: The server automatically handles client-side routing by serving `index.html` for routes without file extensions while preserving the URL path. Real files are served normally, and missing files with extensions return 404.

### `demo`

Run the built-in chatroom demo application.

**Requirements:**
- Current directory must be empty

**Usage:**
```bash
mkdir demo-test
cd demo-test
../ipfs-webapp demo [--noopen] [-v | -vv | -vvv]
```

**Flags:**
- `--noopen`: Do not open browser automatically
- `-v, --verbose`: Verbose output (can be specified multiple times)
  - `-v`: Log peer creation, connections, and messages
  - `-vv`: Additional debug information
  - `-vvv`: Maximum verbosity

This extracts the demo chatroom application and runs it.

### `ls`

List files available in the embedded demo directory.

**Usage:**
```bash
./ipfs-webapp ls
```

**Description:**
- Shows all files available in the embedded demo directory
- Displays file names that can be copied with the `cp` command
- Files are listed in alphabetical order
- Useful for discovering what files are available before using `cp`

**Example Output:**
```
Files available in embedded demo directory (7):

client.d.ts
client.js
index.html
index.js
ipfs-webapp-client.js
types.d.ts
types.js
```

### `cp`

Copy files from the embedded demo directory to a target location.

**Usage:**
```bash
./ipfs-webapp cp SOURCE... DEST
```

**Arguments:**
- `SOURCE...`: One or more glob patterns or file names from the embedded demo directory
- `DEST`: Destination directory (will be created if it doesn't exist)

**Examples:**
```bash
# Copy client library files
./ipfs-webapp cp client.js my-project/
./ipfs-webapp cp client.* my-project/         # copies client.js and client.d.ts

# Copy multiple file types
./ipfs-webapp cp *.js *.html my-project/

# Extract specific demo files
./ipfs-webapp cp index.html style.css my-app/html/
```

**Behavior:**
- Supports glob patterns (e.g., `*.js`, `client.*`) for flexible file selection
- Creates destination directory if it doesn't exist
- Preserves original file names (no renaming)
- Validates that at least one file matches the patterns
- Fails if no files match or if destination is not a directory when copying multiple files

**Note**: This command is particularly useful for extracting the TypeScript client library (`client.js`, `client.d.ts`) from the embedded demo files for use in your own projects.

### `version`

Display version information:

```bash
./ipfs-webapp version
```

## TypeScript Client Library

The TypeScript client library is automatically built as part of the main build process. After building, the compiled library is available in `internal/commands/demo/` for use in your applications.

To build just the client library:

```bash
make client
```

Or build from the client directory directly:

```bash
cd pkg/client
npm install
npm run build
```

### Quick Start

```typescript
import { IPFSWebAppClient } from '@ipfs-webapp/client';

const client = new IPFSWebAppClient();

// Connect to server
await client.connect();

// Initialize peer
const [peerID, peerKey] = await client.peer();
console.log('Peer ID:', peerID);

// Start protocol with listener for direct messages
const PROTOCOL = '/my-app/1.0.0';
await client.start(PROTOCOL, (peer, data) => {
  console.log(`Message from ${peer}:`, data);
});

// Send to specific peer
await client.send(targetPeerID, PROTOCOL, { message: 'Hello!' });

// Subscribe to topic for pub/sub
await client.subscribe('my-topic', (senderPeerID, data) => {
  console.log(`Topic message from ${senderPeerID}:`, data);
});

// Publish to topic
await client.publish('my-topic', { message: 'Hello, P2P!' });
```

### API Reference

#### Connection Management

**`connect(url?: string): Promise<void>`**
- Connect to WebSocket server
- Auto-detects URL from browser location if not provided

**`close(): void`**
- Close WebSocket connection
- Cleans up all listeners

#### Peer Operations

**`peer(peerKey?: string): Promise<[string, string]>`**
- Initialize peer with optional existing peer key
- Returns `[peerID, peerKey]` tuple
- Must be called first before other operations
- Can only be called once per connection

**`get peerID(): string | null`**
- Get current peer ID

**`get peerKey(): string | null`**
- Get current peer key

#### Protocol Streams

**`start(protocol: string, onData: (peer: string, data: any) => void | Promise<void>): Promise<void>`**
- Start a protocol and register listener
- Listener receives `(peer, data)` for ALL messages on this protocol
- Must be called before sending on the protocol
- Protocol validation enforced

**`stop(protocol: string): Promise<void>`**
- Stop protocol and remove listener
- Prevents further sending on this protocol

**`send(peer: string, protocol: string, data: any): Promise<void>`**
- Send data to peer on protocol
- Uses (peer, protocol) addressing
- Protocol must be started first
- Server manages stream lifecycle transparently

#### Pub/Sub

**`subscribe(topic: string, onData: (peerID: string, data: any) => void | Promise<void>): Promise<void>`**
- Subscribe to topic
- `onData` callback receives sender peer ID and data
- Listener auto-removed on unsubscribe or disconnect

**`publish(topic: string, data: any): Promise<void>`**
- Publish data to topic

**`unsubscribe(topic: string): Promise<void>`**
- Unsubscribe from topic

**`listPeers(topic: string): Promise<string[]>`**
- Get list of peer IDs subscribed to a topic

**Note**: All callbacks support both synchronous and asynchronous (Promise-returning) functions.

## Protocol Specification

### Message Format

All messages are JSON with the following structure:

```typescript
{
  requestid: number,      // Sequential ID starting at 0
  method?: string,        // Method name for requests
  params?: any,          // Method parameters
  result?: any,          // Response result
  error?: {              // Error information
    code: number,
    message: string
  },
  isresponse: boolean    // true for responses, false for requests
}
```

### Client Request Methods

| Method | Parameters | Response | Description |
|--------|-----------|----------|-------------|
| `peer` | `peerkey?: string` | `{peerid: string, peerkey: string}` | Initialize peer (required first, once only) |
| `start` | `protocol: string` | `null` | Start protocol (required before sending) |
| `stop` | `protocol: string` | `null` | Stop protocol |
| `send` | `peer: string, protocol: string, data: any` | `null` | Send data to peer on protocol |
| `subscribe` | `topic: string` | `null` | Subscribe to topic |
| `publish` | `topic: string, data: any` | `null` | Publish to topic |
| `unsubscribe` | `topic: string` | `null` | Unsubscribe from topic |
| `listPeers` | `topic: string` | `string[]` (peer IDs) | Get peers on topic |

### Server Request Methods

These are sent from server to client:

| Method | Parameters | Description |
|--------|-----------|-------------|
| `peerData` | `peer: string, protocol: string, data: any` | Data from peer on protocol |
| `topicData` | `topic: string, peerid: string, data: any` | Topic message |

## Architecture

### Go Components

```
internal/
├── protocol/       # Message protocol and routing
│   ├── messages.go    # Message type definitions
│   └── handler.go     # Request/response handling
├── peer/          # Peer and connection management
│   └── manager.go     # libp2p peer manager
├── ipfs/          # IPFS node integration
│   └── node.go        # ipfs-lite wrapper
├── server/        # HTTP and WebSocket server
│   ├── server.go      # HTTP server
│   └── websocket.go   # WebSocket handler
└── commands/      # CLI commands
    ├── serve.go       # Serve command
    ├── demo.go        # Demo command
    └── version.go     # Version command
```

### Design Patterns

- **Dependency Inversion**: Protocol handler depends on PeerManager interface
- **Single Responsibility**: Each component has one clear purpose
- **Open/Closed**: Easy to extend with new message types
- **Interface Segregation**: Minimal, focused interfaces

## Development

### Running Tests

```bash
go test ./...
```

### Building TypeScript Client

```bash
cd pkg/client
npm install
npm run build
```

### Project Structure

```
ipfs-webapp/
├── cmd/ipfs-webapp/     # Main entry point
├── internal/            # Go implementation
├── pkg/client/          # TypeScript client library
├── demo/               # Demo application
├── CLAUDE.md           # Project specification
├── plan.md             # Implementation plan
└── README.md           # This file
```

## Examples

### Simple Chat Application

```html
<!DOCTYPE html>
<html>
<head>
  <title>P2P Chat</title>
</head>
<body>
  <div id="messages"></div>
  <input type="text" id="message-input" placeholder="Type a message...">
  <button id="send-button">Send</button>

  <script type="module">
    import { IPFSWebAppClient } from './client.js';

    const client = new IPFSWebAppClient();
    const messagesDiv = document.getElementById('messages');
    const messageInput = document.getElementById('message-input');
    const sendButton = document.getElementById('send-button');

    // Store messages for display when switching views
    const roomMessages = [];

    function addMessage(text, isOwn) {
      const msg = document.createElement('div');
      msg.textContent = `${isOwn ? 'You' : 'Peer'}: ${text}`;
      messagesDiv.appendChild(msg);
    }

    async function init() {
      // Connect and initialize peer
      await client.connect();
      const [peerID, peerKey] = await client.peer();
      console.log('My Peer ID:', peerID);

      // Subscribe to chat topic
      await client.subscribe('chat', (senderPeerID, data) => {
        if (senderPeerID !== client.peerID) {
          // Always store messages
          roomMessages.push({ text: data.message, isOwn: false });
          // Display if in room mode
          addMessage(data.message, false);
        }
      });

      // Send message handler
      async function sendMessage() {
        const text = messageInput.value.trim();
        if (!text) return;

        await client.publish('chat', { message: text });
        roomMessages.push({ text, isOwn: true });
        addMessage(text, true);
        messageInput.value = '';
      }

      sendButton.addEventListener('click', sendMessage);
      messageInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') sendMessage();
      });
    }

    init().catch(console.error);
  </script>
</body>
</html>
```

### Direct Peer Connection

```typescript
const PROTOCOL = '/my-protocol/1.0.0';

// Start protocol with listener (both peers do this)
await client.start(PROTOCOL, (peer, data) => {
  console.log(`Received from ${peer}:`, data);
  // This listener receives ALL messages on this protocol
  // The 'peer' parameter tells you who sent it
});

// Send data to specific peer
// No connection management needed - server handles it transparently
await client.send(targetPeerID, PROTOCOL, { hello: 'world' });

// Stop protocol when done
await client.stop(PROTOCOL);
```

**Note**: The server transparently manages stream lifecycle, retry logic, and reliability. Your application simply uses (peer, protocol) addressing.

## License

MIT

## Contributing

1. Follow SOLID principles
2. Write unit tests for new features
3. Keep code minimal and focused
4. Update documentation
