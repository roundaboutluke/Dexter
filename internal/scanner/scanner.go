package scanner

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"dexter/internal/circuitbreaker"
	"dexter/internal/config"
)

// Client queries scanner databases for gym/pokestop metadata.
type Client struct {
	scannerType string
	conns       []*sql.DB
	breaker     *circuitbreaker.Breaker
}

// GymNameResolver resolves gym ids to names.
type GymNameResolver interface {
	GetGymName(gymID string) (string, error)
}

// PokestopNameResolver resolves pokestop ids to names.
type PokestopNameResolver interface {
	GetPokestopName(stopID string) (string, error)
}

// StationNameResolver resolves station ids to names.
type StationNameResolver interface {
	GetStationName(stationID string) (string, error)
}

// StopData describes nearby stop or gym details.
type StopData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Type      string  `json:"type"`
	TeamID    int     `json:"teamId,omitempty"`
	Slots     int     `json:"slots,omitempty"`
}

// StationEntry describes a station id and name for autocomplete.
type StationEntry struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
	HasCoords bool
}

// GymEntry describes a gym id and name for autocomplete.
type GymEntry struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
	HasCoords bool
}

// New builds a scanner client based on database.scannerType and database.scanner config.
func New(cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, nil
	}
	scannerType, _ := cfg.GetString("database.scannerType")
	scannerType = strings.ToLower(scannerType)
	if scannerType == "" || scannerType == "none" {
		return nil, nil
	}
	if scannerType != "rdm" && scannerType != "mad" && scannerType != "golbat" {
		return nil, fmt.Errorf("unsupported scannerType: %s", scannerType)
	}

	confs, err := scannerConfigs(cfg)
	if err != nil {
		return nil, err
	}
	if len(confs) == 0 {
		return nil, fmt.Errorf("scanner config missing")
	}

	conns := make([]*sql.DB, 0, len(confs))
	for _, conf := range confs {
		dsn, err := mysqlDSNFromMap(conf)
		if err != nil {
			return nil, err
		}
		conn, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		if maxConns, ok := cfg.GetInt("tuning.maxDatabaseConnections"); ok && maxConns > 0 {
			conn.SetMaxOpenConns(maxConns)
			conn.SetMaxIdleConns(maxConns)
		}
		conns = append(conns, conn)
	}

	return &Client{scannerType: scannerType, conns: conns}, nil
}

// SetBreaker attaches a circuit breaker to the scanner client.
func (c *Client) SetBreaker(b *circuitbreaker.Breaker) {
	if c != nil {
		c.breaker = b
	}
}

// checkBreaker returns an error if the circuit breaker is open.
// On nil breaker or closed/half-open state, it returns nil.
func (c *Client) checkBreaker() error {
	if c == nil || c.breaker == nil {
		return nil
	}
	if !c.breaker.Allow() {
		return fmt.Errorf("scanner db circuit breaker open")
	}
	return nil
}

func (c *Client) recordSuccess() {
	if c != nil && c.breaker != nil {
		c.breaker.RecordSuccess()
	}
}

func (c *Client) recordFailure() {
	if c != nil && c.breaker != nil {
		c.breaker.RecordFailure()
	}
}

// Close shuts down scanner DB connections.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GetGymName returns the gym name for a given gym id.
func (c *Client) GetGymName(gymID string) (string, error) {
	if c == nil || gymID == "" {
		return "", nil
	}
	if err := c.checkBreaker(); err != nil {
		return "", err
	}
	query := "SELECT name FROM gym WHERE id = ? LIMIT 1"
	if c.scannerType == "mad" {
		query = "SELECT name FROM gymdetails WHERE gym_id = ? LIMIT 1"
	}
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		var name sql.NullString
		err := conn.QueryRow(query, gymID).Scan(&name)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			c.recordFailure()
			return "", err
		}
		c.recordSuccess()
		if name.Valid && name.String != "" {
			return name.String, nil
		}
	}
	return "", nil
}

// SearchGyms returns gym ids and names matching the query.
func (c *Client) SearchGyms(query string, limit int) ([]GymEntry, error) {
	if c == nil {
		return nil, nil
	}
	query = strings.TrimSpace(query)
	if limit <= 0 {
		limit = 25
	}
	like := "%" + query + "%"
	if query == "" {
		like = "%"
	}

	type gymQuery struct {
		sql string
	}
	queries := []gymQuery{
		{sql: "SELECT id, name, latitude, longitude FROM gym WHERE name LIKE ? ORDER BY name LIMIT ?"},
		{sql: "SELECT id, name, lat, lon FROM gym WHERE name LIKE ? ORDER BY name LIMIT ?"},
	}
	if c.scannerType == "mad" {
		queries = []gymQuery{
			{sql: "SELECT gym_id, name, lat, lon FROM gymdetails WHERE name LIKE ? ORDER BY name LIMIT ?"},
			{sql: "SELECT gym_id, name, latitude, longitude FROM gymdetails WHERE name LIKE ? ORDER BY name LIMIT ?"},
		}
	} else if c.scannerType == "golbat" {
		queries = []gymQuery{
			{sql: "SELECT gym_id, name, latitude, longitude FROM gym WHERE name LIKE ? ORDER BY name LIMIT ?"},
			{sql: "SELECT gym_id, name, lat, lon FROM gym WHERE name LIKE ? ORDER BY name LIMIT ?"},
			{sql: "SELECT id, name, latitude, longitude FROM gym WHERE name LIKE ? ORDER BY name LIMIT ?"},
			{sql: "SELECT id, name, lat, lon FROM gym WHERE name LIKE ? ORDER BY name LIMIT ?"},
		}
	}

	results := []GymEntry{}
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		for _, q := range queries {
			rows, err := conn.Query(q.sql, like, limit)
			if err != nil {
				continue
			}
			scanErr := func() error {
				defer rows.Close()
				for rows.Next() {
					var id sql.NullString
					var name sql.NullString
					var lat sql.NullFloat64
					var lon sql.NullFloat64
					if err := rows.Scan(&id, &name, &lat, &lon); err != nil {
						return err
					}
					if !id.Valid || !name.Valid || name.String == "" {
						continue
					}
					entry := GymEntry{ID: id.String, Name: name.String}
					if lat.Valid && lon.Valid {
						entry.Latitude = lat.Float64
						entry.Longitude = lon.Float64
						entry.HasCoords = true
					}
					results = append(results, entry)
					if len(results) >= limit {
						break
					}
				}
				return rows.Err()
			}()
			if scanErr != nil {
				return results, scanErr
			}
			if len(results) > 0 {
				return results, nil
			}
		}
	}
	return results, nil
}

// SearchGymsNearby returns nearby gym ids and names ordered by distance.
func (c *Client) SearchGymsNearby(lat, lon float64, limit int) ([]GymEntry, error) {
	if c == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}

	type gymQuery struct {
		sql string
	}
	queries := []gymQuery{
		{sql: "SELECT id, name, latitude, longitude FROM gym WHERE name != '' ORDER BY ((latitude-?)*(latitude-?)+(longitude-?)*(longitude-?)) ASC LIMIT ?"},
		{sql: "SELECT id, name, lat, lon FROM gym WHERE name != '' ORDER BY ((lat-?)*(lat-?)+(lon-?)*(lon-?)) ASC LIMIT ?"},
	}
	if c.scannerType == "mad" {
		queries = []gymQuery{
			{sql: "SELECT gym_id, name, lat, lon FROM gymdetails WHERE name != '' ORDER BY ((lat-?)*(lat-?)+(lon-?)*(lon-?)) ASC LIMIT ?"},
			{sql: "SELECT gym_id, name, latitude, longitude FROM gymdetails WHERE name != '' ORDER BY ((latitude-?)*(latitude-?)+(longitude-?)*(longitude-?)) ASC LIMIT ?"},
		}
	} else if c.scannerType == "golbat" {
		queries = []gymQuery{
			{sql: "SELECT gym_id, name, latitude, longitude FROM gym WHERE name != '' ORDER BY ((latitude-?)*(latitude-?)+(longitude-?)*(longitude-?)) ASC LIMIT ?"},
			{sql: "SELECT gym_id, name, lat, lon FROM gym WHERE name != '' ORDER BY ((lat-?)*(lat-?)+(lon-?)*(lon-?)) ASC LIMIT ?"},
			{sql: "SELECT id, name, latitude, longitude FROM gym WHERE name != '' ORDER BY ((latitude-?)*(latitude-?)+(longitude-?)*(longitude-?)) ASC LIMIT ?"},
			{sql: "SELECT id, name, lat, lon FROM gym WHERE name != '' ORDER BY ((lat-?)*(lat-?)+(lon-?)*(lon-?)) ASC LIMIT ?"},
		}
	}

	results := []GymEntry{}
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		for _, q := range queries {
			rows, err := conn.Query(q.sql, lat, lat, lon, lon, limit)
			if err != nil {
				continue
			}
			scanErr := func() error {
				defer rows.Close()
				for rows.Next() {
					var id sql.NullString
					var name sql.NullString
					var entryLat sql.NullFloat64
					var entryLon sql.NullFloat64
					if err := rows.Scan(&id, &name, &entryLat, &entryLon); err != nil {
						return err
					}
					if !id.Valid || !name.Valid || name.String == "" {
						continue
					}
					entry := GymEntry{ID: id.String, Name: name.String}
					if entryLat.Valid && entryLon.Valid {
						entry.Latitude = entryLat.Float64
						entry.Longitude = entryLon.Float64
						entry.HasCoords = true
					}
					results = append(results, entry)
					if len(results) >= limit {
						break
					}
				}
				return rows.Err()
			}()
			if scanErr != nil {
				return results, scanErr
			}
			if len(results) > 0 {
				return results, nil
			}
		}
	}
	return results, nil
}

// GetPokestopName returns the pokestop name for a given pokestop id.
func (c *Client) GetPokestopName(stopID string) (string, error) {
	if c == nil || stopID == "" {
		return "", nil
	}
	if err := c.checkBreaker(); err != nil {
		return "", err
	}
	query := "SELECT name FROM pokestop WHERE id = ? LIMIT 1"
	if c.scannerType == "mad" {
		query = "SELECT name FROM pokestop WHERE pokestop_id = ? LIMIT 1"
	}
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		var name sql.NullString
		err := conn.QueryRow(query, stopID).Scan(&name)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			c.recordFailure()
			return "", err
		}
		c.recordSuccess()
		if name.Valid && name.String != "" {
			return name.String, nil
		}
	}
	return "", nil
}

// GetStationName returns the station name for a given station id (golbat only).
func (c *Client) GetStationName(stationID string) (string, error) {
	if c == nil || stationID == "" {
		return "", nil
	}
	if c.scannerType != "golbat" {
		return "", nil
	}
	if err := c.checkBreaker(); err != nil {
		return "", err
	}
	query := "SELECT name FROM station WHERE id = ? LIMIT 1"
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		var name sql.NullString
		err := conn.QueryRow(query, stationID).Scan(&name)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			c.recordFailure()
			return "", err
		}
		c.recordSuccess()
		if name.Valid && name.String != "" {
			return name.String, nil
		}
	}
	return "", nil
}

// SearchStations returns station ids and names matching the query (golbat only).
func (c *Client) SearchStations(query string, limit int) ([]StationEntry, error) {
	if c == nil {
		return nil, nil
	}
	if c.scannerType != "golbat" {
		return nil, nil
	}
	query = strings.TrimSpace(query)
	if limit <= 0 {
		limit = 25
	}
	likeName := "%" + query + "%"
	likeID := "%" + query + "%"
	if query == "" {
		likeName = "%"
		likeID = "%"
	}

	type stationQuery struct {
		sql    string
		idLike bool
	}
	queries := []stationQuery{
		{sql: "SELECT id, name, lat, lon FROM station WHERE (name LIKE ? OR id LIKE ?) ORDER BY name LIMIT ?", idLike: true},
		{sql: "SELECT id, name, latitude, longitude FROM station WHERE (name LIKE ? OR id LIKE ?) ORDER BY name LIMIT ?", idLike: true},
		{sql: "SELECT id, name, lat, lon FROM station WHERE name LIKE ? ORDER BY name LIMIT ?", idLike: false},
		{sql: "SELECT id, name, latitude, longitude FROM station WHERE name LIKE ? ORDER BY name LIMIT ?", idLike: false},
	}

	results := []StationEntry{}
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		for _, q := range queries {
			var rows *sql.Rows
			var err error
			if q.idLike {
				rows, err = conn.Query(q.sql, likeName, likeID, limit)
			} else {
				rows, err = conn.Query(q.sql, likeName, limit)
			}
			if err != nil {
				continue
			}
			scanErr := func() error {
				defer rows.Close()
				for rows.Next() {
					var id sql.NullString
					var name sql.NullString
					var lat sql.NullFloat64
					var lon sql.NullFloat64
					if err := rows.Scan(&id, &name, &lat, &lon); err != nil {
						return err
					}
					if !id.Valid || !name.Valid || name.String == "" {
						continue
					}
					entry := StationEntry{ID: id.String, Name: name.String}
					if lat.Valid && lon.Valid {
						entry.Latitude = lat.Float64
						entry.Longitude = lon.Float64
						entry.HasCoords = true
					}
					results = append(results, entry)
					if len(results) >= limit {
						break
					}
				}
				return rows.Err()
			}()
			if scanErr != nil {
				return results, scanErr
			}
			if len(results) > 0 {
				return results, nil
			}
		}
	}
	return results, nil
}

// SearchStationsNearby returns nearby station ids and names ordered by distance (golbat only).
func (c *Client) SearchStationsNearby(lat, lon float64, limit int) ([]StationEntry, error) {
	if c == nil {
		return nil, nil
	}
	if c.scannerType != "golbat" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 25
	}

	type stationQuery struct {
		sql string
	}
	queries := []stationQuery{
		{sql: "SELECT id, name, lat, lon FROM station WHERE name != '' ORDER BY ((lat-?)*(lat-?)+(lon-?)*(lon-?)) ASC LIMIT ?"},
		{sql: "SELECT id, name, latitude, longitude FROM station WHERE name != '' ORDER BY ((latitude-?)*(latitude-?)+(longitude-?)*(longitude-?)) ASC LIMIT ?"},
	}

	results := []StationEntry{}
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		for _, q := range queries {
			rows, err := conn.Query(q.sql, lat, lat, lon, lon, limit)
			if err != nil {
				continue
			}
			scanErr := func() error {
				defer rows.Close()
				for rows.Next() {
					var id sql.NullString
					var name sql.NullString
					var entryLat sql.NullFloat64
					var entryLon sql.NullFloat64
					if err := rows.Scan(&id, &name, &entryLat, &entryLon); err != nil {
						return err
					}
					if !id.Valid || !name.Valid || name.String == "" {
						continue
					}
					entry := StationEntry{ID: id.String, Name: name.String}
					if entryLat.Valid && entryLon.Valid {
						entry.Latitude = entryLat.Float64
						entry.Longitude = entryLon.Float64
						entry.HasCoords = true
					}
					results = append(results, entry)
					if len(results) >= limit {
						break
					}
				}
				return rows.Err()
			}()
			if scanErr != nil {
				return results, scanErr
			}
			if len(results) > 0 {
				return results, nil
			}
		}
	}
	return results, nil
}

// GetStopData returns nearby stop and gym details within bounds.
func (c *Client) GetStopData(aLat, aLon, bLat, bLon float64) ([]StopData, error) {
	if c == nil {
		return nil, nil
	}
	minLat := math.Min(aLat, bLat)
	maxLat := math.Max(aLat, bLat)
	minLon := math.Min(aLon, bLon)
	maxLon := math.Max(aLon, bLon)

	stops := []StopData{}
	for _, conn := range c.conns {
		if conn == nil {
			continue
		}
		stopQuery := "SELECT lat, lon FROM pokestop WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ? AND deleted = 0 AND enabled = 1"
		gymQuery := "SELECT lat, lon, team_id, available_slots FROM gym WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ? AND deleted = 0 AND enabled = 1"
		if c.scannerType == "mad" {
			stopQuery = "SELECT latitude, longitude FROM pokestop WHERE latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ?"
			gymQuery = "SELECT latitude, longitude, team_id, slots_available FROM gym WHERE latitude BETWEEN ? AND ? AND longitude BETWEEN ? AND ?"
		}

		rows, err := conn.Query(stopQuery, minLat, maxLat, minLon, maxLon)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var lat, lon float64
			if err := rows.Scan(&lat, &lon); err != nil {
				_ = rows.Close()
				return nil, err
			}
			stops = append(stops, StopData{
				Latitude:  lat,
				Longitude: lon,
				Type:      "stop",
			})
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}

		gymRows, err := conn.Query(gymQuery, minLat, maxLat, minLon, maxLon)
		if err != nil {
			return nil, err
		}
		for gymRows.Next() {
			var lat, lon float64
			var teamID, slots int
			if err := gymRows.Scan(&lat, &lon, &teamID, &slots); err != nil {
				_ = gymRows.Close()
				return nil, err
			}
			stops = append(stops, StopData{
				Latitude:  lat,
				Longitude: lon,
				Type:      "gym",
				TeamID:    teamID,
				Slots:     slots,
			})
		}
		if err := gymRows.Close(); err != nil {
			return nil, err
		}
	}
	return stops, nil
}

func scannerConfigs(cfg *config.Config) ([]map[string]any, error) {
	raw, ok := cfg.Get("database.scanner")
	if !ok || raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case map[string]any:
		return []map[string]any{v}, nil
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, entry := range v {
			if m, ok := entry.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	default:
		return nil, nil
	}
}

func mysqlDSNFromMap(values map[string]any) (string, error) {
	host := getString(values["host"])
	name := getString(values["database"])
	if host == "" || name == "" {
		return "", fmt.Errorf("scanner database.host and database.database are required")
	}
	user := getString(values["user"])
	pass := getString(values["password"])
	port := getInt(values["port"], 3306)
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&loc=Local&multiStatements=true", user, pass, host, port, name), nil
}

func getString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

func getInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var out int
		if _, err := fmt.Sscanf(v, "%d", &out); err == nil {
			return out
		}
	}
	return fallback
}
