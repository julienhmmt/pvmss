# Graph Report - /Users/jh/git/gh/pvmss  (2026-05-04)

## Corpus Check

- Large corpus: 537 files · ~270,864 words. Semantic extraction will be expensive (many Claude tokens). Consider running on a subfolder, or use --no-semantic to run AST-only.

## Summary

- 2578 nodes · 4406 edges · 75 communities detected
- Extraction: 73% EXTRACTED · 27% INFERRED · 0% AMBIGUOUS · INFERRED: 1203 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)

- [[_COMMUNITY_Admin API Handlers|Admin API Handlers]]
- [[_COMMUNITY_Database Layer|Database Layer]]
- [[_COMMUNITY_noVNC RFB Protocol|noVNC RFB Protocol]]
- [[_COMMUNITY_Frontend Admin API|Frontend Admin API]]
- [[_COMMUNITY_API Router & Middleware|API Router & Middleware]]
- [[_COMMUNITY_State Manager|State Manager]]
- [[_COMMUNITY_Logging Layer|Logging Layer]]
- [[_COMMUNITY_HTTP Handlers|HTTP Handlers]]
- [[_COMMUNITY_Admin Settings API|Admin Settings API]]
- [[_COMMUNITY_Auth & Guard|Auth & Guard]]
- [[_COMMUNITY_Cloud-Init & Errors|Cloud-Init & Errors]]
- [[_COMMUNITY_Rate Limit & Proxmox Health|Rate Limit & Proxmox Health]]
- [[_COMMUNITY_noVNC Crypto|noVNC Crypto]]
- [[_COMMUNITY_noVNC Compression|noVNC Compression]]
- [[_COMMUNITY_VNC & Proxmox Client|VNC & Proxmox Client]]
- [[_COMMUNITY_Utils & Cache|Utils & Cache]]
- [[_COMMUNITY_Sidebar UI|Sidebar UI]]
- [[_COMMUNITY_noVNC Display|noVNC Display]]
- [[_COMMUNITY_Setup Flow|Setup Flow]]
- [[_COMMUNITY_noVNC WebSocket|noVNC WebSocket]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 35|Community 35]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 37|Community 37]]
- [[_COMMUNITY_Community 39|Community 39]]
- [[_COMMUNITY_Community 40|Community 40]]
- [[_COMMUNITY_Community 41|Community 41]]
- [[_COMMUNITY_Community 42|Community 42]]
- [[_COMMUNITY_Community 44|Community 44]]
- [[_COMMUNITY_Community 45|Community 45]]
- [[_COMMUNITY_Community 46|Community 46]]
- [[_COMMUNITY_Community 47|Community 47]]
- [[_COMMUNITY_Community 50|Community 50]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 54|Community 54]]
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 58|Community 58]]
- [[_COMMUNITY_Community 59|Community 59]]
- [[_COMMUNITY_Community 60|Community 60]]
- [[_COMMUNITY_Community 61|Community 61]]
- [[_COMMUNITY_Community 64|Community 64]]
- [[_COMMUNITY_Community 65|Community 65]]
- [[_COMMUNITY_Community 69|Community 69]]
- [[_COMMUNITY_Community 70|Community 70]]
- [[_COMMUNITY_Community 71|Community 71]]
- [[_COMMUNITY_Community 72|Community 72]]
- [[_COMMUNITY_Community 74|Community 74]]
- [[_COMMUNITY_Community 75|Community 75]]
- [[_COMMUNITY_Community 76|Community 76]]
- [[_COMMUNITY_Community 77|Community 77]]
- [[_COMMUNITY_Community 81|Community 81]]
- [[_COMMUNITY_Community 82|Community 82]]
- [[_COMMUNITY_Community 86|Community 86]]
- [[_COMMUNITY_Community 87|Community 87]]
- [[_COMMUNITY_Community 89|Community 89]]
- [[_COMMUNITY_Community 90|Community 90]]
- [[_COMMUNITY_Community 95|Community 95]]
- [[_COMMUNITY_Community 98|Community 98]]
- [[_COMMUNITY_Community 101|Community 101]]
- [[_COMMUNITY_Community 102|Community 102]]
- [[_COMMUNITY_Community 103|Community 103]]
- [[_COMMUNITY_Community 144|Community 144]]

## God Nodes (most connected - your core abstractions)

1. `RFB` - 102 edges
2. `errInternal()` - 63 edges
3. `writeJSON()` - 60 edges
4. `appState` - 59 edges
5. `errBadRequest()` - 51 edges
6. `mockStateManager` - 46 edges
7. `mockStateManager` - 46 edges
8. `transformKeysToCamelCase()` - 41 edges
9. `usernameFromCtx()` - 41 edges
10. `Contains()` - 38 edges

## Surprising Connections (you probably didn't know these)

- `main()` --calls--> `SetProductionMode()`  [INFERRED]
  backend/main.go → backend/security/middleware/headers.go
- `TestContains()` --calls--> `Contains()`  [INFERRED]
  backend/utils/generics_test.go → backend/utils/generics.go
- `RegisterRoutes()` --calls--> `MakeVNCHandler()`  [INFERRED]
  backend/api/v1/router.go → backend/api/v1/vnc.go
- `InitHandlers()` --calls--> `MakeSettingsHandler()`  [INFERRED]
  backend/handlers/handlers.go → backend/handlers/settings.go
- `inflate()` --calls--> `inflate_fast()`  [INFERRED]
  frontend/static/noVNC-1.6.0/vendor/pako/lib/zlib/inflate.js → frontend/static/noVNC-1.6.0/vendor/pako/lib/zlib/inffast.js

## Communities

### Community 0 - "Admin API Handlers"

Cohesion: 0.03
Nodes (144): TestAudit_AuditEntry_Fields(), TestAudit_FilterByTable(), TestAudit_LimitAndOffset(), TestAudit_OldAndNewValueStored(), TestAudit_OrderedMostRecentFirst(), TestAudit_WritesCreateEntries(), HasRole(), CreateTicketResty() (+136 more)

### Community 1 - "Database Layer"

Cohesion: 0.03
Nodes (92): AppSettings, appendAudit(), buildAuditQuery(), nullableString(), scanAuditEntries(), AuditEntry, scanCloudInitRow(), scanCloudInitRows() (+84 more)

### Community 2 - "noVNC RFB Protocol"

Cohesion: 0.03
Nodes (28): _buildExtendedClipboardFlags(), clientCutText(), clientEncodings(), clientFence(), enableContinuousUpdates(), extendedClipboardCaps(), extendedClipboardNotify(), extendedClipboardProvide() (+20 more)

### Community 3 - "Frontend Admin API"

Cohesion: 0.03
Nodes (67): getAppInfo(), getAuditLog(), createCloudInit(), getCloudInits(), updateCloudInit(), getISOs(), toggleISO(), getLimits() (+59 more)

### Community 4 - "API Router & Middleware"

Cohesion: 0.03
Nodes (64): initializeApp(), main(), NewSessionManager(), IsDevelopment(), IsProduction(), TestEnvDetectionConsistency(), TestIsDevelopment(), TestIsProduction() (+56 more)

### Community 5 - "State Manager"

Cohesion: 0.04
Nodes (16): ProxmoxFailure(), appState, cloneNodeDetails(), newAppState(), translateProxmoxMessage(), copyCloudInitTemplates(), copyNodeLimitsDeep(), copyStrings() (+8 more)

### Community 6 - "Logging Layer"

Cohesion: 0.06
Nodes (63): ContextKey, AdminEvent(), APIEvent(), APIFailure(), AuthEvent(), AuthFailure(), ConsoleEvent(), DatabaseEvent() (+55 more)

### Community 7 - "HTTP Handlers"

Cohesion: 0.04
Nodes (56): ErrorHelper, ErrorResponse, LocalizeErrorWithFallback(), MakeErrorHelper(), RenderErrorPageWithI18n(), RespondWithCustomError(), RespondWithError(), RespondWithErrorAndLog() (+48 more)

### Community 8 - "Admin Settings API"

Cohesion: 0.05
Nodes (63): OpenMemory(), openTestDB(), TestClose_Idempotent(), TestOpen_InMemory(), makeVMProfileConfig(), openTestDB(), TestAppState_CloudInitTemplate_CRUD(), TestAppState_ConcurrentReadWrite() (+55 more)

### Community 9 - "Auth & Guard"

Cohesion: 0.04
Nodes (19): IsAdmin(), IsAuthenticated(), newTestRequest(), TestIsAdmin_IsAdmin(), TestIsAdmin_NotAdmin(), TestIsAuthenticated_Authenticated(), TestIsAuthenticated_NoSessionManager(), TestIsAuthenticated_NoStateManager() (+11 more)

### Community 10 - "Cloud-Init & Errors"

Cohesion: 0.04
Nodes (48): ValidationError, IsValidYAML(), ParseCloudInitYAML(), SanitizeYAML(), TestIsValidYAML(), TestParseCloudInitYAML(), TestSanitizeYAML(), TestValidateCloudInitYAML() (+40 more)

### Community 11 - "Rate Limit & Proxmox Health"

Cohesion: 0.03
Nodes (14): bucket, contextKey, Limiter, TestClientIP(), TestProxmoxStatusMiddlewareInjectsContext(), TestRateLimitMiddlewareRejectsAfterQuota(), TestRateLimitRefillAllowsAfterWait(), mockStateManager (+6 more)

### Community 12 - "noVNC Crypto"

Cohesion: 0.05
Nodes (12): AESEAXCipher, AESECBCipher, bigIntToU8Array(), modPow(), u8ArrayToBigInt(), LegacyCrypto, DES, DESCBCCipher (+4 more)

### Community 13 - "noVNC Compression"

Cohesion: 0.06
Nodes (40): Deflator, Inflate, adler32(), crc32(), deflate(), deflate_fast(), deflate_huff(), deflate_rle() (+32 more)

### Community 14 - "VNC & Proxmox Client"

Cohesion: 0.06
Nodes (16): H264Context, H264Decoder, H264Parser, getEnvConfig(), MakeRestyClientCookieAuthFromEnv(), MakeRestyClientCookieAuthFromEnvConfig(), normalizeBaseURL(), MakeRestyClient() (+8 more)

### Community 15 - "Utils & Cache"

Cohesion: 0.06
Nodes (42): Cache, cacheItem, CacheWith(), Coalesce(), Deref(), DerefOr(), Filter(), Find() (+34 more)

### Community 16 - "Sidebar UI"

Cohesion: 0.06
Nodes (2): SidebarState, useSidebar()

### Community 17 - "noVNC Display"

Cohesion: 0.13
Nodes (2): Display, toSigned32bit()

### Community 18 - "Setup Flow"

Cohesion: 0.09
Nodes (19): MakeSetupHandler(), RequireSetupIncompleteForTest(), newStubState(), TestRequireSetupIncomplete_BlocksWhenComplete(), TestRequireSetupIncomplete_PassesWhenNotComplete(), TestSetupHandler_Complete_InvalidBody(), TestSetupHandler_Complete_PersistsConfigAndMarksBootstrap(), TestSetupHandler_Status_Complete() (+11 more)

### Community 19 - "noVNC WebSocket"

Cohesion: 0.09
Nodes (1): Websock

### Community 20 - "Community 20"

Cohesion: 0.07
Nodes (5): AssertLen(), RequestBuilder, TableTest, TestRequest, TestResponse

### Community 21 - "Community 21"

Cohesion: 0.16
Nodes (26): bi_flush(), bi_reverse(), bi_windup(), build_bl_tree(), build_tree(), compress_block(), copy_block(), d_code() (+18 more)

### Community 22 - "Community 22"

Cohesion: 0.11
Nodes (19): buildCloudInitSection(), buildListSection(), buildNodeLimitsSection(), buildSFTPSection(), buildVMLimitsSection(), buildVMProfilesSection(), safeStringSlice(), vmProfileByID() (+11 more)

### Community 23 - "Community 23"

Cohesion: 0.07
Nodes (27): AdminAppInfoResponse, AdminCloudInitListResponse, AdminCloudInitResponse, AdminClusterInfoResponse, AdminISOResponse, AdminLimitsResponse, AdminNodeResponse, AdminPoolResponse (+19 more)

### Community 24 - "Community 24"

Cohesion: 0.09
Nodes (3): isTestAdminAuthenticated(), setTestAdminSessionCookie(), TestHandlerCollection

### Community 25 - "Community 25"

Cohesion: 0.21
Nodes (1): GestureHandler

### Community 26 - "Community 26"

Cohesion: 0.11
Nodes (12): AppSettings, CloudInitTemplate, LimitsConfig, mapCloudInitTemplatesFromDB(), mapDBToStateSettings(), mapSFTPFromDB(), mapVMProfilesFromDB(), NodeResourceLimits (+4 more)

### Community 27 - "Community 27"

Cohesion: 0.16
Nodes (12): MakeLRUCache(), TestLRUCache_BasicOperations(), TestLRUCache_CleanExpired(), TestLRUCache_Clear(), TestLRUCache_Concurrency(), TestLRUCache_Delete(), TestLRUCache_Eviction(), TestLRUCache_LRUOrdering() (+4 more)

### Community 28 - "Community 28"

Cohesion: 0.16
Nodes (13): cachedVMBRs, buildVMBRDescription(), collectAllVMBRs(), collectVMBRsFromCache(), getCachedVMBRsForNode(), getVMBRInterface(), getVMBRsFromNode(), vmbrsFromSnapshot() (+5 more)

### Community 29 - "Community 29"

Cohesion: 0.22
Nodes (1): Cursor

### Community 31 - "Community 31"

Cohesion: 0.11
Nodes (5): TestNodeLimits_SetCreatesAuditEntry(), TestSFTPConfig_SecondSetAuditActionIsUpdate(), TestSFTPConfig_SetCreatesAuditEntry(), TestVMLimits_SecondSetAuditActionIsUpdate(), TestVMLimits_SetCreatesAuditEntry()

### Community 32 - "Community 32"

Cohesion: 0.15
Nodes (5): applyDefaults(), applyProfile(), clamp(), findBestStorage(), selectProfile()

### Community 33 - "Community 33"

Cohesion: 0.17
Nodes (2): RA2Cipher, RSAAESAuthenticationState

### Community 35 - "Community 35"

Cohesion: 0.23
Nodes (13): fetchRawTagStyle(), fetchTagStyleBuilder(), GetTagColorsResty(), normalizeHex(), parseTagStyle(), SetTagColorResty(), splitTagStyle(), TestParseTagStyle() (+5 more)

### Community 36 - "Community 36"

Cohesion: 0.28
Nodes (1): TightDecoder

### Community 37 - "Community 37"

Cohesion: 0.35
Nodes (12): add(), cmn(), ff(), gg(), hh(), ii(), M(), MD5() (+4 more)

### Community 39 - "Community 39"

Cohesion: 0.27
Nodes (10): newTemplate(), TestCloudInit_CreateAndList(), TestCloudInit_CreateCreatesAuditEntry(), TestCloudInit_CreateDuplicate_ReturnsError(), TestCloudInit_Delete(), TestCloudInit_DeleteCreatesAuditEntry(), TestCloudInit_GetByID(), TestCloudInit_Update() (+2 more)

### Community 40 - "Community 40"

Cohesion: 0.15
Nodes (2): TestEnabledNodes_SetCreatesAuditEntry(), TestTags_SetCreatesAuditEntry()

### Community 41 - "Community 41"

Cohesion: 0.27
Nodes (10): newProfile(), TestVMProfile_CreateAndList(), TestVMProfile_CreateCreatesAuditEntry(), TestVMProfile_CreateDuplicate_ReturnsError(), TestVMProfile_Delete(), TestVMProfile_DeleteCreatesAuditEntry(), TestVMProfile_GetByID(), TestVMProfile_Update() (+2 more)

### Community 42 - "Community 42"

Cohesion: 0.23
Nodes (11): boolToForm(), isConflictError(), normalizeUserID(), poolPath(), EnsurePoolACLResty(), EnsurePoolResty(), EnsureRoleResty(), EnsureUserResty() (+3 more)

### Community 44 - "Community 44"

Cohesion: 0.2
Nodes (8): createSFTPClient(), DeleteSnippetFileSFTP(), ListSnippetFilesResty(), UploadSnippetFileSFTP(), CloudInitConfig, CloudInitParams, CloudInitSFTPConfig, SnippetFile

### Community 45 - "Community 45"

Cohesion: 0.26
Nodes (9): GenerateRandomMACAddress(), NormalizeMACAddress(), BenchmarkGenerateRandomMACAddress(), BenchmarkNormalizeMACAddress(), BenchmarkValidateMACAddress(), TestGenerateRandomMACAddress(), TestNormalizeMACAddress(), TestValidateMACAddress() (+1 more)

### Community 46 - "Community 46"

Cohesion: 0.17
Nodes (11): AuthResponse, contextKey, ErrorResponse, LoginRequest, MeResponse, PaginationMetadata, VMActionRequest, VMActionResponse (+3 more)

### Community 47 - "Community 47"

Cohesion: 0.3
Nodes (10): MBToGB(), buildNodeUsageFromSnapshot(), CalculateNodeResourceUsage(), getCachedNodeUsage(), returnLocalizedError(), splitTags(), storeNodeUsageCache(), ValidateVMResourcesAgainstNodeLimits() (+2 more)

### Community 50 - "Community 50"

Cohesion: 0.35
Nodes (9): assertEqualDuration(), assertEqualInt(), assertEqualString(), TestContextTimeouts(), TestDefaultValues(), TestHTTPConfiguration(), TestProxmoxConfiguration(), TestSecurityConfiguration() (+1 more)

### Community 51 - "Community 51"

Cohesion: 0.42
Nodes (1): ZRLEDecoder

### Community 54 - "Community 54"

Cohesion: 0.33
Nodes (5): handleColorChange(), handleColorReset(), handleCreate(), handleDelete(), load()

### Community 55 - "Community 55"

Cohesion: 0.36
Nodes (1): Cache[K, V]

### Community 56 - "Community 56"

Cohesion: 0.39
Nodes (2): sendJSONResponse(), HealthHandler

### Community 58 - "Community 58"

Cohesion: 0.36
Nodes (6): MakeTestApp(), registerTestRoutes(), TestApp, TestCSRFProtection(), TestProtectedRoutes(), TestPublicRoutes()

### Community 59 - "Community 59"

Cohesion: 0.32
Nodes (7): NetworkUpdateRequest, buildNetLine(), pollVMStatus(), waitForVMStarted(), waitForVMStopped(), VMHardwareUpdateRequest, VMHardwareUpdateResponse

### Community 60 - "Community 60"

Cohesion: 0.43
Nodes (1): HextileDecoder

### Community 61 - "Community 61"

Cohesion: 0.33
Nodes (1): EventTargetMixin

### Community 64 - "Community 64"

Cohesion: 0.47
Nodes (4): GetAppLocation(), TestGetAppLocation(), TestGetAppLocationSingleton(), TestGetAppLocationWithTime()

### Community 65 - "Community 65"

Cohesion: 0.5
Nodes (1): JPEGDecoder

### Community 69 - "Community 69"

Cohesion: 0.4
Nodes (1): Optional[T]

### Community 70 - "Community 70"

Cohesion: 0.4
Nodes (1): Result[T]

### Community 71 - "Community 71"

Cohesion: 0.4
Nodes (1): HealthHandler

### Community 72 - "Community 72"

Cohesion: 1.0
Nodes (3): getKey(), getKeycode(), getKeysym()

### Community 74 - "Community 74"

Cohesion: 0.5
Nodes (1): RREDecoder

### Community 75 - "Community 75"

Cohesion: 0.5
Nodes (1): TightPNGDecoder

### Community 76 - "Community 76"

Cohesion: 0.5
Nodes (1): RawDecoder

### Community 77 - "Community 77"

Cohesion: 0.5
Nodes (1): ZlibDecoder

### Community 81 - "Community 81"

Cohesion: 0.67
Nodes (2): getInitialLocale(), getLocaleFromNavigator()

### Community 82 - "Community 82"

Cohesion: 0.83
Nodes (3): handleToggle(), load(), $t()

### Community 86 - "Community 86"

Cohesion: 0.5
Nodes (3): GetClusterStatusResty(), ClusterInfo, ClusterStatusItem

### Community 87 - "Community 87"

Cohesion: 0.5
Nodes (3): ProxmoxStatusProvider, SettingsProvider, StateManager

### Community 89 - "Community 89"

Cohesion: 0.67
Nodes (1): CopyRectDecoder

### Community 90 - "Community 90"

Cohesion: 0.67
Nodes (1): ApiRequestError

### Community 95 - "Community 95"

Cohesion: 0.67
Nodes (1): IsMobile

### Community 98 - "Community 98"

Cohesion: 1.0
Nodes (2): if(), $t()

### Community 101 - "Community 101"

Cohesion: 0.67
Nodes (1): Config

### Community 102 - "Community 102"

Cohesion: 0.67
Nodes (2): ListResponse, Response

### Community 103 - "Community 103"

Cohesion: 0.67
Nodes (2): VNCProxyOptions, VNCProxyResponse

### Community 144 - "Community 144"

Cohesion: 1.0
Nodes (1): AppSettings

## Knowledge Gaps

- **166 isolated node(s):** `AppSettings`, `bucket`, `Rule`, `contextKey`, `AuditEntry` (+161 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Sidebar UI`** (34 nodes): `constants.ts`, `context.svelte.ts`, `index.ts`, `sidebar-content.svelte`, `sidebar-footer.svelte`, `sidebar-group-action.svelte`, `sidebar-group-content.svelte`, `sidebar-group-label.svelte`, `sidebar-group.svelte`, `sidebar-header.svelte`, `sidebar-input.svelte`, `sidebar-inset.svelte`, `sidebar-menu-action.svelte`, `sidebar-menu-badge.svelte`, `sidebar-menu-button.svelte`, `sidebar-menu-item.svelte`, `sidebar-menu-skeleton.svelte`, `sidebar-menu-sub-button.svelte`, `sidebar-menu-sub-item.svelte`, `sidebar-menu-sub.svelte`, `sidebar-menu.svelte`, `sidebar-provider.svelte`, `sidebar-rail.svelte`, `sidebar-separator.svelte`, `sidebar.svelte`, `sidebar-trigger.svelte`, `setSidebar()`, `SidebarState`, `.constructor()`, `.isMobile()`, `useSidebar()`, `child()`, `child()`, `child()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `noVNC Display`** (33 nodes): `Display`, `.absX()`, `.absY()`, `.autoscale()`, `.blitImage()`, `.clipViewport()`, `.constructor()`, `.copyImage()`, `._damage()`, `.drawImage()`, `.fillRect()`, `.flip()`, `.flush()`, `.getImageData()`, `.height()`, `.imageRect()`, `.pending()`, `._renderQPush()`, `._rescale()`, `.resize()`, `._resumeRenderQ()`, `.scale()`, `._scanRenderQ()`, `._setFillColor()`, `.toBlob()`, `.toDataURL()`, `.videoFrame()`, `.viewportChangePos()`, `.viewportChangeSize()`, `.width()`, `display.js`, `int.js`, `toSigned32bit()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `noVNC WebSocket`** (31 nodes): `Websock`, `._allocateBuffers()`, `.attach()`, `.close()`, `.constructor()`, `._expandCompactRQ()`, `.flush()`, `.init()`, `.off()`, `.on()`, `.open()`, `.readyState()`, `._recvMessage()`, `.rQpeek8()`, `.rQpeekBytes()`, `._rQshift()`, `.rQshift16()`, `.rQshift32()`, `.rQshift8()`, `.rQshiftBytes()`, `.rQshiftStr()`, `.rQshiftTo()`, `.rQskipBytes()`, `.rQwait()`, `._sQensureSpace()`, `.sQpush16()`, `.sQpush32()`, `.sQpush8()`, `.sQpushBytes()`, `.sQpushString()`, `websock.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 25`** (22 nodes): `gesturehandler.js`, `GestureHandler`, `.attach()`, `.constructor()`, `.detach()`, `._eventHandler()`, `._getAverageDistance()`, `._getAverageMovement()`, `._getPosition()`, `._hasDetectedGesture()`, `._isTwoTouchTimeoutRunning()`, `._longpressTimeout()`, `._pushEvent()`, `._startLongpressTimeout()`, `._startTwoTouchTimeout()`, `._stateToGesture()`, `._stopLongpressTimeout()`, `._stopTwoTouchTimeout()`, `._touchEnd()`, `._touchMove()`, `._touchStart()`, `._twoTouchTimeout()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 29`** (18 nodes): `cursor.js`, `Cursor`, `.attach()`, `._captureIsActive()`, `.change()`, `.clear()`, `.constructor()`, `.detach()`, `._handleMouseLeave()`, `._handleMouseMove()`, `._handleMouseOver()`, `._handleMouseUp()`, `._hideCursor()`, `.move()`, `._shouldShowCursor()`, `._showCursor()`, `._updatePosition()`, `._updateVisibility()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 33`** (16 nodes): `RA2Cipher`, `.constructor()`, `.makeMessage()`, `.receiveMessage()`, `.setKey()`, `RSAAESAuthenticationState`, `.approveServer()`, `.checkInternalEvents()`, `.constructor()`, `.disconnect()`, `.hasStarted()`, `.negotiateRA2neAuthAsync()`, `._waitApproveKeyAsync()`, `._waitCredentialsAsync()`, `._waitSockAsync()`, `ra2.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 36`** (15 nodes): `TightDecoder`, `._basicRect()`, `.constructor()`, `._copyFilter()`, `.decodeRect()`, `._fillRect()`, `._getScratchBuffer()`, `._gradientFilter()`, `._jpegRect()`, `._monoRect()`, `._paletteFilter()`, `._paletteRect()`, `._pngRect()`, `._readData()`, `tight.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 40`** (13 nodes): `lists_test.go`, `TestEnabledISOs_SetAndGet()`, `TestEnabledNodes_EmptyOnFreshDB()`, `TestEnabledNodes_Replace()`, `TestEnabledNodes_SetAndGet()`, `TestEnabledNodes_SetCreatesAuditEntry()`, `TestEnabledNodes_SetEmpty_ClearsAll()`, `TestEnabledStorages_Replace()`, `TestEnabledStorages_SetAndGet()`, `TestEnabledVMBRs_SetAndGet()`, `TestTags_Replace()`, `TestTags_SetAndGet()`, `TestTags_SetCreatesAuditEntry()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 51`** (10 nodes): `ZRLEDecoder`, `.constructor()`, `._decodePaletteTile()`, `.decodeRect()`, `._decodeRLEPaletteTile()`, `._decodeRLETile()`, `._getBitsPerPixelInPalette()`, `._readPixels()`, `._readRLELength()`, `zrle.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 55`** (9 nodes): `Cache[K, V]`, `.Clear()`, `.Delete()`, `.evictOldest()`, `.Get()`, `.GetOrSet()`, `.Set()`, `.SetWithTTL()`, `.Size()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 56`** (9 nodes): `health.go`, `MakeHealthHandler()`, `sendJSONResponse()`, `HealthHandler`, `.HealthCheckHandler()`, `.MethodNotAllowedHandler()`, `.NotFoundHandler()`, `.ProxmoxStatusHandler()`, `.RegisterRoutes()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 60`** (7 nodes): `HextileDecoder`, `.constructor()`, `.decodeRect()`, `._finishTile()`, `._startTile()`, `._subTile()`, `hextile.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 61`** (6 nodes): `eventtarget.js`, `EventTargetMixin`, `.addEventListener()`, `.constructor()`, `.dispatchEvent()`, `.removeEventListener()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 65`** (5 nodes): `JPEGDecoder`, `.constructor()`, `.decodeRect()`, `._readSegment()`, `jpeg.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 69`** (5 nodes): `Optional[T]`, `.Get()`, `.GetOrDefault()`, `.GetOrElse()`, `.IsPresent()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 70`** (5 nodes): `Result[T]`, `.IsErr()`, `.IsOk()`, `.Unwrap()`, `.UnwrapOr()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 71`** (5 nodes): `health.go`, `MakeHealthHandler()`, `HealthHandler`, `.Health()`, `.HealthProxmox()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 74`** (4 nodes): `RREDecoder`, `.constructor()`, `.decodeRect()`, `rre.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 75`** (4 nodes): `TightPNGDecoder`, `._basicRect()`, `._pngRect()`, `tightpng.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 76`** (4 nodes): `RawDecoder`, `.constructor()`, `.decodeRect()`, `raw.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 77`** (4 nodes): `ZlibDecoder`, `.constructor()`, `.decodeRect()`, `zlib.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 81`** (4 nodes): `index.ts`, `getInitialLocale()`, `getLocaleFromNavigator()`, `setLocale()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 89`** (3 nodes): `CopyRectDecoder`, `.decodeRect()`, `copyrect.js`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 90`** (3 nodes): `api.ts`, `ApiRequestError`, `.constructor()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 95`** (3 nodes): `is-mobile.svelte.ts`, `IsMobile`, `.constructor()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 98`** (3 nodes): `if()`, `$t()`, `+page.svelte`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 101`** (3 nodes): `config.go`, `Config`, `GetConfig()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 102`** (3 nodes): `types.go`, `ListResponse`, `Response`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 103`** (3 nodes): `vnc.go`, `VNCProxyOptions`, `VNCProxyResponse`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 144`** (2 nodes): `AppSettings`, `types.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions

_Questions this graph is uniquely positioned to answer:_

- **Why does `RFB` connect `noVNC RFB Protocol` to `State Manager`?**
  _High betweenness centrality (0.094) - this node is a cross-community bridge._
- **Why does `InitHandlers()` connect `HTTP Handlers` to `Auth & Guard`, `Rate Limit & Proxmox Health`, `API Router & Middleware`, `Community 28`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._
- **Are the 61 inferred relationships involving `errInternal()` (e.g. with `.GetTaskStatus()` and `.GetTaskLog()`) actually correct?**
  _`errInternal()` has 61 INFERRED edges - model-reasoned connections that need verification._
- **Are the 58 inferred relationships involving `writeJSON()` (e.g. with `.GetTaskStatus()` and `.GetTaskLog()`) actually correct?**
  _`writeJSON()` has 58 INFERRED edges - model-reasoned connections that need verification._
- **Are the 49 inferred relationships involving `errBadRequest()` (e.g. with `.GetTaskStatus()` and `.GetTaskLog()`) actually correct?**
  _`errBadRequest()` has 49 INFERRED edges - model-reasoned connections that need verification._
- **What connects `AppSettings`, `bucket`, `Rule` to the rest of the system?**
  _166 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Admin API Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.03 - nodes in this community are weakly interconnected._
