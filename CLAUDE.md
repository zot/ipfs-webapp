# ipfs-webapp
- A Go application to host peer-to-peer applications
- Proxies opinionated IPFS and libp2p operations for managed peers
- Provides a TypeScript library for easy communication with the Go application

## 🎯 Core Principles
- Use **SOLID principles** in all implementations
- Create comprehensive **unit tests** for all components
- code and specs are as MINIMAL as POSSIBLE

## 🔨 Building
```bash
# Run the demo (automatically builds)
make demo

# Build everything (TypeScript library + Go binary)
make build      # or just: make
                # automatically installs dependencies if needed

# Clean all build artifacts
make clean
```

## 🧪 Testing with Playwright
When testing with Playwright MCP:
1. **ALWAYS check for running instances BEFORE starting tests**
   ```bash
   pgrep -a ipfs-webapp  # Check for any running instances
   kill -9 <PID>         # Kill if found
   ```
2. **Always use an empty tmp directory** for the demo command
   ```bash
   cd /tmp/ipfs-demo-test && rm -rf * && /path/to/ipfs-webapp demo --noopen -v
   ```
3. **Track and kill processes properly**
   - **IMPORTANT**: DO NOT use `ps aux | grep ipfs-webapp` to find the PID!
     - This grep pattern will match BOTH the ipfs-webapp binary AND the Claude process
     - The Claude process command line contains the working directory path which includes "ipfs-webapp"
     - Using this pattern with kill will accidentally kill Claude too!
   - **Safe alternatives**:
     - Use `pgrep -f "ipfs-webapp demo"` to find only the actual binary
     - Use `pgrep ipfs-webapp` to find by process name only
     - Capture the PID when starting: `./ipfs-webapp demo --noopen -v & echo $!`
   - Kill and verify: `kill <PID> && sleep 1 && ps -p <PID>`
   - The `demo` command requires an empty directory or it will fail

The build process:
1. Checks and installs TypeScript dependencies if `node_modules` is missing
2. Compiles the TypeScript client library (`pkg/client/src/`) to ES modules
3. Copies the compiled library to `internal/commands/demo/` where it's embedded with the demo HTML
4. Builds the Go binary with the embedded assets
5. Users can extract the client library from the demo directory for use in their own projects

## Form: a single Go executable for the server and a TypeScript library for the browser
- the TypeScript library (`pkg/client/`) provides a clean API over the WebSocket protocol
  - located at `pkg/client/src/`
  - compiled to ES modules and bundled with the demo
  - promise-based API for all operations
  - uses protocol-based addressing with (peer, protocol) tuples instead of connection IDs
  - `start(protocol, onData)` registers listener that receives (peer, data) for all messages on that protocol
  - `send(peer, protocol, data)` sends data directly using peer+protocol addressing
  - server manages all connection lifecycle, retry, and reliability concerns transparently
  - listeners automatically removed on `stop()` or WebSocket disconnect
  - the demo (`internal/commands/demo/index.html`) demonstrates how to use the library
  - users can get the library from the demo directory after building
- implements a web server to launch a site on a random port on localhost
  - **SPA routing support**: automatically serves `index.html` for routes without file extensions
  - preserves URL path for client-side routing (e.g., `/settings`, `/adventure/world/123`)
  - real files (with extensions) served normally or return 404 if not found
- uses [ipfs-lite](https://github.com/hsanjuan/ipfs-lite) to manage the IPFS connection
- manages peers for browser connections
- commands
  - serve
    - current dir must contain these subdirectories
      - html: website to serve, must contain index.html
        - launches this in a browser by default
      - ipfs: content to make available in IPFS
      - storage: server storage (peer keys, etc.)
    - hosts WebSocket-based JSON-RPC protocol service for the site
      - use text-bsed JSON for commands
      - each side pushes commands to the other
    - flags
      - --noopen: do not open browser automatically
      - -v, --verbose: verbose output (can be specified multiple times: -v, -vv, -vvv)
        - level 1: log peer creation, connections, and messages
        - level 2+: additional debug information
      - -p, --port PORT: specify port to listen on (default: auto-select starting from 10000)
        - if port not available, automatically tries the next port (up to 100 attempts)
        - example: `./ipfs-webapp serve -p 8080`
  - demo
    - current dir must be empty
    - copies an embedded chatroom example into a directory and serves it
    - flags
      - --noopen: do not open browser automatically
      - -v, --verbose: verbose output (can be specified multiple times: -v, -vv, -vvv)
    - chatroom demo features
      - uses listPeers to discover peers subscribed to the chat topic
      - displays a list of peers on the right side
        - first entry: "Chat room" (room chat mode)
        - remaining entries: individual users
      - two communication modes indicated at the top of the view
        - room chat (default)
          - uses pubsub for group messaging
          - selected by clicking "Chat room" in the peer list
        - direct messaging
          - uses peer-to-peer protocol for private messages
          - selected by clicking a user in the peer list
      - demo listener architecture
        - single protocol listener registered at startup with `start(protocol, onData)`
        - **room chat**: `subscribe` callback always stores messages via `storeRoomMessage()`
          - stores messages even when in DM mode (never lost)
          - only displays messages if currently in room mode
          - when switching to room mode, `displayStoredRoomMessages()` shows all stored messages
        - **direct messages**: protocol listener callback receives `(peer, data)` and:
          - always stores messages via `storeDMMessage(peer, ...)` (never lost)
          - only displays via `addMessage()` if currently viewing that DM
        - `switchToDM()` simply switches UI view - no connection management needed
        - server transparently manages all stream lifecycle and reliability
  - version
    displays the current version
  - ls
    - lists files available in the embedded demo directory
    - shows files that can be copied with the cp command
    - usage: `./ipfs-webapp ls`
    - displays file names in a clean list format
    - useful for discovering what files are available before using cp
  - cp
    - copies files from the embedded demo directory to a target location
    - supports glob patterns for source selection (e.g., `*.js`, `client.*`)
    - similar to UNIX cp command but operates on embedded demo files
    - usage: `./ipfs-webapp cp SOURCE... DEST`
      - SOURCE: one or more glob patterns or file names from the embedded demo directory
      - DEST: destination directory (must exist or will be created)
    - examples
      - `./ipfs-webapp cp client.js my-project/` - copy single file
      - `./ipfs-webapp cp client.* my-project/` - copy client.js and client.d.ts
      - `./ipfs-webapp cp *.js *.html my-project/` - copy multiple patterns
    - creates destination directory if it doesn't exist
    - preserves file names (no renaming)
    - validates that at least one file matches the patterns
    - error handling
      - fails if no files match the patterns
      - fails if destination is not a directory (when copying multiple files)

## Message format
- a request has a requestID
  - starts at 0 and counts up
- a response contains the ID of its request
- message definitions are name(param...) params are strings unless otherwise indicated
  - Each message corresponds to a JSON-encodable Go struct in the server and a TypeScript struct in the browser
  - JSON properties are lowercase

## Client Request messages

### Peer(peerkey?)
- Create a new peer for this websocket connection with peerkey. If none given, use a fresh peerkey.
- Must be the first command from the browser for a websocket connection
- Cannot be sent more than once
- on the server, the new peer is associated with this WebSocket connection
#### Response: [peerid, peerkey] or error

### start(protocol)
- Start a protocol and register to receive messages
- Must be called before sending on the protocol
- Protocol validation: cannot send unless protocol is started
#### Response: null or error

### stop(protocol)
- Stop a protocol and remove message listener
#### Response: null or error

### send(peer, protocol, data: any)
- Send data to a peer on a protocol
- Uses (peer, protocol) addressing instead of connection IDs
- Server manages stream lifecycle transparently
- Validation: protocol must be started first
#### Response: null or error

### subscribe(topic: string)
#### Response: null or error

### publish(topic, data: any)
#### Response: null or error

### unsubscribe(topic)
#### Response: null or error

### listPeers(topic: string)
- Get list of peers subscribed to a topic
#### Response: array of peer IDs or error

## Server Request messages

### peerData(peer, protocol, data: any)
- Notifies client of data received from a peer on a protocol
- Sent to the protocol listener registered with `start(protocol, onData)`
- Listener receives `(peer, data)` for all messages on the protocol
#### Response: null or error

### topicData(topic, peerId, data: any)
#### Response: null or error

## Implementation Details

### Virtual Connection Model
The server manages all connection lifecycle transparently using a virtual connection model:
- Client uses (peer, protocol) tuples for addressing instead of connection IDs
- Server internally manages libp2p streams using `"peerID:protocol"` keys
- Streams are created on-demand when sending data
- Streams are automatically reused for multiple messages
- Server handles all retry logic, buffering, and reliability concerns
- Client API is simplified - no connection state management needed

### Private Address Support
libp2p by default blocks connections on private/localhost addresses. To enable local development and testing:
- `internal/peer/manager.go` implements `allowPrivateGater` type
- This ConnectionGater returns `true` for all connection gating methods
- Passed to `libp2p.New()` via `libp2p.ConnectionGater(&allowPrivateGater{})`
- Allows peers on the same machine to connect via localhost addresses

### Verbose Logging
Complete verbose logging with peer aliases:
- Manager tracks: `peerAliases` map (peerID→alias), `aliasCounter`, `verbosity` level
- Peer struct includes `alias` field
- Aliases generated as "peer-a", "peer-b", etc.
- Verbose output preceded by peer alias: `[peer-a] message`
- Logging at different verbosity levels:
  - Level 1: peer creation, connections, messages
  - Level 2: WebSocket message send/receive with request IDs
  - Level 3+: additional debug information

### Client Library Message Handling
The TypeScript client library implements robust message handling with proper ordering:

#### Sequential Message Processing
- Server-initiated messages are queued and processed strictly sequentially
- Messages processed one at a time via `processMessageQueue()` with async/await
- Ensures message ordering is preserved (critical for peerData)
- Each message fully processed before the next begins
- Error handling wraps each message processing to continue on failure
- Response messages (replies to client requests) are NOT queued - processed immediately

#### Protocol-Based Message Routing
- `start(protocol, onData)` registers a listener for a protocol
- Listener receives `(peer, data)` for all messages on that protocol
- `peerData` messages routed directly to the protocol listener
- No buffering needed - messages delivered immediately to registered listener
- `stop(protocol)` removes the listener and prevents further message delivery
- Simple, straightforward routing with no connection state to manage

#### Async Callback Support
- All callbacks (ProtocolDataCallback, TopicDataCallback) support both sync and async
- Type signature: `(peer: string, data: any) => void | Promise<void>`
- Client library awaits callback completion with try/catch error handling
- Errors logged to console but don't stop message processing

#### Protocol Lifecycle
- `start(protocol, onData)`: Register protocol listener, enable sending
- `peerData(peer, protocol, data)`: Route to protocol listener
- `send(peer, protocol, data)`: Validate protocol started, send message
- `stop(protocol)`: Remove listener, disable sending

### SPA Routing Support
The server implements automatic SPA (Single Page Application) routing fallback:
- **Route detection**: Paths without file extensions are treated as SPA routes
- **Fallback behavior**: Non-existent routes serve `index.html` while preserving the URL
- **File serving**: Real files (with extensions) are served normally
- **404 handling**: Missing files with extensions return proper 404 errors
- **Implementation**: `internal/server/server.go:spaHandler()`

Examples:
- `/` → serves `html/index.html`
- `/settings` → serves `html/index.html` (URL stays `/settings`)
- `/adventure/world/123` → serves `html/index.html` (URL stays `/adventure/world/123`)
- `/main.js` → serves `html/main.js` (actual file)
- `/nonexistent.js` → returns 404 (file with extension not found)
