package filter

import (
	"container/list"
	"sync"
)

// lruCache implements a thread-safe LRU cache
type lruCache struct {
	size      int
	evictList *list.List
	items     map[string]*list.Element
	mu        sync.RWMutex
}

// entry is stored in the cache
type entry struct {
	key   string
	value any
}

// newLRUCache creates a new LRU cache with the given size
func newLRUCache(size int) *lruCache {
	return &lruCache{
		size:      size,
		evictList: list.New(),
		items:     make(map[string]*list.Element),
	}
}

// Get retrieves a value from the cache
func (c *lruCache) Get(key string) (any, bool) {
	c.mu.RLock()
	node, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Move to front (most recently used)
	c.mu.Lock()
	c.evictList.MoveToFront(node)
	c.mu.Unlock()

	return node.Value.(*entry).value, true
}

// Put adds or updates a value in the cache
func (c *lruCache) Put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key exists
	if node, exists := c.items[key]; exists {
		c.evictList.MoveToFront(node)
		node.Value.(*entry).value = value
		return
	}

	// Add new entry
	ent := &entry{key: key, value: value}
	node := c.evictList.PushFront(ent)
	c.items[key] = node

	// Evict if necessary
	if c.evictList.Len() > c.size {
		c.removeOldest()
	}
}

// removeOldest removes the least recently used item
func (c *lruCache) removeOldest() {
	node := c.evictList.Back()
	if node != nil {
		c.evictList.Remove(node)
		kv := node.Value.(*entry)
		delete(c.items, kv.key)
	}
}

// Clear removes all items from the cache
func (c *lruCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
}

// Size returns the number of items in the cache
func (c *lruCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.evictList.Len()
}
