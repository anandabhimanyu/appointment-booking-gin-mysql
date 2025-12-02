package tests

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http/httptest"
    "os"
    "testing"
    "time"

    "appointment-booking-gin-mysql/internal/api"
    "appointment-booking-gin-mysql/internal/db"

    "github.com/gin-gonic/gin"
)

func TestIntegrationBasicFlow(t *testing.T) {
    dsn := os.Getenv("TEST_MYSQL_DSN")
    if dsn == "" {
        t.Skip("skip integration tests unless TEST_MYSQL_DSN is set")
    }
    sqlDB, err := db.Open("mysql", dsn)
    if err != nil {
        t.Fatalf("db.Open: %v", err)
    }
    defer sqlDB.Close()

    g := gin.Default()
    api.RegisterRoutes(g, sqlDB)

    // 1) create coach
    coachBody := map[string]string{"name": "Test Coach", "timezone": "Asia/Kolkata"}
    b, _ := json.Marshal(coachBody)
    req := httptest.NewRequest("POST", "/coaches", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    g.ServeHTTP(w, req)
    if w.Code != 201 {
        t.Fatalf("expected 201 got %d body=%s", w.Code, w.Body.String())
    }
    var coachResp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &coachResp)
    coachID := int(coachResp["id"].(float64))

    // 2) create user directly in DB
    res, err := sqlDB.Exec("INSERT INTO users (name, created_at) VALUES (?, UTC_TIMESTAMP())", "Test User")
    if err != nil {
        t.Fatalf("insert user: %v", err)
    }
    uid64, _ := res.LastInsertId()
    userID := int(uid64)

    // 3) add availability (Tuesday 09:00-12:00)
    av := map[string]interface{}{"coach_id": coachID, "day": 2, "start_time": "09:00", "end_time": "12:00"}
    ab, _ := json.Marshal(av)
    req = httptest.NewRequest("POST", "/coaches/availability", bytes.NewReader(ab))
    req.Header.Set("Content-Type", "application/json")
    w = httptest.NewRecorder()
    g.ServeHTTP(w, req)
    if w.Code != 201 {
        t.Fatalf("expected 201 availability got %d %s", w.Code, w.Body.String())
    }

    // 4) find next Tuesday in coach timezone (Asia/Kolkata)
    loc, _ := time.LoadLocation("Asia/Kolkata")
    now := time.Now().In(loc)
    daysAhead := (2 - int(now.Weekday()) + 7) % 7
    if daysAhead == 0 {
        daysAhead = 7
    }
    target := now.AddDate(0, 0, daysAhead)
    dateStr := target.Format("2006-01-02")

    // 5) get slots
    req = httptest.NewRequest("GET", "/users/slots?coach_id="+fmt.Sprintf("%d", coachID)+"&date="+dateStr, nil)
    w = httptest.NewRecorder()
    g.ServeHTTP(w, req)
    if w.Code != 200 {
        t.Fatalf("expected 200 for slots got %d %s", w.Code, w.Body.String())
    }
    var slotsResp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &slotsResp)
    slots := slotsResp["slots"].([]interface{})
    if len(slots) == 0 {
        t.Fatalf("expected >0 slots")
    }

    // 6) book first slot
    first := slots[0].(string)
    book := map[string]interface{}{"user_id": userID, "coach_id": coachID, "datetime": first}
    bb, _ := json.Marshal(book)
    req = httptest.NewRequest("POST", "/users/bookings", bytes.NewReader(bb))
    req.Header.Set("Content-Type", "application/json")
    w = httptest.NewRecorder()
    g.ServeHTTP(w, req)
    if w.Code != 201 {
        t.Fatalf("expected 201 for booking got %d %s", w.Code, w.Body.String())
    }

    // 7) booking same slot should 409
    req = httptest.NewRequest("POST", "/users/bookings", bytes.NewReader(bb))
    req.Header.Set("Content-Type", "application/json")
    w = httptest.NewRecorder()
    g.ServeHTTP(w, req)
    if w.Code != 409 {
        t.Fatalf("expected 409 for double booking got %d %s", w.Code, w.Body.String())
    }

    // 8) get user bookings
    req = httptest.NewRequest("GET", "/users/bookings?user_id="+fmt.Sprintf("%d", userID), nil)
    w = httptest.NewRecorder()
    g.ServeHTTP(w, req)
    if w.Code != 200 {
        t.Fatalf("expected 200 for user bookings got %d %s", w.Code, w.Body.String())
    }
}

func TestMain(m *testing.M) {
    os.Exit(m.Run())
}
