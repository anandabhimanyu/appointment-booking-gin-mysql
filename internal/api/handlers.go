package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

// DTOs
type CreateCoachReq struct {
	Name     string `json:"name" binding:"required"`
	Timezone string `json:"timezone" binding:"required"`
}

type AvailabilityReq struct {
	CoachID   int    `json:"coach_id" binding:"required"`
	Day       int    `json:"day" binding:"required"`
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`
}

type BookingReq struct {
	UserID   int    `json:"user_id" binding:"required"`
	CoachID  int    `json:"coach_id" binding:"required"`
	Datetime string `json:"datetime" binding:"required"` // RFC3339 with tz
}

// RegisterRoutes registers HTTP routes
func RegisterRoutes(r *gin.Engine, db *sql.DB) {
	r.POST("/coaches", CreateCoachHandler(db))
	r.POST("/coaches/availability", PostAvailabilityHandler(db))
	r.GET("/users/slots", GetSlotsHandler(db))
	r.POST("/users/bookings", PostBookingHandler(db))
	r.GET("/users/bookings", GetUserBookingsHandler(db))
	r.DELETE("/users/bookings/:id", CancelBookingHandler(db))
}

// createCoachHandler - POST /coaches
func CreateCoachHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateCoachReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// validate timezone
		if _, err := time.LoadLocation(req.Timezone); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timezone"})
			return
		}
		q := `INSERT INTO coaches (name, timezone, created_at) VALUES (?, ?, UTC_TIMESTAMP())`
		res, err := db.Exec(q, req.Name, req.Timezone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		id, _ := res.LastInsertId()
		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

// postAvailabilityHandler - POST /coaches/availability
func PostAvailabilityHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r AvailabilityReq
		if err := c.ShouldBindJSON(&r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if r.Day < 0 || r.Day > 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "day must be 0..6"})
			return
		}
		if _, err := time.Parse("15:04", r.StartTime); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start_time must be HH:MM"})
			return
		}
		if _, err := time.Parse("15:04", r.EndTime); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be HH:MM"})
			return
		}
		if r.StartTime >= r.EndTime {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start_time must be before end_time"})
			return
		}

		q := `INSERT INTO coach_availabilities (coach_id, day_of_week, start_time, end_time, created_at, updated_at) VALUES (?, ?, ?, ?, UTC_TIMESTAMP(), UTC_TIMESTAMP())`
		res, err := db.Exec(q, r.CoachID, r.Day, r.StartTime, r.EndTime)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		id, _ := res.LastInsertId()
		c.JSON(http.StatusCreated, gin.H{"id": id})
	}
}

// getSlotsHandler - GET /users/slots?coach_id=1&date=YYYY-MM-DD
// date is interpreted in the coach's timezone (so slots correspond to that local date)
func GetSlotsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		coachID := c.Query("coach_id")
		dateStr := c.Query("date")
		if coachID == "" || dateStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "coach_id and date required"})
			return
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date must be YYYY-MM-DD"})
			return
		}

		// fetch coach timezone
		var tz string
		if err := db.QueryRow(`SELECT timezone FROM coaches WHERE id = ?`, coachID).Scan(&tz); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "coach not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		loc, _ := time.LoadLocation(tz)

		weekday := int(date.In(loc).Weekday())

		// load availabilities for that weekday
		rows, err := db.Query(`SELECT start_time, end_time FROM coach_availabilities WHERE coach_id = ? AND day_of_week = ?`, coachID, weekday)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type av struct{ S, E string }
		avs := []av{}
		for rows.Next() {
			var s, e string
			if err := rows.Scan(&s, &e); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			avs = append(avs, av{S: s, E: e})
		}

		// generate slots in coach local timezone, convert to UTC for comparisons and response
		slots := []time.Time{}
		for _, a := range avs {
			sp, _ := time.ParseInLocation("15:04", a.S, loc)
			ep, _ := time.ParseInLocation("15:04", a.E, loc)
			start := time.Date(date.Year(), date.Month(), date.Day(), sp.Hour(), sp.Minute(), 0, 0, loc)
			end := time.Date(date.Year(), date.Month(), date.Day(), ep.Hour(), ep.Minute(), 0, 0, loc)
			for t := start; !t.Add(30 * time.Minute).After(end); t = t.Add(30 * time.Minute) {
				slots = append(slots, t.UTC())
			}
		}

		// compute day bounds in UTC for this coach local date
		startOfDayLocal := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
		endOfDayLocal := startOfDayLocal.Add(24 * time.Hour)
		startUTC := startOfDayLocal.UTC()
		endUTC := endOfDayLocal.UTC()

		// fetch existing bookings between these bounds
		bookedMap := map[string]bool{}
		bq := `SELECT start_time FROM bookings WHERE coach_id = ? AND start_time >= ? AND start_time < ?`
		brows, err := db.Query(bq, coachID, startUTC, endUTC)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer brows.Close()

		for brows.Next() {
			var st time.Time
			if err := brows.Scan(&st); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			bookedMap[st.UTC().Format(time.RFC3339)] = true
		}

		resp := []string{}
		for _, s := range slots {
			key := s.UTC().Format(time.RFC3339)
			if !bookedMap[key] {
				resp = append(resp, key)
			}
		}

		c.JSON(http.StatusOK, gin.H{"slots": resp, "timezone": tz})
	}
}

// postBookingHandler - POST /users/bookings
func PostBookingHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r BookingReq
		if err := c.ShouldBindJSON(&r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		start, err := time.Parse(time.RFC3339, r.Datetime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "datetime must be RFC3339 with timezone"})
			return
		}
		start = start.UTC()
		end := start.Add(30 * time.Minute)

		// load coach timezone
		var tz string
		if err := db.QueryRow(`SELECT timezone FROM coaches WHERE id = ?`, r.CoachID).Scan(&tz); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "coach not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		loc, _ := time.LoadLocation(tz)

		// validate 30-minute boundary
		if !IsOnThirtyMinuteBoundary(start) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "slot must be on 30-minute boundary (minute 00 or 30)"})
			return
		}

		// Validate slot falls within availability for the coach's local date
		startLocal := start.In(loc)
		localDate := time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, loc)
		weekday := int(localDate.Weekday())

		rows, err := db.Query(`SELECT start_time, end_time FROM coach_availabilities WHERE coach_id = ? AND day_of_week = ?`, r.CoachID, weekday)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		allowed := false
		for rows.Next() {
			var s, e string
			if err := rows.Scan(&s, &e); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			sp, _ := time.ParseInLocation("15:04", s, loc)
			ep, _ := time.ParseInLocation("15:04", e, loc)
			availStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), sp.Hour(), sp.Minute(), 0, 0, loc)
			availEnd := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), ep.Hour(), ep.Minute(), 0, 0, loc)

			if !startLocal.Before(availStart) && !startLocal.Add(30*time.Minute).After(availEnd) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requested slot not within coach availability for that local date"})
			return
		}

		// Insert booking inside a transaction; unique index on (coach_id, start_time) prevents races
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer tx.Rollback()

		insertQ := `INSERT INTO bookings (user_id, coach_id, start_time, end_time, created_at) VALUES (?, ?, ?, ?, UTC_TIMESTAMP())`
		if _, err := tx.Exec(insertQ, r.UserID, r.CoachID, start, end); err != nil {
			var me *mysql.MySQLError
			if errors.As(err, &me) && me.Number == 1062 {
				c.JSON(http.StatusConflict, gin.H{"error": "slot already booked"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"status": "booked", "start": start.Format(time.RFC3339)})
	}
}

// getUserBookingsHandler - GET /users/bookings?user_id=...
func GetUserBookingsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Query("user_id")
		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
			return
		}
		q := `SELECT id, coach_id, start_time, end_time, created_at FROM bookings WHERE user_id = ? AND start_time >= ? ORDER BY start_time ASC`
		rows, err := db.Query(q, userID, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type br struct {
			ID      int    `json:"id"`
			CoachID int    `json:"coach_id"`
			Start   string `json:"start"`
			End     string `json:"end"`
			Created string `json:"created_at"`
		}
		out := []br{}
		for rows.Next() {
			var id, coach int
			var start, end, created time.Time
			if err := rows.Scan(&id, &coach, &start, &end, &created); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = append(out, br{
				ID:      id,
				CoachID: coach,
				Start:   start.UTC().Format(time.RFC3339),
				End:     end.UTC().Format(time.RFC3339),
				Created: created.UTC().Format(time.RFC3339),
			})
		}
		c.JSON(http.StatusOK, gin.H{"bookings": out})
	}
}

// cancelBookingHandler - DELETE /users/bookings/:id
func CancelBookingHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
			return
		}
		res, err := db.Exec(`DELETE FROM bookings WHERE id = ?`, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
	}
}

// helper: ensure time aligns to 30-minute boundary
func IsOnThirtyMinuteBoundary(t time.Time) bool {
	return t.Minute()%30 == 0 && t.Second() == 0 && t.Nanosecond() == 0
}
