# Merged ModelServer Controller

## Overview

The ModelServer controller has been merged with the Pod controller functionality to share a single workqueue and handle both resource types efficiently.

## Key Features

### Shared Workqueue
- Single `workqueue.TypedRateLimitingInterface[QueueItem]` handles both ModelServer and Pod events
- `QueueItem` struct contains `ResourceType` and `Key` fields to distinguish resource types
- Eliminates need for separate controllers and reduces resource overhead

### Resource Types
```go
type ResourceType string

const (
    ResourceTypeModelServer ResourceType = "ModelServer"
    ResourceTypePod         ResourceType = "Pod"
)

type QueueItem struct {
    ResourceType ResourceType
    Key          string
}
```

### Event Handling
- **ModelServer events**: Create, Update, Delete operations on ModelServer resources
- **Pod events**: Create, Update, Delete operations on Pod resources
- Both resource types use the same workqueue with different `ResourceType` identifiers

### Processing Logic
1. Items are dequeued and processed based on their `ResourceType`
2. `syncModelServerHandler()` processes ModelServer resources
3. `syncPodHandler()` processes Pod resources
4. Initial sync signal is handled using an empty `QueueItem{}`

## Architecture Benefits

1. **Resource Efficiency**: Single workqueue reduces memory footprint
2. **Simplified Logic**: One controller to manage instead of two
3. **Shared State**: Both resource types can share the same data store
4. **Consistent Error Handling**: Unified retry and rate limiting logic

## Usage

```go
// Create the merged controller
controller := NewModelServerController(
    kthenaInformerFactory,
    kubeInformerFactory,
    store,
)

// Run with multiple workers
err := controller.Run(workers, stopCh)
```

## Migration Notes

- The original PodController functionality has been merged into ModelServerController
- All Pod-related event handling is now done through the same workqueue
- The controller name remains ModelServerController for backward compatibility
- Event registration handles both ModelServer and Pod informers
