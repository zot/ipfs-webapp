# p2p-webapp
- A Go application to host peer-to-peer applications
- Proxies opinionated IPFS and libp2p operations for managed peers
- Provides a TypeScript library for easy communication with the Go application

## 🎯 Core Principles
- Use **SOLID principles** in all implementations
- Create comprehensive **unit tests** for all components
- code and specs are as MINIMAL as POSSIBLE

## 🔨 Building
```bash
# Run the demo (automatically builds and extracts bundled demo)
make demo

# Build everything (TypeScript library + Go binary with bundled demo)
make build      # or just: make
                # automatically installs dependencies if needed

# Clean all build artifacts
make clean
```

## 🧪 Testing with Playwright
When testing with Playwright MCP:
1. **ALWAYS check for running instances BEFORE starting tests**
   ```bash
   pgrep -a p2p-webapp  # Check for any running instances
   kill -9 <PID>         # Kill if found
   ```
2. **Always use an empty tmp directory** for the extract command
   ```bash
   cd /tmp/ipfs-extract-test && rm -rf * && /path/to/p2p-webapp extract --noopen -v
   ```
3. **Track and kill processes properly**
   - **IMPORTANT**: DO NOT use `ps aux | grep p2p-webapp` to find the PID!
     - This grep pattern will match BOTH the p2p-webapp binary AND the Claude process
     - The Claude process command line contains the working directory path which includes "p2p-webapp"
     - Using this pattern with kill will accidentally kill Claude too!
   - **Safe alternatives**:
     - Use `pgrep -f "p2p-webapp extract"` to find only the actual binary
     - Use `pgrep p2p-webapp` to find by process name only
     - Capture the PID when starting: `./p2p-webapp extract --noopen -v & echo $!`
   - Kill and verify: `kill <PID> && sleep 1 && ps -p <PID>`
   - The `extract` command requires an empty directory or it will fail

The build process:
1. Checks and installs TypeScript dependencies if `node_modules` is missing
2. Compiles the TypeScript client library (`pkg/client/src/`) to ES modules
3. Copies the compiled library to `internal/commands/demo/` for bundling
4. Builds a temporary Go binary
5. Bundles the demo site (from `internal/commands/demo/`) into the final binary using ZIP append
6. The final binary always ships with the demo bundled and ready to extract
7. Users can extract the demo with the `extract` command and get the client library files
8. The `ls` and `cp` commands operate directly on the bundled content (no go:embed needed)

## Form: a single Go executable for the server and a TypeScript library for the browser
- the TypeScript library (`pkg/client/`) provides a clean API over the WebSocket protocol
  - located at `pkg/client/src/`
  - compiled to ES modules and bundled with the demo
  - promise-based API for all operations
  - `connect(peerKey?)` connects to server and initializes peer in one call
    - merges WebSocket connection and peer initialization for simplicity
    - returns `[peerID, peerKey]` tuple
    - internally performs WebSocket connect followed by peer protocol message
  - uses protocol-based addressing with (peer, protocol) tuples instead of connection IDs
  - `start(protocol, onData)` registers listener that receives (peer, data) for all messages on that protocol
  - `send(peer, protocol, data, onAck?)` sends data directly using peer+protocol addressing
    - optional `onAck` callback invoked when delivery is confirmed
    - client library manages ack numbers internally (upward counting)
    - consumers don't see ack numbers, only get delivery confirmation via callback
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
    - operates in one of two modes
      - **default mode** (no --dir flag): serves directly from bundled site without filesystem extraction
        - requires binary to be bundled (use `bundle` command)
        - creates `.p2p-webapp-storage` directory in current directory for peer keys
        - efficient - no extraction needed, serves directly from ZIP
      - **directory mode** (with --dir flag): serves from filesystem directory
        - directory must contain these subdirectories
          - html: website to serve, must contain index.html
            - launches this in a browser by default
          - ipfs: content to make available in IPFS (optional)
          - storage: server storage (peer keys, etc.)
        - use after extracting with `extract` command
        - example: `./p2p-webapp serve --dir .`
    - hosts WebSocket-based JSON-RPC protocol service for the site
      - use text-based JSON for commands
      - each side pushes commands to the other
    - flags
      - --dir DIR: directory to serve from (if not specified, serves from bundled site)
      - --noopen: do not open browser automatically
      - -v, --verbose: verbose output (can be specified multiple times: -v, -vv, -vvv)
        - level 1: log peer creation, connections, and messages
        - level 2+: additional debug information
      - -p, --port PORT: specify port to listen on (default: auto-select starting from 10000)
        - if port not available, automatically tries the next port (up to 100 attempts)
        - example: `./p2p-webapp serve -p 8080`
  - extract
    - extracts the bundled site from the binary to the current directory
    - current dir must be empty
    - extracts html/, ipfs/, and storage/ subdirectories
    - does NOT start the server - run `p2p-webapp serve --dir .` afterwards to serve
    - the default bundled site is a chatroom application that demonstrates key features
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
  - bundle
    - creates a standalone binary with a site bundled into it
    - works with both bundled and unbundled source binaries (replaces existing bundle if present)
    - no compilation tools needed - works out of the box
    - usage: `./p2p-webapp bundle [site-directory] -o [output-binary]`
    - site directory must contain
      - html/: website files (must contain index.html)
      - ipfs/: IPFS content (optional)
      - storage/: storage directory (optional, created if missing)
    - example: `./p2p-webapp bundle my-site -o my-app`
    - the output binary can be distributed and will extract/serve the bundled site
    - uses ZIP append technique - appends ZIP archive + footer to binary
      - footer contains: magic marker (8 bytes) + offset (8 bytes) + size (8 bytes)
      - Go binaries can have trailing data without affecting execution
      - on startup, checks for bundled content and extracts if found
  - version
    displays the current version
  - ls
    - lists files available in the bundled site
    - shows files that can be copied with the cp command
    - usage: `./p2p-webapp ls`
    - displays file names in a clean list format
    - useful for discovering what files are available before using cp
    - reads directly from the bundled content (no extraction needed)
  - cp
    - copies files from the bundled site to a target location
    - supports glob patterns for source selection (e.g., `*.js`, `client.*`)
    - similar to UNIX cp command but operates on bundled site files
    - usage: `./p2p-webapp cp SOURCE... DEST`
      - SOURCE: one or more glob patterns or file names from the bundled site
      - DEST: destination directory (must exist or will be created)
    - examples
      - `./p2p-webapp cp client.js my-project/` - copy single file
      - `./p2p-webapp cp client.* my-project/` - copy client.js and client.d.ts
      - `./p2p-webapp cp *.js *.html my-project/` - copy multiple patterns
    - creates destination directory if it doesn't exist
    - preserves file names (no renaming)
    - validates that at least one file matches the patterns
    - reads directly from the bundled content (no extraction needed)
    - error handling
      - fails if no files match the patterns
      - fails if destination is not a directory (when copying multiple files)
      - fails if binary is not bundled
  - ps
    - lists process IDs for all running p2p-webapp instances
    - usage: `./p2p-webapp ps`
    - flags
      - -v, --verbose: also shows command line arguments for each instance
    - displays PID and optionally command line args in a clean table format
    - automatically cleans stale entries from the tracking file
  - kill
    - terminates a specific running instance by PID
    - usage: `./p2p-webapp kill PID`
    - validates that PID is actually an p2p-webapp instance
    - removes the instance from the tracking file
    - returns error if PID is not found or not an p2p-webapp process
  - killall
    - terminates all running p2p-webapp instances
    - usage: `./p2p-webapp killall`
    - kills all instances tracked in the PID file
    - automatically validates and cleans up stale entries
    - reports how many instances were killed

## Process Tracking
p2p-webapp maintains a JSON list of running instance PIDs for process management:
- **PID file location**: `/tmp/.p2p-webapp` on systems with `/tmp`
- **File locking**: Uses file locking to prevent race conditions during read/write operations
- **Process verification**: When reading the file, automatically verifies PIDs are actual running p2p-webapp instances
- **Auto-correction**: Removes stale entries for processes that are no longer running
- **Library**: Uses `github.com/shirou/gopsutil` for cross-platform process management
- **Registration**: Each `serve` and `extract` command automatically registers its PID on startup
- **Cleanup**: PIDs are removed when the process exits normally or via `kill`/`killall` commands

The tracking system ensures:
1. File is locked during read/write operations to prevent corruption
2. Only verified p2p-webapp processes are listed
3. Stale entries are automatically cleaned up
4. Safe concurrent access from multiple instances

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
- If a peer with the resulting peer ID is already registered (e.g., duplicate browser tab with same peer key), returns an error
  - This prevents multiple WebSocket connections from using the same peer identity
  - Common cause: user opens the same app in multiple browser tabs with the same stored peer key
#### Response: [peerid, peerkey] or error

### start(protocol)
- Start a protocol and register to receive messages
- Must be called before sending on the protocol
- Protocol validation: cannot send unless protocol is started
#### Response: null or error

### stop(protocol)
- Stop a protocol and remove message listener
#### Response: null or error

### send(peer, protocol, data: any, ack: number = -1)
- Send data to a peer on a protocol
- Uses (peer, protocol) addressing instead of connection IDs
- Server manages stream lifecycle transparently
- Validation: protocol must be started first
- `ack` parameter: if non-negative (>= 0), server will send an `ack` message back to this client when delivery to the peer is confirmed
  - If `ack` is -1 or not provided, no acknowledgment is sent
  - Ack numbers must be non-negative integers (0, 1, 2, ...)
  - This is an internal protocol detail - client library manages ack numbers automatically
#### Response: null or error

### subscribe(topic: string)
- Subscribe to a topic to receive published messages
- Automatically monitors the topic for peer join/leave events
- Server will send `peerChange` notifications when peers join or leave
#### Response: null or error

### publish(topic, data: any)
#### Response: null or error

### unsubscribe(topic)
- Unsubscribe from a topic
- Stops monitoring peer join/leave events for this topic
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

### peerChange(topic: string, peerId: string, joined: boolean)
- Notifies client that a peer joined or left a subscribed topic
- `joined` is true when peer joins, false when peer leaves
- Automatically sent for all subscribed topics (no separate monitoring needed)
#### Response: null or error

### ack(ack: number)
- Notifies client that a message with the given ack number was successfully delivered to the peer
- Only sent in response to `send` requests with a non-negative `ack` parameter (>= 0)
- If a `send` request has `ack` = -1 or no ack parameter, no `ack` message is sent back
- Client library manages ack numbers internally and notifies consumers via callback
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

#### Message Acknowledgment
The client library provides optional delivery confirmation for sent messages:
- **Client API**: `send(peer, protocol, data, onAck?)`
  - `onAck` is an optional callback: `() => void | Promise<void>`
  - Called when the message is successfully delivered to the peer
  - If `onAck` is provided, client library automatically:
    - Assigns an internal ack number (starting from 0, incrementing)
    - Includes ack number in the send request to server
    - Stores the callback in an internal map keyed by ack number
  - If `onAck` is not provided, no ack number is sent and no acknowledgment occurs
- **Server behavior**: Only if `ack` parameter was in the send request, sends `ack(ack: number)` message when delivery confirmed
- **Client handling**: On receiving `ack` message:
  - Looks up callback in internal map by ack number
  - Invokes callback (with async/await support)
  - Removes callback from map
- **Consumer perspective**: Simple callback interface, no ack number management needed

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
