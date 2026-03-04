# CODING STANDARDS

Generate/modify Go code per these rules. No placeholders, no incomplete implementations.

## ARCHITECTURE
- Dependency direction (inward only): `entity` ← `usecase` ← `repository` / `presenter` / `handler`
- `handler` → usecase interface + presenter + pkg. Never imports repository.
- `usecase/service.go` → its own `interface.go` + entity + pkg. Never imports another usecase.
- `entity` → pkg/id, pkg/glad, pkg/logger, pkg/common only.
- JSON tags on presenter structs only — never on entity structs.
- All contracts in `usecase/<domain>/interface.go`; impls in `repository/<domain>_pgsql.go` and `usecase/<domain>/service.go`.

## FILE ORDER
`copyright` → `package` → `imports (stdlib / internal / external)` → `const` → `var` → `types` → `New*` → exported methods → private helpers

## NAMING
| Kind | Rule | Example |
|---|---|---|
| Exported type/func | PascalCase | `EventPGSQL`, `NewService` |
| Unexported | camelCase | `cRepo`, `scanRow` |
| Single-method interface | `-er` suffix | `EventReader`, `EventWriter` |
| Repository struct | `<Domain>PGSQL` | `EventPGSQL` |
| Use-case struct | always `Service` | `type Service struct` |
| DB column constants | `EntityDB<Column>` | `EventDBSyncStatus` |
| Entity enum constants | `EntityTypeValue` | `EventStatusDraft` |
| JSON keys | camelCase, omitempty if optional | `json:"tenantID"`, `json:"notes,omitempty"` |
| Optional fields | pointer | `ExtID *string`, `IsActive *bool` |
| Feature flags | `enable`/`feat` prefix | `enableShortURL`, `featSyncIAI` |
| Packages / files | lowercase, underscores | `event`, `event_pgsql.go` |
| Acronyms | fully uppercase | `ID`, `URL`, `GRPC`, `HTTP` |

## COMMENTS
- Copyright block on every `.go` file.
- Every exported type: one-line doc saying what it *represents*.
- Every constructor: list key dependencies and why.
- Every exported method: behavior, branching, side effects, external calls.
- Inline only for non-obvious decisions: `// note:`, `// TODO: <name>`, `// CODESMELL: <reason>`.

## CODE STYLE
- Max: **120 chars/line** (tab=4), **50 lines/func**, **500 lines/file**.
- No `panic` outside `main()`. No mutable package-level `var`.
- `ctx context.Context` is always the first param on I/O functions.
- Pre-allocate slices: `make([]*T, 0, len(ids))`.
- Handlers: package-level funcs returning `http.Handler`, verb-first: `createEvent`, `listEventsV2`.
- Presenter conversions: `To<Entity>(ctx, ...)` → entity; `From<Entity>(e *entity.X) error` → DTO.

## ERROR HANDLING
```
Repository → fmt.Errorf("failed to …: %w", err)   or   return sql.ErrNoRows as-is
Service    → glad.NewErrorWithContext(ctx, glad.NotFound, "short msg", "detail with IDs")
Handler    → glad.HandleError(err, w); return
```
If handler receives a raw `error`, type-assert to `*glad.Error`; if not ok, wrap with `glad.GenericError`. Never use `http.Error`. Never silently ignore errors.

## LOGGING (`l "ac9/glad/pkg/logger"`)
- Prefer `l.Log.CtxWarnf(ctx, "msg, key=%v, err=%v", val, err)` when ctx is in scope.
- Fall back to `l.Log.Warnf(...)` without context.
- Log at every error path before returning. Never log secrets/tokens/PII.
- Levels: DEBUG=verbose dev data · INFO=normal ops · WARN=recoverable · ERROR=unexpected.

## DATABASE (`ac9/glad/pkg/util`)
| Call | Rule |
|---|---|
| `util.DBTimeNow()` | All timestamp inserts/updates (UTC, `FormatDateTimeMS`) |
| `util.SafeClose(x)` | `defer` on every `stmt` and `rows` |
| `util.GenBulkInsertPGSQL(table, cols, n, fn)` | Bulk INSERT — parameterized |
| `util.GenBulkUpdatePGSQL(table, key, cols, n, fn)` | Bulk UPDATE via CASE/WHEN |
| `util.GenBulkDeletePGSQL(table, cols, n, fn)` | Bulk DELETE |
| `util.BuildQueryWhereClauseIn(extra, field, n, fn, page, limit)` | WHERE field IN + pagination |
| `util.GenCustomUpdatePGSQL(table, fields, where)` | Selective UPDATE with WHERE map |
| `pq.Array(slice)` | PostgreSQL ANY($n) array params |
| `sql.NullString/Int64/Bool` | Nullable column scanning |
- Always `db.PrepareContext` → `stmt.QueryContext/ExecContext`. DB column names as `const`, never inline strings.

## OTHER PKG/UTIL HELPERS
| Function | Purpose |
|---|---|
| `util.NewString(s)` | `*string` from literal |
| `util.Contains[T](slice, val)` | Generic membership check |
| `util.IsEmptyOrWhiteSpace(s)` | Blank string check |
| `util.GetUniqueValues[T](slice)` | Deduplicate preserving order |
| `util.GetValuesFromMap[K,T](m)` | Map values → slice |
| `util.ParseDate(s)` | YYYY-MM-DD or RFC3339 → `*time.Time` |
| `util.ToStandardTZ(tz)` | Short alias → IANA (input layer) |
| `util.ToDisplayTZ(tz)` | IANA → short alias (presenter layer) |
| `util.GetStrEnvOrConfig(k, fb)` / `GetIntEnvOrConfig` / `GetBoolEnvOrConfig` | Env var with fallback |
| `util.GetEncodedPermissions(ctx, aliases)` | Permission aliases → base64 bitmask (OPA) |

## PKG/COMMON — HTTP HELPERS (`ac9/glad/pkg/common`)
Never parse headers or query params manually. Use:

| Function | Returns / Behavior |
|---|---|
| `common.HttpGetTenantID(w,r)` | `X-GLAD-TenantID` → `id.ID`; writes+returns err if missing |
| `common.HttpGetAccountID(w,r)` | `X-GLAD-AccountID` → `id.ID`; writes+returns err if missing |
| `common.HttpGetPageParams(w,r)` | `page`/`limit`; default 1/20, max 100 |
| `common.HttpGetCursorParams(w,r)` | `cursor` as `id.ID` + `limit`; max 100 |
| `common.HttpGetCursorNumberParams(w,r)` | `cursor` as `int` + `limit` |
| `common.HttpQueryGetStr(key,w,r)` | Required string query param; `ParameterAbsent` if missing |
| `common.HttpQueryGetID(key,w,r)` | Required `id.ID` query param; `ParameterInvalid` if bad |
| `common.HTTPGetSortOrderParams(w,r,default,list)` | `sort_by`/`order`; validates allowlist; default ASC |
| `common.DecodeJSON(ctx,w,r,&v)` | Decode body; writes `RequestInvalid` on failure |
| `common.EncodeJSON(ctx,w,v)` | Encode response; writes `GenericError` on failure |
| `common.ContextTraceID(ctx)` | Trace ID string for log lines |

Header constants: `common.HttpHeaderTenantID/AccountID/TotalCount/Authorization/ContentType/AppVersion`
Query constants: `common.HttpParamPage/Limit/Cursor/Query/SortBy/Order`

## INFRASTRUCTURE (`ac9/glad/infrastructure/`)
All external clients live here; inject via constructor as interfaces — never instantiate inside use cases or handlers.

| Package | Interface method | Purpose |
|---|---|---|
| `gcl` | `Send(ctx, method, uri, msg, headers...)` | Inter-service HTTP (Glad Communication Layer) |
| `salesforce` | — | CRM sync |
| `zoom` | — | Live darshan / meetings |
| `s3client` | — | Media storage |
| `awssecret` | — | Secrets at startup |
| `fcm` / `apns` | — | Push notifications |
| `email` | — | Transactional email |
| `googlemaps` | — | Geolocation |
| `tinyurl` / `rebrandly` | — | Short URL creation |

Rules: define interface in `infrastructure/<pkg>/interface.go` · inject via constructor · mock from `infrastructure/<pkg>/mock/` · always set HTTP timeouts.

## TESTING & LINTING
- Table-driven tests. Mocks via `mockgen` on usecase interfaces. In-memory impls in `inmem_<domain>.go`. 80%+ coverage.
- `go test -race` clean. SQL parameterized only. No hardcoded secrets. Input validated via OPA at handler boundary.
- Must pass: `gofmt -s` · `golangci-lint run` (`errcheck`, `govet`, `lll`, `staticcheck`, `unused`, `revive`)
