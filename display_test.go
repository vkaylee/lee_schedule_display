package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestDisplayLayerValidation(t *testing.T) {
	c := defaultDisplayConfig()
	c.Blocks = append(c.Blocks, displayBlock{Type: "rectangle", ID: "layer_box", X: 0, Y: 0, W: 4, H: 2, Color: "#ffffff"}, displayBlock{Type: "text", ID: "layer_text", X: 0, Y: 0, W: 4, H: 1, Text: "Hello"})
	c.Blocks[len(c.Blocks)-1].FontSize = 48
	c.Blocks[len(c.Blocks)-1].Padding = 12
	c.Blocks[len(c.Blocks)-1].Margin = 8
	c.Blocks[len(c.Blocks)-1].Align = "center"
	c.Blocks[len(c.Blocks)-1].VAlign = "bottom"
	if !validDisplayConfig(c) {
		t.Fatal("overlapping layers rejected")
	}
	for _, invalid := range []displayBlock{{FontSize: 7}, {FontSize: 257}, {Padding: -1}, {Padding: 129}, {Margin: -1}, {Margin: 129}, {Align: "justify"}, {VAlign: "baseline"}} {
		candidate := c
		candidate.Blocks = append([]displayBlock(nil), c.Blocks...)
		candidate.Blocks[len(candidate.Blocks)-1].FontSize = invalid.FontSize
		candidate.Blocks[len(candidate.Blocks)-1].Padding = invalid.Padding
		candidate.Blocks[len(candidate.Blocks)-1].Margin = invalid.Margin
		candidate.Blocks[len(candidate.Blocks)-1].Align = invalid.Align
		candidate.Blocks[len(candidate.Blocks)-1].VAlign = invalid.VAlign
		if validDisplayConfig(candidate) {
			t.Fatalf("invalid text style accepted: %+v", invalid)
		}
	}
	c.Blocks[len(c.Blocks)-1].ID = "layer_box"
	if validDisplayConfig(c) {
		t.Fatal("duplicate ID accepted")
	}
	c.Blocks[len(c.Blocks)-1].ID = "layer_text"
	c.Blocks[len(c.Blocks)-1].Text = strings.Repeat("a", 513)
	if validDisplayConfig(c) {
		t.Fatal("oversized text accepted")
	}
}

func TestDisplayWidgetStyleValidation(t *testing.T) {
	c := defaultDisplayConfig()
	c.Blocks[0].BgColor = "#112233"
	c.Blocks[0].TextColor = "#ffffff"
	c.Blocks[0].AccentColor = "#22cc88"
	c.Blocks[0].FontScale = 120
	c.Blocks[0].LetterSpacing = 4
	c.Blocks[0].Padding = 20
	c.Blocks[0].Align = "center"
	c.Blocks[0].VAlign = "top"
	c.Blocks[0].SubLayout = "reverse"
	c.Blocks[0].HideParts = []string{"zone"}
	if !validDisplayConfig(c) {
		t.Fatal("valid widget style rejected")
	}
	for _, invalid := range []displayBlock{
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, BgColor: "red"},
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, FontScale: 59},
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, FontScale: 181},
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, LetterSpacing: -5},
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, LetterSpacing: 21},
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, SubLayout: "floating"},
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, HideParts: []string{"progress"}},
		{Type: "clock", X: 0, Y: 0, W: 3, H: 1, HideParts: []string{"zone", "zone"}},
	} {
		candidate := displayConfig{Blocks: []displayBlock{invalid}}
		if validDisplayConfig(candidate) {
			t.Fatalf("invalid widget style accepted: %+v", invalid)
		}
	}
}

func TestDisplayResolutionValidation(t *testing.T) {
	c := defaultDisplayConfig()
	if !validDisplayConfig(c) {
		t.Fatal("legacy configuration rejected")
	}
	c.Width, c.Height, c.Rows = 1920, 1080, 7
	if !validDisplayConfig(c) {
		t.Fatal("valid resolution rejected")
	}
	for _, invalid := range []displayConfig{
		{Blocks: c.Blocks, Width: 1920, Rows: 7},
		{Blocks: c.Blocks, Width: 8193, Height: 1080, Rows: 7},
		{Blocks: c.Blocks, Width: 1920, Height: 1080, Rows: 6},
		{Blocks: []displayBlock{{Type: "room", X: 0, Y: 7, W: 1, H: 1}}, Width: 1920, Height: 1080, Rows: 7},
	} {
		if validDisplayConfig(invalid) {
			t.Fatalf("invalid configuration accepted: %+v", invalid)
		}
	}
}

func TestDisplayPublicPrivacyAndConfiguration(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	token := strings.Repeat("a", 43)
	if _, err := db.Exec(`INSERT INTO users(username,salt,password_hash) VALUES('admin',X'01',X'02'); INSERT INTO rooms(name,show_title,show_organizer) VALUES('Room',1,1); INSERT INTO devices(room_id,name,token_hash) VALUES(1,'panel',X'01');`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions(token_hash,user_id,csrf,expires) VALUES(?,1,'csrf',?)`, tokenHash(token), now+3600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO bookings(room_id,title,organizer,private,starts,ends) VALUES(1,'private title','private organizer',0,?,?)`, now+300, now+600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO booking_series(room_id,title,organizer,private,rule,actor_id) VALUES(1,'series secret','series organizer',1,'{}',1); INSERT INTO series_occurrences(series_id,original_starts,starts,ends) VALUES(1,?,?,?)`, now+700, now+700, now+800); err != nil {
		t.Fatal(err)
	}
	publicID := strings.Repeat("a", 32)
	if _, err := db.Exec("UPDATE devices SET public_id=? WHERE id=1", publicID); err != nil {
		t.Fatal(err)
	}
	h := handlerWithAuth(db, "https://example.test", netip.Addr{})
	request := func(method, path, body string, admin bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.test"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://example.test")
		if admin {
			r.AddCookie(&http.Cookie{Name: "__Host-session", Value: token})
			r.Header.Set("X-CSRF-Token", "csrf")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/api/display/"+publicID, "", false)
	if w.Code != 200 || strings.Contains(w.Body.String(), "private title") || strings.Contains(w.Body.String(), "private organizer") || strings.Contains(w.Body.String(), "series secret") || strings.Contains(w.Body.String(), "series organizer") || !strings.Contains(w.Body.String(), fmt.Sprintf(`"starts":%d`, now+700)) || !strings.Contains(w.Body.String(), `"room":"Room"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Header())
	}
	config := defaultDisplayConfig()
	config.Blocks[2].RoomID = 1
	config.Blocks[3].RoomID = 1
	config.Blocks[4].RoomID = 1
	config.Organization = "Example"
	config.Width, config.Height, config.Rows = 1920, 1080, 7
	config.Blocks = append(config.Blocks, displayBlock{Type: "rectangle", ID: "layer_box", X: 0, Y: 0, W: 4, H: 2, Color: "#123456"}, displayBlock{Type: "text", ID: "layer_text", X: 0, Y: 0, W: 4, H: 1, Text: "<b>hello</b>"})
	raw, _ := json.Marshal(config)
	if w = request("POST", "/api/devices/1/display", string(raw), false); w.Code != 401 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("POST", "/api/devices/1/display", string(raw), true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request("GET", "/api/display/"+publicID, "", false); w.Code != 200 || !strings.Contains(w.Body.String(), "Example") || !strings.Contains(w.Body.String(), `"width":1920`) || !strings.Contains(w.Body.String(), `"height":1080`) || !strings.Contains(w.Body.String(), `"rows":7`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("GET", "/api/devices/1/display", "", true)
	var loaded displayConfig
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &loaded) != nil || len(loaded.Blocks) != len(config.Blocks) || loaded.Blocks[len(loaded.Blocks)-1].Text != "<b>hello</b>" || loaded.Blocks[len(loaded.Blocks)-2].Color != "#123456" {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.Exec(`INSERT INTO devices(room_id,name,token_hash) VALUES(1,'second',X'02')`); err != nil {
		t.Fatal(err)
	}
	if w = request("GET", "/api/devices/2/display", "", true); w.Code != 200 || strings.Contains(w.Body.String(), `"width"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.Exec("UPDATE rooms SET active=0 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	config.Organization = "Changed"
	raw, _ = json.Marshal(config)
	if w = request("POST", "/api/devices/1/display", string(raw), true); w.Code != 204 {
		t.Fatalf("unchanged inactive room: %d %s", w.Code, w.Body.String())
	}
	config.Blocks = append(config.Blocks, displayBlock{Type: "room", ID: "new-room", RoomID: 1, W: 1, H: 1})
	raw, _ = json.Marshal(config)
	if w = request("POST", "/api/devices/1/display", string(raw), true); w.Code != 400 {
		t.Fatalf("new inactive room: %d %s", w.Code, w.Body.String())
	}
	config.Blocks = config.Blocks[:len(config.Blocks)-1]
	config.Blocks[1].X = 12
	raw, _ = json.Marshal(config)
	if w = request("POST", "/api/devices/1/display", string(raw), true); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := db.Exec("UPDATE devices SET revoked=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w = request("GET", "/api/display/"+publicID, "", false); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
