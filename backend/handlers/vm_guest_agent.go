package handlers

import (
	"strconv"
	"sync"
	"time"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

// Guest agent caches to avoid repeated slow API calls.
var (
	guestAgentUnavailableCache      = make(map[string]time.Time)
	guestAgentUnavailableCacheMutex sync.RWMutex
	guestAgentIPCache               = make(map[string]guestAgentCacheEntry)
	guestAgentIPCacheMutex          sync.RWMutex
)

// guestAgentCacheEntry stores cached guest agent network information.
type guestAgentCacheEntry struct {
	interfaces []proxmox.GuestAgentNetworkInterface
	expiry     time.Time
}

// containsString checks if target exists in items.
func containsString(items []string, target string) bool {
	for _, it := range items {
		if it == target {
			return true
		}
	}
	return false
}

// isGuestAgentUnavailableCached checks if a VM is cached as having no guest agent.
func isGuestAgentUnavailableCached(node string, vmid int) bool {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentUnavailableCacheMutex.RLock()
	defer guestAgentUnavailableCacheMutex.RUnlock()

	if expiry, found := guestAgentUnavailableCache[key]; found {
		if time.Now().Before(expiry) {
			return true
		}
	}
	return false
}

// cacheGuestAgentUnavailable marks a VM as having no guest agent.
func cacheGuestAgentUnavailable(node string, vmid int) {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentUnavailableCacheMutex.Lock()
	guestAgentUnavailableCache[key] = time.Now().Add(constants.GuestAgentCacheTTL)
	guestAgentUnavailableCacheMutex.Unlock()
}

// getGuestAgentIPsFromCache retrieves cached guest agent network interfaces.
func getGuestAgentIPsFromCache(node string, vmid int) ([]proxmox.GuestAgentNetworkInterface, bool) {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentIPCacheMutex.RLock()
	defer guestAgentIPCacheMutex.RUnlock()

	if entry, found := guestAgentIPCache[key]; found {
		if time.Now().Before(entry.expiry) {
			return entry.interfaces, true
		}
	}
	return nil, false
}

// cacheGuestAgentIPs stores guest agent network interfaces in cache.
func cacheGuestAgentIPs(node string, vmid int, interfaces []proxmox.GuestAgentNetworkInterface) {
	key := node + ":" + strconv.Itoa(vmid)
	guestAgentIPCacheMutex.Lock()
	guestAgentIPCache[key] = guestAgentCacheEntry{
		interfaces: interfaces,
		expiry:     time.Now().Add(constants.GuestAgentCacheTTL),
	}
	guestAgentIPCacheMutex.Unlock()
}

// InvalidateGuestAgentCache removes a specific VM's guest agent cache entries.
func InvalidateGuestAgentCache(node string, vmid int) {
	key := node + ":" + strconv.Itoa(vmid)

	guestAgentUnavailableCacheMutex.Lock()
	delete(guestAgentUnavailableCache, key)
	guestAgentUnavailableCacheMutex.Unlock()

	guestAgentIPCacheMutex.Lock()
	delete(guestAgentIPCache, key)
	guestAgentIPCacheMutex.Unlock()

	logger.Get().Debug().Str("node", node).Int("vmid", vmid).Msg("Guest agent cache invalidated for VM")
}

// CleanExpiredGuestAgentCache removes expired entries from both guest agent caches.
func CleanExpiredGuestAgentCache() {
	now := time.Now()

	unavailableCount := 0
	ipCount := 0

	guestAgentUnavailableCacheMutex.Lock()
	for key, expiry := range guestAgentUnavailableCache {
		if now.After(expiry) {
			delete(guestAgentUnavailableCache, key)
			unavailableCount++
		}
	}
	unavailableSize := len(guestAgentUnavailableCache)
	guestAgentUnavailableCacheMutex.Unlock()

	guestAgentIPCacheMutex.Lock()
	for key, entry := range guestAgentIPCache {
		if now.After(entry.expiry) {
			delete(guestAgentIPCache, key)
			ipCount++
		}
	}
	ipSize := len(guestAgentIPCache)
	guestAgentIPCacheMutex.Unlock()

	if unavailableCount > 0 || ipCount > 0 {
		logger.Get().Debug().
			Int("unavailable_expired", unavailableCount).
			Int("unavailable_remaining", unavailableSize).
			Int("ip_expired", ipCount).
			Int("ip_remaining", ipSize).
			Msg("Guest agent cache cleanup completed")
	}
}
