# compose

Generic utilities for composing nested Go structs from flat relational query results.

`compose` does not load data, cache queries, or generate SQL. It takes slices you already fetched and merges them into a tree — nothing more.

## Why This Library Exists

Every project with relational data eventually faces the same problem: you query parents, you need the children, and suddenly you're writing nested for-loops or N+1 queries.

ORMs solve this with eager loading (`INCLUDE`, `Load()`, `with()`) — they query each level with `WHERE IN`, then reconstruct the graph in memory. But if you don't use an ORM (sqlx, sqlc, pgx, database/sql), you end up writing the same grouping-and-merging code by hand every time.

This library extracts that exact pattern into four generic functions. Nothing more, nothing less.

## When to Use Compose

You write raw SQL or use a lightweight query library, and you need to build nested responses like:

- `Category → Products → Variants` (e-commerce)
- `Author → Posts → Comments` (blog)
- `Continent → Country → City → District` (geo hierarchy)
- `Order → Items → Shipments` (commerce)

Without compose, the options are:

1. **N+1 queries** — one query per parent. Simple but doesn't scale.
2. **Single massive JOIN** — returns flat rows that are hard to map into nested structs, and per-level filtering is impossible.
3. **Database JSON aggregation** — works great but locks you into PostgreSQL-specific syntax and loses type safety.
4. **A full ORM** — solves the problem but brings a large API surface, runtime deps, and vendor lock-in.

Compose is for case 4 when you don't want case 4.

## What Compose Does NOT Do

- **No data loading** — you fetch the data. Compose only merges it.
- **No caching** — no query cache, result cache, or write-through.
- **No pagination** — apply `LIMIT`/`OFFSET` in your SQL before passing data in.
- **No query generation** — you write the SQL.
- **No lazy loading** — all levels are fetched eagerly upfront.
- **No N+1 detection** — it cannot detect or prevent N+1; it gives you the tools to avoid them.
- **No schema migration** — not a migration tool.

## The Pattern

```
SELECT * FROM categories                        → []Parent
ExtractIDs(parents)                             → []id
SELECT * FROM children WHERE parent_id IN (…)   → []Child
GroupBy(children, parent_id)                    → map[id][]Child
MergeChildren(parents, grouped)                 → parents now have children
Map(parents, toDTO)                             → []ParentDTO (optional)
```

Each level is an independent query with its own filters, pagination, and sorting.

## Functions

```go
// Groups a slice into a map keyed by K.
func GroupBy[T any, K comparable](objects []T, keyFunc func(T) K) map[K][]T

// Extracts a key from each element, preserving order.
func ExtractIDs[T any, KEY any](objects []T, getKeyFunc func(T) KEY) []KEY

// Merges grouped children into their parents in-place.
func MergeChildren[TParent, TChild any, K comparable](
    parents []TParent,
    childMap map[K][]TChild,
    parentKeyFunc func(TParent) K,
    setChildrenFunc func(*TParent, []TChild),
)

// Maps a slice of type I to type O using a transform function.
func Map[I any, O any](elements []I, mapFunc func(I) O) []O
```

## Use Cases

### With sqlx

```go
import (
    "github.com/AnthonyLonsMax/compose"
    "github.com/jmoiron/sqlx"
)

var cats []Category
db.Select(&cats, "SELECT id, name FROM categories WHERE active = 1")

ids := compose.ExtractIDs(cats, func(c Category) int { return c.ID })
q, args, _ := sqlx.In(
    "SELECT id, category_id, name, price FROM products WHERE category_id IN (?) AND published = 1",
    ids,
)
var prods []Product
db.Select(&prods, q, args...)

byCat := compose.GroupBy(prods, func(p Product) int { return p.CategoryID })
compose.MergeChildren(cats, byCat,
    func(c Category) int { return c.ID },
    func(c *Category, ps []Product) { c.Products = ps },
)
```

### With sqlc

```go
cats, _ := queries.ListCategories(ctx, db)

ids := compose.ExtractIDs(cats, func(c Category) int32 { return c.ID })
prods, _ := queries.ListProductsByCategoryIDs(ctx, db, ids)

byCat := compose.GroupBy(prods, func(p Product) int32 { return p.CategoryID })
compose.MergeChildren(cats, byCat,
    func(c Category) int32 { return c.ID },
    func(c *Category, ps []Product) { c.Products = ps },
)
```

### Deep Nesting (3+ levels)

Query top-down, reconstruct bottom-up:

```go
continents := queryContinents(db)
countries  := queryCountriesByIDs(db, compose.ExtractIDs(continents, ...))
cities     := queryCitiesByIDs(db, compose.ExtractIDs(countries, ...))
districts  := queryDistrictsByIDs(db, compose.ExtractIDs(cities, ...))

compose.MergeChildren(cities,     compose.GroupBy(districts, ...), ...)
compose.MergeChildren(countries,  compose.GroupBy(cities, ...), ...)
compose.MergeChildren(continents, compose.GroupBy(countries, ...), ...)
```

No nested loops. Any depth N requires N queries and N-1 flat `MergeChildren` calls.

See `compose_test.go` for full 3-level, 4-level, and 6-level examples.

## Comparison

| Concern | compose | JSON in SQL | ORM |
|---------|---------|-------------|-----|
| Per-level filters | Yes | No (single query) | Yes |
| DB-agnostic | Any driver | PostgreSQL mostly | Most ORMs |
| Dependencies | Zero | None | Heavy |
| Type-safe | Generics | `[]byte` | Usually |
| Learning curve | 4 functions | Complex SQL | Large API surface |
| N+1 elimination | Batch WHERE IN | Single query | Eager loading |

## Why "Compose"?

The library does not load anything — it **composes** flat slices into nested structures. The name reflects the actual job: take parts (parents, children) and compose them into a whole.
