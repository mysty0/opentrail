# OpenTrail Frontend

This directory contains the TypeScript/React frontend for OpenTrail, a RFC5424 log viewer.

## Development Setup

1. **Install dependencies:**
   ```bash
   npm install
   ```

2. **Start development server:**
   ```bash
   npm run dev
   ```
   This will start Vite dev server with hot reload at `http://localhost:5173`

3. **Build for production:**
   ```bash
   npm run build
   ```
   Or use the provided build scripts:
   - Linux/macOS: `./build.sh`
   - Windows: `build.cmd`

## Project Structure

```
src/
├── components/          # React components
│   ├── ConnectionStatus.tsx
│   ├── FilterPanel.tsx
│   ├── DisplayPanel.tsx
│   ├── LogEntry.tsx
│   └── LogContainer.tsx
├── hooks/              # Custom React hooks
│   ├── useWebSocket.ts
│   └── useLocalStorage.ts
├── services/           # API services
│   └── api.ts
├── types/              # TypeScript type definitions
│   └── index.ts
├── utils/              # Utility functions
│   ├── constants.ts
│   └── formatters.ts
├── App.tsx             # Main application component
├── main.tsx            # Application entry point
└── index.css           # Global styles
```

## Features

- **Real-time log streaming** via WebSocket
- **RFC5424 field filtering** (facility, severity, hostname, etc.)
- **Display customization** to show/hide specific header fields
- **Compact mode** for denser log display
- **Structured data expansion**
- **Auto-scroll control** with smart scroll detection
- **Load-more functionality** when scrolling to top
- **Persistent display preferences** using localStorage
- **Responsive design** for mobile and desktop

## Build Process

The build process uses Vite to:
1. Compile TypeScript to JavaScript
2. Bundle React components
3. Process CSS
4. Generate optimized production files

The built files (`index.html`, `style.css`, `app.js`) are copied to the root of the static directory where they can be served by the Go backend using the embedded filesystem.

## API Integration

The frontend communicates with the Go backend via:
- **REST API** at `/api/logs` for fetching historical logs
- **WebSocket** at `/api/logs/stream` for real-time log streaming

## Development vs Production

- **Development**: Uses Vite dev server with proxy to Go backend
- **Production**: Static files are embedded in Go binary and served directly