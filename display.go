package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type displayBlock struct {
	Type          string   `json:"type"`
	X             int      `json:"x"`
	Y             int      `json:"y"`
	W             int      `json:"w"`
	H             int      `json:"h"`
	ID            string   `json:"id,omitempty"`
	RoomID        int64    `json:"roomId,omitempty"`
	Text          string   `json:"text,omitempty"`
	Color         string   `json:"color,omitempty"`
	BgColor       string   `json:"bgColor,omitempty"`
	TextColor     string   `json:"textColor,omitempty"`
	AccentColor   string   `json:"accentColor,omitempty"`
	FontSize      int      `json:"fontSize,omitempty"`
	FontScale     int      `json:"fontScale,omitempty"`
	LetterSpacing int      `json:"letterSpacing,omitempty"`
	Padding       int      `json:"padding,omitempty"`
	Margin        int      `json:"margin,omitempty"`
	Align         string   `json:"align,omitempty"`
	VAlign        string   `json:"valign,omitempty"`
	SubLayout     string   `json:"subLayout,omitempty"`
	HideParts     []string `json:"hideParts,omitempty"`
}
type displayConfig struct {
	Organization string         `json:"organization"`
	Logo         string         `json:"logo"`
	Blocks       []displayBlock `json:"blocks"`
	Width        int            `json:"width,omitempty"`
	Height       int            `json:"height,omitempty"`
	Rows         int            `json:"rows,omitempty"`
}

func defaultDisplayConfig() displayConfig {
	return displayConfig{Blocks: []displayBlock{{Type: "clock", X: 0, Y: 0, W: 3, H: 1}, {Type: "branding", X: 3, Y: 0, W: 9, H: 1}, {Type: "room", ID: "room", X: 0, Y: 1, W: 12, H: 1}, {Type: "status", ID: "status", X: 0, Y: 2, W: 7, H: 4}, {Type: "upcoming", ID: "upcoming", X: 7, Y: 2, W: 5, H: 4}, {Type: "footer", X: 0, Y: 6, W: 12, H: 1}}}
}

var layerID = regexp.MustCompile(`^layer_[a-zA-Z0-9_-]{1,58}$`)
var widgetID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var layerColor = regexp.MustCompile(`^#[a-fA-F0-9]{6}$`)

var widgetHideParts = map[string]map[string]bool{
	"clock":    {"zone": true, "date": true},
	"branding": {"eyebrow": true, "logo": true},
	"room":     {"eyebrow": true, "accentBar": true},
	"status":   {"pill": true, "icon": true, "until": true, "remaining": true, "progress": true},
	"upcoming": {"count": true, "relative": true, "duration": true},
	"footer":   {"badge": true, "zone": true},
}

func validHexOpt(value string) bool {
	return value == "" || layerColor.MatchString(value)
}

func validHideParts(widgetType string, parts []string) bool {
	if len(parts) > 8 {
		return false
	}
	allowed := widgetHideParts[widgetType]
	seen := map[string]bool{}
	for _, part := range parts {
		if !allowed[part] || seen[part] {
			return false
		}
		seen[part] = true
	}
	return true
}

func validDisplayConfig(c displayConfig) bool {
	if len(c.Organization) > 128 || len(c.Logo) > 256 || c.Blocks == nil || len(c.Blocks) > 32 {
		return false
	}
	if (c.Width == 0) != (c.Height == 0) || c.Width < 0 || c.Height < 0 || c.Width > 8192 || c.Height > 8192 || c.Rows < 0 || c.Rows > 24 || (c.Width > 0 && c.Rows < 7) || (c.Width == 0 && c.Rows != 0) {
		return false
	}
	if c.Logo != "" {
		u, err := url.Parse(c.Logo)
		if err != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(c.Logo, "/") || strings.HasPrefix(c.Logo, "//") || strings.ContainsAny(c.Logo, "\\%") {
			return false
		}
	}
	seen := map[string]bool{}
	for _, a := range c.Blocks {
		key := a.Type
		switch a.Type {
		case "clock", "branding", "room", "status", "upcoming", "footer":
			roomWidget := a.Type == "room" || a.Type == "status" || a.Type == "upcoming"
			if roomWidget {
				if !widgetID.MatchString(a.ID) || a.RoomID < 0 || a.RoomID > 9007199254740991 {
					return false
				}
				key = a.ID
			} else if a.ID != "" || a.RoomID != 0 {
				return false
			}
			if a.Text != "" || a.Color != "" || a.FontSize != 0 || a.Margin != 0 ||
				!validHexOpt(a.BgColor) || !validHexOpt(a.TextColor) || !validHexOpt(a.AccentColor) ||
				a.FontScale < 0 || a.FontScale > 180 || (a.FontScale > 0 && a.FontScale < 60) ||
				a.LetterSpacing < -4 || a.LetterSpacing > 20 ||
				a.Padding < 0 || a.Padding > 128 ||
				(a.Align != "" && a.Align != "left" && a.Align != "center" && a.Align != "right") ||
				(a.VAlign != "" && a.VAlign != "top" && a.VAlign != "center" && a.VAlign != "bottom") ||
				(a.SubLayout != "" && a.SubLayout != "default" && a.SubLayout != "reverse" && a.SubLayout != "compact") ||
				!validHideParts(a.Type, a.HideParts) {
				return false
			}
		case "text":
			if a.RoomID != 0 || !layerID.MatchString(a.ID) || len(a.Text) > 512 || a.Color != "" ||
				!validHexOpt(a.BgColor) || !validHexOpt(a.TextColor) || a.AccentColor != "" ||
				a.FontScale != 0 || a.SubLayout != "" || len(a.HideParts) > 0 ||
				a.LetterSpacing < -4 || a.LetterSpacing > 20 ||
				a.FontSize < 0 || a.FontSize > 256 || (a.FontSize > 0 && a.FontSize < 8) ||
				a.Padding < 0 || a.Padding > 128 || a.Margin < 0 || a.Margin > 128 ||
				(a.Align != "" && a.Align != "left" && a.Align != "center" && a.Align != "right") ||
				(a.VAlign != "" && a.VAlign != "top" && a.VAlign != "center" && a.VAlign != "bottom") {
				return false
			}
			key = a.ID
		case "rectangle":
			if a.RoomID != 0 || !layerID.MatchString(a.ID) || !layerColor.MatchString(a.Color) || a.Text != "" ||
				a.BgColor != "" || a.TextColor != "" || a.AccentColor != "" ||
				a.FontSize != 0 || a.FontScale != 0 || a.LetterSpacing != 0 ||
				a.Padding != 0 || a.Margin != 0 || a.Align != "" || a.VAlign != "" ||
				a.SubLayout != "" || len(a.HideParts) > 0 {
				return false
			}
			key = a.ID
		default:
			return false
		}
		if seen[key] || a.X < 0 || a.Y < 0 || a.W < 1 || a.H < 1 || a.X > 11 || a.Y > 23 || a.W > 12-a.X || a.H > 24-a.Y || (c.Rows > 0 && a.H > c.Rows-a.Y) {
			return false
		}
		seen[key] = true
	}
	return true
}

func registerDisplayHandlers(mux *http.ServeMux, db *sql.DB, post func(http.HandlerFunc) http.HandlerFunc, session sessionCheck) {
	idOf := func(r *http.Request) (int64, error) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if id <= 0 {
			return 0, errors.New("invalid id")
		}
		return id, err
	}
	mux.HandleFunc("GET /api/devices/{id}/display", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if _, _, _, ok := session(w, r); !ok {
			return
		}
		id, err := idOf(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var raw string
		err = db.QueryRow("SELECT display_config FROM devices WHERE id=? AND revoked=0", id).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		c, err := readDisplayConfig(raw)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c)
	})
	mux.HandleFunc("POST /api/devices/{id}/display", post(func(w http.ResponseWriter, r *http.Request) {
		token, _, csrf, ok := session(w, r)
		if !ok {
			return
		}
		if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		id, err := idOf(r)
		var c displayConfig
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if err != nil || d.Decode(&c) != nil || d.Decode(&struct{}{}) != io.EOF || !validDisplayConfig(c) {
			http.Error(w, "invalid request", 400)
			return
		}
		raw, err := json.Marshal(c)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec("UPDATE users SET active=active WHERE 0"); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		var actor int64
		err = tx.QueryRow("SELECT users.id FROM sessions JOIN users ON users.id=sessions.user_id WHERE sessions.token_hash=? AND sessions.csrf=? AND sessions.expires>? AND users.active=1", tokenHash(token), csrf, time.Now().Unix()).Scan(&actor)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "unauthorized", 401)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		var previousRaw string
		err = tx.QueryRow("SELECT display_config FROM devices WHERE id=? AND revoked=0", id).Scan(&previousRaw)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		previous, err := readDisplayConfig(previousRaw)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		oldRooms := make(map[string]displayBlock)
		for _, block := range previous.Blocks {
			if block.RoomID > 0 {
				oldRooms[block.ID] = block
			}
		}
		for _, block := range c.Blocks {
			old := oldRooms[block.ID]
			if block.RoomID == 0 || old.RoomID == block.RoomID && old.Type == block.Type {
				continue
			}
			var active bool
			err = tx.QueryRow("SELECT active FROM rooms WHERE id=?", block.RoomID).Scan(&active)
			if errors.Is(err, sql.ErrNoRows) || err == nil && !active {
				http.Error(w, "invalid room", 400)
				return
			}
			if err != nil {
				http.Error(w, "internal error", 500)
				return
			}
		}
		result, err := tx.Exec("UPDATE devices SET display_config=? WHERE id=? AND revoked=0", string(raw), id)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		n, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		if n == 0 {
			http.NotFound(w, r)
			return
		}
		if tx.Commit() != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/display/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		value := r.PathValue("id")
		if len(value) != 32 || strings.Trim(value, "0123456789abcdef") != "" {
			http.NotFound(w, r)
			return
		}
		tx, err := db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		defer tx.Rollback()
		var raw, language string
		err = tx.QueryRow("SELECT display_config,language FROM devices WHERE public_id=? AND revoked=0", value).Scan(&raw, &language)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		c, err := readDisplayConfig(raw)
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		now := time.Now().Unix()
		rooms := make(map[string]publicRoom)
		for _, block := range c.Blocks {
			if block.RoomID <= 0 {
				continue
			}
			key := strconv.FormatInt(block.RoomID, 10)
			if _, ok := rooms[key]; ok {
				continue
			}
			rooms[key] = loadPublicRoom(tx, block.RoomID, now)
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Config   displayConfig         `json:"config"`
			Language string                `json:"language"`
			Now      int64                 `json:"now"`
			Rooms    map[string]publicRoom `json:"rooms"`
		}{c, language, now, rooms})
	})
}
