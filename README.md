# compose

Generic utilities for composing nested Go structs from flat relational query results.

`compose` does not load data, cache queries, or generate SQL. It takes slices you already fetched and merges them into a tree — nothing more.

```
go get github.com/AnthonyLonsMax/compose@v1.0.1
```

## Why This Library Exists

Every project with relational data eventually faces the same problem: you query parents, you need the children, and suddenly you're writing nested for-loops or N+1 queries.

ORMs solve this with eager loading (`INCLUDE`, `Load()`, `with()`) — they query each level with `WHERE IN`, then reconstruct a graph of entities in memory. This approach treats the result as a **graph of composed entities** rather than flat rows, and crucially it allows **complex logic at each level**: different `WHERE` filters, pagination, sorting, or business rules per nesting level — something a single JOIN can never do.

`compose` extracts that exact pattern from the ORM world into four generic functions. You keep writing your own SQL, but you get the same graph-composition power that Laravel, Hibernate, or Entity Framework provide internally.

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

## Examples

Run the full working examples to see compose in action:

```bash
go run ./_examples/blog/
go run ./_examples/ecommerce/
```

### Blog output (Author → Posts → Comments)

```json
[
  {
    "id": 1,
    "name": "Alice",
    "posts": [
      {
        "id": 1,
        "author_id": 1,
        "title": "First Post",
        "comments": [
          { "id": 1, "post_id": 1, "author": "Charlie", "body": "Great post!" },
          { "id": 2, "post_id": 1, "author": "Diana", "body": "Thanks!" }
        ]
      },
      {
        "id": 2,
        "author_id": 1,
        "title": "Second Post",
        "comments": [
          { "id": 3, "post_id": 2, "author": "Eve", "body": "Nice write-up" }
        ]
      }
    ]
  },
  {
    "id": 2,
    "name": "Bob",
    "posts": [
      {
        "id": 3,
        "author_id": 2,
        "title": "Hello World",
        "comments": [
          { "id": 4, "post_id": 3, "author": "Frank", "body": "First comment!" },
          { "id": 5, "post_id": 3, "author": "Grace", "body": "Awesome blog" }
        ]
      }
    ]
  }
]
```

### E-commerce output (Category → Product → Variant → Inventory)

With per-level filters: only active categories, published products with price > 0, variants with stock > 0, inventory with quantity > 0.

```json
[
  {
    "id": 1,
    "name": "Clothing",
    "active": true,
    "products": [
      {
        "id": 1,
        "category_id": 1,
        "name": "T-Shirt",
        "price": 19.99,
        "published": true,
        "variants": [
          {
            "id": 1,
            "product_id": 1,
            "name": "Small",
            "stock": 10,
            "inventory": [
              { "id": 1, "variant_id": 1, "warehouse": "Warehouse A", "quantity": 20 },
              { "id": 2, "variant_id": 1, "warehouse": "Warehouse B", "quantity": 5 }
            ]
          },
          {
            "id": 3,
            "product_id": 1,
            "name": "Large",
            "stock": 5,
            "inventory": [
              { "id": 3, "variant_id": 3, "warehouse": "Warehouse A", "quantity": 10 }
            ]
          }
        ]
      }
    ]
  },
  {
    "id": 2,
    "name": "Electronics",
    "active": true,
    "products": [
      {
        "id": 3,
        "category_id": 2,
        "name": "Headphones",
        "price": 99.99,
        "published": true,
        "variants": [
          {
            "id": 4,
            "product_id": 3,
            "name": "Wired",
            "stock": 3,
            "inventory": [
              { "id": 5, "variant_id": 4, "warehouse": "Warehouse B", "quantity": 8 }
            ]
          }
        ]
      }
    ]
  }
]
```

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

The name reflects the actual job: take parts (flat parent/child slices) and **compose** them into a nested whole. No loading, no fetching — just composition.
