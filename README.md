# Appointment Booking Backend (Go + Gin + MySQL)

This repository implements a timezone-aware appointment booking backend:
- Per-coach timezone-aware weekly availability (IANA timezone, e.g. "Asia/Kolkata").
- Generate 30-minute slots for a coach on a given date (date interpreted in coach's timezone).
- Book a 30-minute slot (validates it lies within coach availability for that local date).
- Prevent double-booking via a unique DB constraint + transaction.
- Cancel bookings.
- OpenAPI spec and Postman collection included.
- Integration tests (run only if TEST_MYSQL_DSN provided).

## Prereqs
- Go 1.25+
- MySQL server

## Create DB & run migrations
1. Create DB:
   ```sql
   CREATE DATABASE bookingdb;
   ```
2. Apply migrations:
   ```bash
   mysql -u root -p bookingdb < internal/models/migrations.sql
   ```

## Environment
Set environment variables:
- `MYSQL_DSN` (required) — e.g. `user:pass@tcp(127.0.0.1:3306)/bookingdb?parseTime=true`
- `PORT` (8080)

## Run
```bash
export MYSQL_DSN="user:pass@tcp(127.0.0.1:3306)/bookingdb?parseTime=true"
export PORT=8080
go run ./cmd/server
```

## Endpoints (summary)
- `POST /coaches`
  - body: `{ "name":"Coach A", "timezone":"Asia/Kolkata" }`
- `POST /coaches/availability`
  - body: `{ "coach_id":1, "day":2, "start_time":"09:00", "end_time":"14:00" }`
- `GET /users/slots?coach_id=1&date=2025-10-28`
  - returns slots (RFC3339 UTC) for coach local date
- `POST /users/bookings`
  - body: `{ "user_id":101, "coach_id":1, "datetime":"2025-10-28T09:30:00+05:30" }`
- `GET /users/bookings?user_id=101`
- `DELETE /users/bookings/:id`

## Tests
Integration tests (optional) are in `tests/integration_test.go`. They run only when `TEST_MYSQL_DSN` is set:
```bash
export TEST_MYSQL_DSN="user:pass@tcp(127.0.0.1:3306)/test_bookingdb?parseTime=true"
go test ./tests -v
```
## Notes
- All booking times are stored in UTC in the DB.
- Availability is stored as `HH:MM` strings and interpreted in the coach's timezone when generating slots.
- The project intentionally uses raw SQL with `database/sql`
