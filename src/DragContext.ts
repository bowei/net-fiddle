import { createContext } from 'react';

// Holds the ID of the container node currently being hovered during any drag.
// Used by ContainerNode to show drop-target highlighting without touching node state.
export const DragContext = createContext<string | null>(null);
