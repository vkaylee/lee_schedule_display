package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupSeriesScopeTest(t *testing.T, loc *time.Location) (*sql.DB, http.Handler, string, string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "scope.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO rooms(name) VALUES('Room A'),('Room B'); INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02')"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("z", 43)
	if _, err = db.Exec("INSERT INTO sessions VALUES (?,1,'csrf',?)", tokenHash(token), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	h := handlerWithTimezone(db, "https://schedule.example", netip.Addr{}, loc)
	return db, h, token, "csrf"
}

func doSeriesReq(t *testing.T, h http.Handler, method, path, token, csrf string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Origin", "https://schedule.example")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSeriesMigrationVersion16(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 16 {
		t.Fatalf("version=%d err=%v", version, err)
	}
	for _, stmt := range []string{
		"DROP TABLE series_audit",
		"ALTER TABLE booking_series DROP COLUMN version",
		"ALTER TABLE booking_series DROP COLUMN canceled",
	} {
		t.Run(stmt, func(t *testing.T) {
			d2, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer d2.Close()
			d2.SetMaxOpenConns(1)
			if err = migrate(d2); err != nil {
				t.Fatal(err)
			}
			if _, err = d2.Exec(stmt); err != nil {
				t.Fatal(err)
			}
			if err = migrate(d2); err == nil {
				t.Fatal("expected fail-closed migration error")
			}
		})
	}
}

func TestCanceledSeriesExcludedEverywhere(t *testing.T) {
	db, h, token, csrf := setupSeriesScopeTest(t, time.UTC)
	deviceToken := strings.Repeat("d", 43)
	pubID := strings.Repeat("a", 32)
	if _, err := db.Exec("INSERT INTO devices(room_id,name,token_hash,public_id) VALUES(1,'kiosk',?,?)", tokenHash(deviceToken), pubID); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Second).Add(2 * time.Hour)
	// Unbounded series on Room 1
	rec := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 1, "title": "Standup", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1,
	}, nil)
	if rec.Code != 201 {
		t.Fatalf("create unbounded: %d %s", rec.Code, rec.Body.String())
	}
	// Another series overlapping on Room 1 outside 12m window won't stop unbounded? Let's create a second series overlapping first day by direct DB or single? Series 2 at year+2 or overlap with booking. Let's create conflicting series after inserting occurrences, then run scanConflicts.
	if _, err := db.Exec(`INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id,checked_through) VALUES(1,'Other','admin',0,'{"rule":{"Count":1,"Until":"0001-01-01T00:00:00Z"}}',1,1); INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(2,?,?,?)`, start.Unix(), start.Unix(), start.Unix()+1800); err != nil {
		t.Fatal(err)
	}
	if err := runConflictScan(db, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var activeAlerts int
	if err := db.QueryRow("SELECT COUNT(*) FROM conflict_alerts WHERE active=1").Scan(&activeAlerts); err != nil || activeAlerts != 1 {
		t.Fatalf("expected 1 active alert, got %d %v", activeAlerts, err)
	}
	// Cancel series 1 via HTTP
	cancelRec := doSeriesReq(t, h, "POST", "/api/series/1/cancel", token, csrf, map[string]any{"scope": "all"}, map[string]string{"If-Match": "1"})
	if cancelRec.Code != 204 {
		t.Fatalf("cancel all: %d %s", cancelRec.Code, cancelRec.Body.String())
	}
	// Canceling again returns 409
	if again := doSeriesReq(t, h, "POST", "/api/series/1/cancel", token, csrf, map[string]any{"scope": "all"}, map[string]string{"If-Match": "2"}); again.Code != 409 {
		t.Fatalf("expected 409 on re-cancel, got %d", again.Code)
	}
	// Mutating occurrence of canceled series returns 409
	occRec := doSeriesReq(t, h, "POST", "/api/series/1/occurrences/"+jsonNum(start.Unix())+"/cancel", token, csrf, nil, map[string]string{"If-Match": "2"})
	if occRec.Code != 409 {
		t.Fatalf("expected 409 mutating canceled series occurrence, got %d", occRec.Code)
	}
	// Single booking at start should no longer conflict with canceled series 1 (cancel series 2 too first)
	if _, err := db.Exec("UPDATE booking_series SET canceled=1 WHERE id=2; UPDATE series_occurrences SET canceled=1 WHERE series_id=2"); err != nil {
		t.Fatal(err)
	}
	single := doSeriesReq(t, h, "POST", "/api/bookings", token, csrf, map[string]any{
		"room_id": 1, "title": "Single", "organizer": "admin", "private": false, "starts": start.Unix(), "ends": start.Unix() + 1800,
	}, nil)
	if single.Code != 201 {
		t.Fatalf("single after series cancel: %d %s", single.Code, single.Body.String())
	}
	// Remove single booking, check kiosk and public display have empty bookings and nil valid_until
	if _, err := db.Exec("DELETE FROM bookings"); err != nil {
		t.Fatal(err)
	}
	kioskReq := httptest.NewRequest("GET", "/api/kiosk", nil)
	kioskReq.Header.Set("Authorization", "Bearer "+deviceToken)
	kioskRec := httptest.NewRecorder()
	h.ServeHTTP(kioskRec, kioskReq)
	if kioskRec.Code != 200 {
		t.Fatalf("kiosk: %d", kioskRec.Code)
	}
	var kioskPayload struct {
		Bookings   []any  `json:"bookings"`
		ValidUntil *int64 `json:"valid_until"`
	}
	if err := json.Unmarshal(kioskRec.Body.Bytes(), &kioskPayload); err != nil || len(kioskPayload.Bookings) != 0 || kioskPayload.ValidUntil != nil {
		t.Fatalf("unexpected kiosk payload after cancel: %s (%v)", kioskRec.Body.String(), err)
	}
	dispRec := httptest.NewRecorder()
	h.ServeHTTP(dispRec, httptest.NewRequest("GET", "/api/display/"+pubID, nil))
	if dispRec.Code != 200 {
		t.Fatalf("display: %d", dispRec.Code)
	}
	var dispPayload struct {
		Bookings   []any  `json:"bookings"`
		ValidUntil *int64 `json:"valid_until"`
	}
	if err := json.Unmarshal(dispRec.Body.Bytes(), &dispPayload); err != nil || len(dispPayload.Bookings) != 0 || dispPayload.ValidUntil != nil {
		t.Fatalf("unexpected display payload after cancel: %s (%v)", dispRec.Body.String(), err)
	}
	// Re-run conflict scan: alerts deactivated and no materialization coverage remains
	if err := runConflictScan(db, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM conflict_alerts WHERE active=1").Scan(&activeAlerts); err != nil || activeAlerts != 0 {
		t.Fatalf("expected 0 active alerts, got %d", activeAlerts)
	}
	var covCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM materialization_coverage").Scan(&covCount); err != nil || covCount != 0 {
		t.Fatalf("expected 0 coverage rows, got %d", covCount)
	}
}

func jsonNum(v int64) string { return string(bytes.TrimSpace(mustMarshal(v))) }
func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestSeriesUpdateAllPreservesAndRejectsExceptions(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	db, h, token, csrf := setupSeriesScopeTest(t, loc)
	start := time.Date(2028, 1, 10, 9, 0, 0, 0, loc)
	rec := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 1, "title": "Weekly", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 3600, "frequency": 1, "interval": 1, "count": 4,
	}, nil)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	// Make day 2 an exception (title override + time shift by +2h) and day 3 canceled
	day2 := start.AddDate(0, 0, 1)
	day3 := start.AddDate(0, 0, 2)
	if r := doSeriesReq(t, h, "POST", "/api/series/1/occurrences/"+jsonNum(day2.Unix()), token, csrf, map[string]any{
		"room_id": 1, "title": "Special Day 2", "organizer": "lead", "private": true,
		"starts": day2.Add(2 * time.Hour).Unix(), "ends": day2.Add(3 * time.Hour).Unix(), "version": 1,
	}, nil); r.Code != 204 {
		t.Fatalf("exc day2: %d %s", r.Code, r.Body.String())
	}
	if r := doSeriesReq(t, h, "POST", "/api/series/1/occurrences/"+jsonNum(day3.Unix())+"/cancel", token, csrf, nil, map[string]string{"If-Match": "1"}); r.Code != 204 {
		t.Fatalf("cancel day3: %d %s", r.Code, r.Body.String())
	}
	// Reject update all if count=2 would drop day3 canceled exception's original_starts
	dropExc := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "all", "version": 1, "room_id": 1, "title": "Shortened", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 2,
	}, nil)
	if dropExc.Code != 409 {
		t.Fatalf("expected 409 dropping exception, got %d %s", dropExc.Code, dropExc.Body.String())
	}
	// Reject update all if start time-of-day shifts so original_starts of day2 disappears
	shiftHour := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "all", "version": 1, "room_id": 1, "title": "Shifted", "organizer": "admin", "private": false,
		"starts": start.Add(time.Hour).Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 4,
	}, nil)
	if shiftHour.Code != 409 {
		t.Fatalf("expected 409 shifting original_starts, got %d %s", shiftHour.Code, shiftHour.Body.String())
	}
	// Reject DST gap (2028-03-12 02:30 America/New_York)
	dst := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "all", "version": 1, "room_id": 1, "title": "DST", "organizer": "admin", "private": false,
		"starts": time.Date(2028, 3, 10, 2, 30, 0, 0, loc).Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 4,
	}, nil)
	if dst.Code != 400 && dst.Code != 409 {
		t.Fatalf("expected rejection on DST/exception, got %d %s", dst.Code, dst.Body.String())
	}
	// Single booking blocking day 4 at new duration (2h -> overlaps single booking placed at +1.5h)
	day4 := start.AddDate(0, 0, 3)
	if _, err = db.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'blocker','admin',0,?,?)", day4.Add(90*time.Minute).Unix(), day4.Add(150*time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	conflictAll := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "all", "version": 1, "room_id": 1, "title": "Updated All", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 7200, "frequency": 1, "interval": 1, "count": 4,
	}, nil)
	if conflictAll.Code != 409 {
		t.Fatalf("expected 409 conflict on day 4, got %d %s", conflictAll.Code, conflictAll.Body.String())
	}
	// Verify zero partial write
	var title string
	var ver int64
	if err = db.QueryRow("SELECT title,version FROM booking_series WHERE id=1").Scan(&title, &ver); err != nil || title != "Weekly" || ver != 1 {
		t.Fatalf("partial write happened: title=%q ver=%d err=%v", title, ver, err)
	}
	if _, err = db.Exec("DELETE FROM bookings"); err != nil {
		t.Fatal(err)
	}
	// Valid update all: keep same start time-of-day and count>=4, move series to Room B with new title & 45m duration
	okAll := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "all", "version": 1, "room_id": 2, "title": "Updated All", "organizer": "admin2", "private": true,
		"starts": start.Unix(), "duration_seconds": 2700, "frequency": 1, "interval": 1, "count": 4,
	}, nil)
	if okAll.Code != 204 {
		t.Fatalf("valid update all failed: %d %s", okAll.Code, okAll.Body.String())
	}
	// Verify exceptions preserved verbatim and non-exceptions updated
	var excTitle string
	var excRoom sql.NullInt64
	var excStarts, excEnds, excVer int64
	var excCanceled bool
	if err = db.QueryRow("SELECT room_id,title,starts,ends,version,canceled FROM series_occurrences WHERE series_id=1 AND original_starts=?", day2.Unix()).Scan(&excRoom, &excTitle, &excStarts, &excEnds, &excVer, &excCanceled); err != nil || !excRoom.Valid || excRoom.Int64 != 1 || excTitle != "Special Day 2" || excStarts != day2.Add(2*time.Hour).Unix() || excEnds != day2.Add(3*time.Hour).Unix() || excVer != 2 || excCanceled {
		t.Fatalf("day2 exception altered: room=%v title=%q starts=%d ver=%d err=%v", excRoom, excTitle, excStarts, excVer, err)
	}
	if err = db.QueryRow("SELECT canceled,version FROM series_occurrences WHERE series_id=1 AND original_starts=?", day3.Unix()).Scan(&excCanceled, &excVer); err != nil || !excCanceled || excVer != 2 {
		t.Fatalf("day3 canceled altered: canceled=%v ver=%d err=%v", excCanceled, excVer, err)
	}
	var d4Ends int64
	if err = db.QueryRow("SELECT ends FROM series_occurrences WHERE series_id=1 AND original_starts=?", day4.Unix()).Scan(&d4Ends); err != nil || d4Ends != day4.Unix()+2700 {
		t.Fatalf("day4 not updated: ends=%d err=%v", d4Ends, err)
	}
}

func TestSeriesSplitFromOccurrence(t *testing.T) {
	db, h, token, csrf := setupSeriesScopeTest(t, time.UTC)
	start := time.Now().UTC().Truncate(time.Second).Add(48 * time.Hour)
	// Create 5 daily occurrences
	if r := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 1, "title": "Series", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 3600, "frequency": 1, "interval": 1, "count": 5,
	}, nil); r.Code != 201 {
		t.Fatalf("create: %d %s", r.Code, r.Body.String())
	}
	day2 := start.AddDate(0, 0, 1)
	day3 := start.AddDate(0, 0, 2)
	day4 := start.AddDate(0, 0, 3)
	day5 := start.AddDate(0, 0, 4)
	// Exception before split (day2) and after split (day4)
	if r := doSeriesReq(t, h, "POST", "/api/series/1/occurrences/"+jsonNum(day2.Unix()), token, csrf, map[string]any{
		"room_id": 1, "title": "Before Exc", "organizer": "admin", "private": false,
		"starts": day2.Unix(), "ends": day2.Unix() + 3600, "version": 1,
	}, nil); r.Code != 204 {
		t.Fatalf("day2 exc: %d", r.Code)
	}
	if r := doSeriesReq(t, h, "POST", "/api/series/1/occurrences/"+jsonNum(day4.Unix()), token, csrf, map[string]any{
		"room_id": 2, "title": "After Exc", "organizer": "admin", "private": true,
		"starts": day4.Add(3 * time.Hour).Unix(), "ends": day4.Add(4 * time.Hour).Unix(), "version": 1,
	}, nil); r.Code != 204 {
		t.Fatalf("day4 exc: %d", r.Code)
	}
	// Reject split from day3 if new rule drops day4 exception's original_starts
	badSplit := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "from", "original_starts": day3.Unix(), "version": 1,
		"room_id": 1, "title": "Bad Split", "organizer": "admin", "private": false,
		"starts": day3.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 1,
	}, nil)
	if badSplit.Code != 409 {
		t.Fatalf("expected 409 losing after exception, got %d %s", badSplit.Code, badSplit.Body.String())
	}
	// Reject split if new series overlaps kept part of old series (e.g., new starts at day2!)
	overlapOld := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "from", "original_starts": day3.Unix(), "version": 1,
		"room_id": 1, "title": "Overlap", "organizer": "admin", "private": false,
		"starts": day2.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 4,
	}, nil)
	if overlapOld.Code != 409 {
		t.Fatalf("expected 409 overlapping old kept prefix, got %d %s", overlapOld.Code, overlapOld.Body.String())
	}
	// Valid split from day3 with count=3 (day3, day4, day5)
	okSplit := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "from", "original_starts": day3.Unix(), "version": 1,
		"room_id": 1, "title": "New Tail", "organizer": "lead", "private": true,
		"starts": day3.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 3,
	}, nil)
	if okSplit.Code != 204 {
		t.Fatalf("valid split failed: %d %s", okSplit.Code, okSplit.Body.String())
	}
	// Verify series 1 has 2 occurrences (start, day2) with Before Exc intact, and Count=2 in rule
	var oldOccs, newOccs int
	if err := db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE series_id=1").Scan(&oldOccs); err != nil || oldOccs != 2 {
		t.Fatalf("old occs=%d err=%v", oldOccs, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE series_id=2").Scan(&newOccs); err != nil || newOccs != 3 {
		t.Fatalf("new occs=%d err=%v", newOccs, err)
	}
	var oldRuleRaw string
	if err := db.QueryRow("SELECT rule FROM booking_series WHERE id=1").Scan(&oldRuleRaw); err != nil || !strings.Contains(oldRuleRaw, `"Count":2`) {
		t.Fatalf("old rule not updated to Count:2: %s %v", oldRuleRaw, err)
	}
	// Verify day4 exception and its occurrence_audit moved to series 2
	var movedTitle string
	var movedAuditSeries int64
	if err := db.QueryRow("SELECT title FROM series_occurrences WHERE series_id=2 AND original_starts=?", day4.Unix()).Scan(&movedTitle); err != nil || movedTitle != "After Exc" {
		t.Fatalf("after exc missing on series 2: %q %v", movedTitle, err)
	}
	if err := db.QueryRow("SELECT series_id FROM occurrence_audit WHERE original_starts=?", day4.Unix()).Scan(&movedAuditSeries); err != nil || movedAuditSeries != 2 {
		t.Fatalf("audit not moved to series 2: %d %v", movedAuditSeries, err)
	}
	// Cancel from last occurrence (day5) on series 2
	cancelLast := doSeriesReq(t, h, "POST", "/api/series/2/cancel", token, csrf, map[string]any{"scope": "from", "original_starts": day5.Unix()}, map[string]string{"If-Match": "1"})
	if cancelLast.Code != 204 {
		t.Fatalf("cancel last: %d %s", cancelLast.Code, cancelLast.Body.String())
	}
	var day5Rows, day5Audit int
	var day3Canceled bool
	if err := db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE series_id=2 AND original_starts>=?", day5.Unix()).Scan(&day5Rows); err != nil || day5Rows != 0 {
		t.Fatalf("day5 tail not removed: %d %v", day5Rows, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM occurrence_audit WHERE series_id=2 AND original_starts=? AND action='cancel'", day5.Unix()).Scan(&day5Audit); err != nil || day5Audit != 1 {
		t.Fatalf("day5 cancel audit missing: %d %v", day5Audit, err)
	}
	if err := db.QueryRow("SELECT canceled FROM series_occurrences WHERE series_id=2 AND original_starts=?", day3.Unix()).Scan(&day3Canceled); err != nil || day3Canceled {
		t.Fatalf("day3 wrongly canceled: %v %v", day3Canceled, err)
	}
	// Split from first occurrence (start) on series 1 updates series 1 in place without creating series 3
	firstSplit := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "from", "original_starts": start.Unix(), "version": 2,
		"room_id": 1, "title": "First In Place", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 2,
	}, nil)
	if firstSplit.Code != 204 {
		t.Fatalf("first split: %d %s", firstSplit.Code, firstSplit.Body.String())
	}
	var seriesCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM booking_series").Scan(&seriesCount); err != nil || seriesCount != 2 {
		t.Fatalf("expected 2 series (in-place update on first occurrence), got %d", seriesCount)
	}
}

func TestSeriesSplitUnboundedAndRollbackOnConflict(t *testing.T) {
	db, h, token, csrf := setupSeriesScopeTest(t, time.UTC)
	start := time.Now().UTC().Truncate(time.Second).Add(24 * time.Hour)
	// Create unbounded daily series
	if r := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 1, "title": "Inf", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1,
	}, nil); r.Code != 201 {
		t.Fatalf("create inf: %d %s", r.Code, r.Body.String())
	}
	day3 := start.AddDate(0, 0, 2)
	// Place single booking on day 10 to trigger conflict when splitting from day3 into a new unbounded series
	day10 := start.AddDate(0, 0, 9)
	if _, err := db.Exec("INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'block','admin',0,?,?)", day10.Unix(), day10.Unix()+1800); err != nil {
		t.Fatal(err)
	}
	conflictSplit := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "from", "original_starts": day3.Unix(), "version": 1,
		"room_id": 1, "title": "Inf Tail", "organizer": "admin", "private": false,
		"starts": day3.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1,
	}, nil)
	if conflictSplit.Code != 409 {
		t.Fatalf("expected 409 conflict on day10, got %d %s", conflictSplit.Code, conflictSplit.Body.String())
	}
	var seriesCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM booking_series").Scan(&seriesCount); err != nil || seriesCount != 1 {
		t.Fatalf("split left partial series: %d", seriesCount)
	}
	// Remove blocker and split into a bounded tail (count=5) -> old series must lose unbounded coverage and become Until before day3
	if _, err := db.Exec("DELETE FROM bookings"); err != nil {
		t.Fatal(err)
	}
	okSplit := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "from", "original_starts": day3.Unix(), "version": 1,
		"room_id": 1, "title": "Finite Tail", "organizer": "admin", "private": false,
		"starts": day3.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 5,
	}, nil)
	if okSplit.Code != 204 {
		t.Fatalf("split inf->finite failed: %d %s", okSplit.Code, okSplit.Body.String())
	}
	var cov int
	if err := db.QueryRow("SELECT COUNT(*) FROM materialization_coverage WHERE series_id=1").Scan(&cov); err != nil || cov != 0 {
		t.Fatalf("expected coverage cleared for truncated old series, got %d", cov)
	}
	// Materialize another year forward: old series 1 must NOT generate occurrences on or after day3!
	if err := materializeUnbounded(db, start, start.AddDate(1, 0, 10)); err != nil {
		t.Fatal(err)
	}
	var oldAfter int
	if err := db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE series_id=1 AND original_starts>=?", day3.Unix()).Scan(&oldAfter); err != nil || oldAfter != 0 {
		t.Fatalf("truncated old series regenerated occurrences after split: %d", oldAfter)
	}
}

func TestCancelFromThenEditSeriesAndUntilTruncation(t *testing.T) {
	db, h, token, csrf := setupSeriesScopeTest(t, time.UTC)
	start := time.Now().UTC().Truncate(time.Second).Add(24 * time.Hour)
	// Create Count=5 series
	if r := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 1, "title": "CountSeries", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 5,
	}, nil); r.Code != 201 {
		t.Fatalf("create count series: %d %s", r.Code, r.Body.String())
	}
	day4 := start.AddDate(0, 0, 3)
	if r := doSeriesReq(t, h, "POST", "/api/series/1/cancel", token, csrf, map[string]any{"scope": "from", "original_starts": day4.Unix()}, map[string]string{"If-Match": "1"}); r.Code != 204 {
		t.Fatalf("cancel from day4: %d %s", r.Code, r.Body.String())
	}
	// Audit preserved, tail rows removed from series_occurrences
	var tailRows, cancelAudits int
	if err := db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE series_id=1 AND original_starts>=?", day4.Unix()).Scan(&tailRows); err != nil || tailRows != 0 {
		t.Fatalf("expected 0 tail rows after cancel_from, got %d %v", tailRows, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM occurrence_audit WHERE series_id=1 AND action='cancel' AND original_starts>=?", day4.Unix()).Scan(&cancelAudits); err != nil || cancelAudits != 2 {
		t.Fatalf("expected 2 cancel audit rows, got %d %v", cancelAudits, err)
	}
	// Editing entire series after cancel_from must succeed (not fail with exception_conflict)
	if r := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "all", "version": 2, "room_id": 1, "title": "Edited After CancelFrom", "organizer": "admin", "private": false,
		"starts": start.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 4,
	}, nil); r.Code != 204 {
		t.Fatalf("edit all after cancel_from failed: %d %s", r.Code, r.Body.String())
	}

	// Create Until-bounded series on Room 2, cancel from day3, verify Until truncated
	uStart := time.Date(2028, 5, 1, 10, 0, 0, 0, time.UTC)
	if r := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 2, "title": "UntilSeries", "organizer": "admin", "private": false,
		"starts": uStart.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "until": "2028-05-05",
	}, nil); r.Code != 201 {
		t.Fatalf("create until series: %d %s", r.Code, r.Body.String())
	}
	uDay3 := uStart.AddDate(0, 0, 2)
	if r := doSeriesReq(t, h, "POST", "/api/series/2/cancel", token, csrf, map[string]any{"scope": "from", "original_starts": uDay3.Unix()}, map[string]string{"If-Match": "1"}); r.Code != 204 {
		t.Fatalf("cancel from until day3: %d %s", r.Code, r.Body.String())
	}
	var untilRule string
	if err := db.QueryRow("SELECT rule FROM booking_series WHERE id=2").Scan(&untilRule); err != nil || !strings.Contains(untilRule, "2028-05-02") {
		t.Fatalf("expected Until truncated to 2028-05-02, got %s %v", untilRule, err)
	}
}

func TestUnboundedPastStartAndFarPrefixSplitConflict(t *testing.T) {
	db, h, token, csrf := setupSeriesScopeTest(t, time.UTC)
	now := time.Now().UTC().Truncate(time.Second)
	pastStart := now.AddDate(0, 0, -3)
	// Insert unbounded series starting 3 days in the past (plus materialized past occurrences)
	if r := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 1, "title": "PastInf", "organizer": "admin", "private": false,
		"starts": pastStart.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1,
	}, nil); r.Code != 201 {
		t.Fatalf("create past inf: %d %s", r.Code, r.Body.String())
	}
	if err := materializeUnbounded(db, pastStart, now.AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	// Update all keeping pastStart: past occurrences [pastStart, now) must NOT be deleted
	if r := doSeriesReq(t, h, "POST", "/api/series/1", token, csrf, map[string]any{
		"scope": "all", "version": 1, "room_id": 1, "title": "PastInf Edited", "organizer": "admin", "private": false,
		"starts": pastStart.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1,
	}, nil); r.Code != 204 {
		t.Fatalf("update past inf: %d %s", r.Code, r.Body.String())
	}
	var pastCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM series_occurrences WHERE series_id=1 AND original_starts<?", now.Unix()).Scan(&pastCount); err != nil || pastCount < 3 {
		t.Fatalf("lost past occurrences on unbounded edit: count=%d err=%v", pastCount, err)
	}

	// Finite series > 12m out: create 10 daily occurrences starting 15 months from now, convert to unbounded with exception at +15m
	farStart := now.AddDate(1, 3, 0)
	if r := doSeriesReq(t, h, "POST", "/api/series", token, csrf, map[string]any{
		"room_id": 2, "title": "FarSeries", "organizer": "admin", "private": false,
		"starts": farStart.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 10,
	}, nil); r.Code != 201 {
		t.Fatalf("create far: %d %s", r.Code, r.Body.String())
	}
	// Make day 2 an exception beyond 12m horizon, then convert series 2 to unbounded (must preserve beyond-horizon exception)
	farDay2 := farStart.AddDate(0, 0, 1)
	if r := doSeriesReq(t, h, "POST", "/api/series/2/occurrences/"+jsonNum(farDay2.Unix()), token, csrf, map[string]any{
		"room_id": 2, "title": "Far Exc", "organizer": "admin", "private": false,
		"starts": farDay2.Unix(), "ends": farDay2.Unix() + 1800, "version": 1,
	}, nil); r.Code != 204 {
		t.Fatalf("far exc: %d", r.Code)
	}
	if r := doSeriesReq(t, h, "POST", "/api/series/2", token, csrf, map[string]any{
		"scope": "all", "version": 1, "room_id": 2, "title": "Far Now Unbounded", "organizer": "admin", "private": false,
		"starts": farStart.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1,
	}, nil); r.Code != 204 {
		t.Fatalf("convert far finite with exc to unbounded failed: %d %s", r.Code, r.Body.String())
	}
	// Split series 2 from day 5 (>12m out), trying to move new tail back onto day 3 of kept prefix (>12m out): must return 409!
	farDay3 := farStart.AddDate(0, 0, 2)
	farDay5 := farStart.AddDate(0, 0, 4)
	if r := doSeriesReq(t, h, "POST", "/api/series/2", token, csrf, map[string]any{
		"scope": "from", "original_starts": farDay5.Unix(), "version": 2,
		"room_id": 2, "title": "Overlap Far Prefix", "organizer": "admin", "private": false,
		"starts": farDay3.Unix(), "duration_seconds": 1800, "frequency": 1, "interval": 1, "count": 5,
	}, nil); r.Code != 409 {
		t.Fatalf("expected 409 overlapping far kept prefix, got %d %s", r.Code, r.Body.String())
	}
}
