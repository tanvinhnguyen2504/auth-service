# Rules: querying the database

Applies to every `internal/*/repositories.go` — the only layer allowed to speak SQL.
Handlers own HTTP, services own business rules, repositories own queries and nothing else.

---

## 1. Collect arguments into a named `args` slice

Every query function has the same three-part shape: `const query`, then `args`, then the call.

### Avoid
```go
func (r *repository) DeleteSession(ctx context.Context, id string) error {
	const query = `DELETE FROM sessions WHERE id = $1`

	if _, err := r.pgPool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}
```

### Prefer
```go
func (r *repository) DeleteSession(ctx context.Context, id string) error {
	const query = `DELETE FROM sessions WHERE id = $1`
	args := []any{
		id,
	}

	if _, err := r.pgPool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}
```

Do this even for a single argument. The point is that every query function reads the
same, and adding a second parameter later is a one-line insert rather than a rewrite of
the call.

---

## 2. One argument per line, in placeholder order

This is where the rule earns its keep. Inline arguments put several values on one line,
and when they share a type the compiler cannot help you.

### Avoid
```go
	// $1..$6 — six strings and a bool on one line. Swapping p.Email and p.Name
	// compiles cleanly and silently writes the wrong columns.
	err := r.pgPool.QueryRow(ctx, query,
		p.Subject, p.Email, p.EmailVerified, p.Name, p.AvatarURL, p.HD,
	).Scan(&u.ID, &u.Email, ...)
```

### Prefer
```go
	// One per line, in $n order, so the SQL and the slice can be read side by side.
	args := []any{
		p.Subject,       // $1 google_sub
		p.Email,         // $2 email
		p.EmailVerified, // $3 email_verified
		p.Name,          // $4 name
		p.AvatarURL,     // $5 avatar_url
		p.HD,            // $6 hd
	}

	err := r.pgPool.QueryRow(ctx, query, args...).Scan(...)
```

Annotate the placeholder number and column when the query has more than two arguments.

---

## 3. `[]any`, never `[]interface{}`

They are the same type — `any` is an alias added in Go 1.18 — but the codebase already
uses `any` everywhere, including `queryOne(ctx, pool, query string, args ...any)`.

### Avoid
```go
	args := []interface{}{id}
```

### Prefer
```go
	args := []any{
		id,
	}
```

---

## 4. Values are always placeholders, never formatted into the SQL

`fmt.Sprintf` may compose the *shape* of a query — placeholder numbers, a column list —
but a value reaches the database only through `args`. There is no exception.

### Avoid
```go
	// SQL injection. A member_id of `' OR 1=1 --` returns every row.
	query := fmt.Sprintf("SELECT %s FROM players WHERE member_id = '%s'", playerColumns, memberID)
	rows, err := r.pgPool.Query(ctx, query)
```

### Prefer
```go
	// Build the predicate list and the args together, so a clause can never be
	// added without its value — the index comes from len(args), not a hand-count.
	var (
		filters = []string{"TRUE"}
		args    []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		filters = append(filters, fmt.Sprintf(clause, len(args)))
	}

	if params.DivisionID != "" {
		add("division_id = $%d", params.DivisionID)
	}
	if params.SkillLevel > 0 {
		add("skill_level = $%d", params.SkillLevel)
	}

	query := fmt.Sprintf("SELECT %s FROM players WHERE %s ORDER BY id",
		playerColumns, strings.Join(filters, " AND "))

	rows, err := r.pgPool.Query(ctx, query, args...)
```

For a `LIKE` filter, escape the metacharacters too, or a user searching `100%` gets a
wildcard:

```go
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(params.Query)
	add("full_name ILIKE '%%' || $%d || '%%'", escaped)
```

---

## 5. Wrap every error with package and operation

A bare `err` from pgx says `no rows in result set` with no hint of which query. The
wrapped form is what the error envelope logs.

### Avoid
```go
	if _, err := r.pgPool.Exec(ctx, query, args...); err != nil {
		return err
	}
```

### Prefer
```go
	if _, err := r.pgPool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
```

Format is `"<package>: <operation>: %w"`. Include the identifier when it narrows things
down — `fmt.Errorf("auth: get user %s: %w", id, err)` — but never a credential or a
whole request body.

---

## 6. Translate `pgx.ErrNoRows` into a package sentinel

A repository returns its own sentinel so the service can map it to a status code without
importing pgx. Leaking `pgx.ErrNoRows` upward makes the service layer depend on the
driver.

### Avoid
```go
func (r *repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	user, err := queryOne[User](ctx, r.pgPool, query, args...)
	if err != nil {
		return nil, fmt.Errorf("auth: user by id: %w", err) // pgx.ErrNoRows leaks
	}
	return user, nil
}
```

### Prefer
```go
var ErrNoUser = errors.New("auth: no user")

func (r *repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	user, err := queryOne[User](ctx, r.pgPool, query, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoUser
	}
	if err != nil {
		return nil, fmt.Errorf("auth: user by id: %w", err)
	}
	return user, nil
}
```

The service then maps the sentinel to an error code — that mapping is the service's job,
never the repository's:

```go
func asAPIError(err error) error {
	if errors.Is(err, ErrNoUser) {
		return handler.Error(handler.CodeNotFound, err)
	}
	return err
}
```

For writes, check `tag.RowsAffected() == 0` rather than expecting an error — an `UPDATE`
or `DELETE` matching nothing is not a driver error.

---

## 7. Keep an explicit `Scan` list in column-list order

Prefer `pgx.RowToStructByName` (via `queryOne`) so columns map by name. When you must
scan explicitly — a computed column like `(xmax = 0) AS is_new_user` has no struct field,
so `RowToStructByName` rejects the extra column — keep the scan list and the column
constant adjacent and in the same order.

### Avoid
```go
	// Column order lives in one file and the scan order in another. A column
	// added to userColumns silently reads into the wrong field.
	const userColumns = "id, email, email_verified, name, avatar_url, created_at, last_login_at"
	...
	.Scan(&u.ID, &u.Name, &u.Email, &u.AvatarURL, ...) // drifted
```

### Prefer
```go
	// Scanned explicitly: is_new_user has no field on User. Keep this list in
	// userColumns order.
	const userColumns = "id, email, email_verified, name, avatar_url, created_at, last_login_at"
	...
	.Scan(
		&u.ID,            // id
		&u.Email,         // email
		&u.EmailVerified, // email_verified
		&u.Name,          // name
		&u.AvatarURL,     // avatar_url
		&u.CreatedAt,     // created_at
		&u.LastLoginAt,   // last_login_at
		&isNewUser,       // (xmax = 0)
	)
```

Better still, give the struct a method that returns its scan targets next to the column
constant, so the two cannot be changed independently.

---

## 8. Always take `ctx` and pass it through

Every pgx call takes the request context, so a cancelled request stops work in the
database instead of holding a pooled connection.

### Avoid
```go
func (r *repository) DeleteSession(id string) error {
	_, err := r.pgPool.Exec(context.Background(), query, id)
```

### Prefer
```go
func (r *repository) DeleteSession(ctx context.Context, id string) error {
	_, err := r.pgPool.Exec(ctx, query, args...)
```

`ctx` is always the first parameter. Use `context.Background()` only where no request
exists — the startup cleanup sweep in `auth.StartSessionCleanup`, for example.

---

## 9. Bound every list query

An unbounded `SELECT` over a growing table is a latency incident waiting for enough
rows. Lists page by keyset, never offset: a row inserted mid-scroll cannot shift a page
boundary.

### Avoid
```go
	const query = `SELECT ` + playerColumns + ` FROM players`
	rows, err := r.pgPool.Query(ctx, query)
```

### Prefer
```go
	// Limit+1 is a lookahead: its presence tells the service a next page exists,
	// which is cheaper than a second COUNT query.
	args := []any{
		params.AfterID,    // $1 cursor
		params.Limit + 1,  // $2
	}

	const query = `SELECT ` + playerColumns + `
FROM players
WHERE id > $1
ORDER BY id
LIMIT $2`

	rows, err := r.pgPool.Query(ctx, query, args...)
```

`model.CursorParams.Normalise()` already enforces a default and a maximum limit — call it
in the service before the query runs, so an unbounded scan cannot be requested.

---

## 10. Close `rows` and check `rows.Err()`

A `Query` that is not drained leaks its connection back to the pool late; a loop that
ignores `rows.Err()` silently truncates the result on a mid-iteration failure.

### Avoid
```go
	rows, _ := r.pgPool.Query(ctx, query, args...)
	for rows.Next() {
		var p Player
		rows.Scan(p.scanDest()...)
		out = append(out, p)
	}
	return out, nil
```

### Prefer
```go
	rows, err := r.pgPool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("players: list: %w", err)
	}
	defer rows.Close()

	out := []Player{} // non-nil: encodes as [] rather than null
	for rows.Next() {
		var p Player
		if err := rows.Scan(p.scanDest()...); err != nil {
			return nil, fmt.Errorf("players: list scan: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("players: list rows: %w", err)
	}

	return out, nil
```
