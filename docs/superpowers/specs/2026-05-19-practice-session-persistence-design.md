## Summary

Fix terminal unavailability caused by `practice_sessions` living only in API process memory while browser auth already persists in MySQL.

## Root Cause

- Browser auth survives API restarts because `users` and `user_sessions` use `MySQLStore`.
- `practice_sessions` still use `service.NewInMemoryPracticeSessionStore()` inside `services/api/internal/http/router.go`.
- After an API restart, the browser still has a valid `gitgym_session` cookie, but `current practice session` and terminal lookup depend on an empty in-memory session map.
- The frontend can reconnect with a stale session id / runner ref path and only receives a generic terminal transport failure.

## Design

- Extend `store.MySQLStore` to also implement `service.PracticeSessionStore`.
- Persist `practice_sessions` rows through:
  - `CreatePracticeSession`
  - `CurrentPracticeSession`
  - `PracticeSessionByID`
- Update API default wiring so that when the auth store also implements `service.PracticeSessionStore`, `PracticeService` is built on that persistent store instead of the in-memory fallback.
- Keep the in-memory store only as a fallback for tests or explicitly injected non-MySQL environments.

## Query Rules

- `CreatePracticeSession` inserts the full session record.
- `CurrentPracticeSession` returns the most recent active session for the user.
- `PracticeSessionByID` returns the session by primary key.
- No terminal protocol changes are required.

## Verification

- Add a failing API test that creates a session through one router instance, rebuilds the router with the same persistent backing store, and verifies `GET /api/v1/practice-sessions/current` still returns the session.
- Keep existing API and frontend tests green after the wiring change.
