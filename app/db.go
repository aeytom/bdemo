package app

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/Jeffail/gabs/v2"
	_ "github.com/mattn/go-sqlite3"
)

type Entry struct {
	Id             int64
	Lfdnr          string
	Start          time.Time
	Thema          string
	Von            string
	Bis            string
	Plz            string
	StrasseNr      string
	Aufzugsstrecke string
}

func (s *Settings) GetDatabase() *sql.DB {
	if s.Db.handle != nil {
		return s.Db.handle
	}

	log.Print("mysql DSN ", s.Db.Dsn, "?", s.Db.Opt)
	if handle, err := sql.Open("sqlite3", s.Db.Dsn+"?"+s.Db.Opt); err != nil {
		s.Fatal(err)
	} else {
		s.Db.handle = handle
		s.initDb(handle)
		s.expire(handle)
	}
	return s.Db.handle
}

func (s *Settings) CloseDatabase() {
	if s.Db.handle != nil {
		s.Db.handle.Close()
	}
}

func (s *Settings) initDb(db *sql.DB) {
	sqlStmt := `CREATE TABLE "feed" (
		"pk"	TEXT NOT NULL,
		"start"	TEXT NOT NULL,
		"sent"	TEXT,
		"id"	INTEGER NOT NULL,
		"lfdnr"	TEXT NOT NULL,
		"thema"	TEXT NOT NULL,
		"von"	TEXT NOT NULL,
		"bis"	TEXT NOT NULL,
		"plz"	TEXT,
		"strasse_nr"	TEXT,
		"aufzugsstrecke"	TEXT,
		"checksum"	TEXT,
		PRIMARY KEY("pk")
	)`
	if _, err := db.Exec(sqlStmt); err != nil {
		s.Log(err)
	}
}

func (s *Settings) expire(db *sql.DB) {
	owa := time.Now().AddDate(0, 0, -7)
	sdel := "DELETE FROM `feed` WHERE `start`<?"
	if _, err := db.Exec(sdel, owa.Format(time.RFC3339)); err != nil {
		s.Fatal(err)
	}
}

func (s *Settings) StoreItem(item Entry) bool {
	db := s.GetDatabase()

	pk := sha256.New()
	pk.Write([]byte(item.Start.UTC().Format(time.RFC3339)))
	pk.Write([]byte(item.Thema))

	sql := "INSERT OR IGNORE INTO `feed` (`pk`,`start`,`id`,`lfdnr`,`thema`,`von`,`bis`,`plz`,`strasse_nr`,`aufzugsstrecke`,`checksum`) VALUES (?,?,?,?,?,?,?,?,?,?,?)"
	if rslt, err := db.Exec(
		sql,
		fmt.Sprintf("%x", pk.Sum(nil)),
		item.Start.Format(time.RFC3339),
		item.Id,
		item.Lfdnr,
		item.Thema,
		item.Von,
		item.Bis,
		item.Plz,
		item.StrasseNr,
		item.Aufzugsstrecke,
		item.Checksum(),
	); err != nil {
		s.Log(err)
	} else if ra, err := rslt.RowsAffected(); err != nil {
		s.Log(err)
	} else {
		return ra > 0
	}
	return false
}

func (s *Settings) StoreJson(j *gabs.Container) bool {

	id, err := j.Path("id").Data().(json.Number).Int64()
	if err != nil {
		s.Log(err)
		return false
	}
	start := j.Path("datum").Data().(string) + " " + j.Path("von").Data().(string)
	subject := j.Path("thema").Data().(string)

	startTime, err := time.ParseInLocation("02.01.2006 15:04", start, s.Location())
	if err != nil {
		s.Log(err)
		return false
	}

	if startTime.Before(time.Now()) || startTime.After(time.Now().Add(8*time.Hour)) {
		// event in
		return false
	}

	item := Entry{
		Id:             id,
		Lfdnr:          j.Path("lfdnr").Data().(string),
		Start:          startTime,
		Thema:          subject,
		Von:            j.Path("von").Data().(string),
		Bis:            j.Path("bis").Data().(string),
		Plz:            j.Path("plz").Data().(string),
		StrasseNr:      j.Path("strasse_nr").Data().(string),
		Aufzugsstrecke: j.Path("aufzugsstrecke").Data().(string),
	}
	return s.StoreItem(item)
}

func (s *Settings) GetUnsent() *Entry {
	db := s.GetDatabase()
	sql := "SELECT `id`,`lfdnr`,`start`,`thema`,`von`,`bis`,`plz`,`strasse_nr`,`aufzugsstrecke` FROM `feed` WHERE `sent` IS NULL AND `start` > ? AND `start` < ? ORDER BY `id` ASC LIMIT 1"
	row := db.QueryRow(sql, time.Now().Format(time.RFC3339), time.Now().Add(8*time.Hour).Format(time.RFC3339))
	entry := Entry{}
	start := ""
	if err := row.Scan(&entry.Id, &entry.Lfdnr, &start, &entry.Thema, &entry.Von, &entry.Bis, &entry.Plz, &entry.StrasseNr, &entry.Aufzugsstrecke); err != nil {
		s.Log(err)
		return nil
	}
	var err error
	entry.Start, err = time.ParseInLocation(time.RFC3339, start, s.Location())
	if err != nil {
		s.Log(err)
	}
	return &entry
}

func (s *Settings) IsDuplicate(item *Entry) bool {
	db := s.GetDatabase()
	sql := "SELECT `sent` FROM `feed` WHERE `sent` IS NOT NULL AND `checksum` = ? ORDER BY `sent` DESC LIMIT 1"
	row := db.QueryRow(sql, item.Checksum())
	sent := ""
	if err := row.Scan(&sent); err != nil {
		s.Log(err)
		return false
	}
	if _, err := time.ParseInLocation("2006-01-02 15:04:05", sent, s.Location()); err != nil {
		s.Log(err)
		return false
	}
	return true
}

func (s *Settings) MarkSent(entry *Entry) {
	db := s.GetDatabase()
	sql := "UPDATE `feed` SET `sent`=datetime() WHERE `id`=?"
	if _, err := db.Exec(sql, entry.Id); err != nil {
		s.Log(err)
	}
}

func (s *Settings) MarkError(entry *Entry, err error) {
	db := s.GetDatabase()
	sql := "UPDATE `feed` SET `sent`=? WHERE `id`=?"
	if _, err := db.Exec(sql, err, entry.Id); err != nil {
		s.Log(err)
	}
}

func (e *Entry) Checksum() string {
	h := sha256.New()
	h.Write(stripNonAlphaNum(e.Thema))
	h.Write(stripNonAlphaNum(e.Aufzugsstrecke))
	h.Write(stripNonAlphaNum(e.Plz))
	h.Write(stripNonAlphaNum(e.StrasseNr))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func stripNonAlphaNum(buf string) []byte {
	out := regexp.MustCompile("[^[:alnum:]]").ReplaceAll([]byte(buf), []byte(""))
	return out
}
