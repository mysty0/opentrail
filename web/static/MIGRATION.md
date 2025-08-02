# Migration from JavaScript to TypeScript/React

This document outlines the migration from the original vanilla JavaScript implementation to a modern TypeScript/React application.

## What Changed

### Architecture
- **Before**: Single `app.js` file with vanilla JavaScript class
- **After**: Modular TypeScript/React application with proper separation of concerns

### File Structure
```
web/static/
├── src/                    # Source code (NEW)
│   ├── components/         # React components
│   ├── hooks/             # Custom hooks
│   ├── services/          # API services
│   ├── types/             # TypeScript types
│   ├── utils/             # Utility functions
│   ├── App.tsx            # Main app component
│   ├── main.tsx           # Entry point
│   └── index.css          # Styles
├── package.json           # Dependencies (NEW)
├── tsconfig.json          # TypeScript config (NEW)
├── vite.config.ts         # Build config (NEW)
├── build.sh / build.cmd   # Build scripts (NEW)
├── index.html             # Updated template
├── style.css              # Generated from build
└── app.js                 # Generated from build
```

### Key Improvements

1. **Type Safety**: Full TypeScript implementation with proper type definitions
2. **Component Architecture**: Modular React components for better maintainability
3. **Custom Hooks**: Reusable logic for WebSocket connections and localStorage
4. **Modern Build System**: Vite for fast development and optimized production builds
5. **Better State Management**: React hooks for state management
6. **Improved Performance**: React's virtual DOM and optimized re-renders

### Component Breakdown

- **App.tsx**: Main application logic and state management
- **ConnectionStatus.tsx**: WebSocket connection status indicator
- **FilterPanel.tsx**: RFC5424 log filtering interface
- **DisplayPanel.tsx**: Display options and field visibility controls
- **LogContainer.tsx**: Log display container with auto-scroll and pagination
- **LogEntry.tsx**: Individual log entry rendering with structured data

### Hooks

- **useWebSocket**: Manages WebSocket connection, reconnection, and message handling
- **useLocalStorage**: Persistent storage for display preferences

### Build Process

1. **Development**: `npm run dev` - Vite dev server with hot reload
2. **Production**: `npm run build` - TypeScript compilation + Vite bundling
3. **Deploy**: Build scripts copy generated files to static root for Go embedding

## Compatibility

The new React application maintains 100% API compatibility with the existing Go backend:
- Same WebSocket endpoint (`/api/logs/stream`)
- Same REST API endpoint (`/api/logs`)
- Same message format and data structures
- Same embedded file serving mechanism

## Migration Steps

1. **Install Node.js** (if not already installed)
2. **Install dependencies**: `npm install`
3. **Build the application**: `npm run build` or use build scripts
4. **Test**: The Go application will serve the new React frontend automatically

## Development Workflow

1. **Start Go backend**: `go run cmd/opentrail/main.go`
2. **Start React dev server**: `npm run dev` (in web/static/)
3. **Develop**: Edit TypeScript/React files with hot reload
4. **Build for production**: `npm run build`
5. **Deploy**: Built files are automatically embedded in Go binary

## Benefits

- **Better Developer Experience**: TypeScript, hot reload, modern tooling
- **Improved Maintainability**: Modular components, clear separation of concerns
- **Enhanced Performance**: React optimizations, efficient re-renders
- **Future-Proof**: Modern stack that's easier to extend and maintain
- **Type Safety**: Catch errors at compile time, better IDE support