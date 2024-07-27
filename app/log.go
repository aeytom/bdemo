package app

import "log"

type Log interface {
	Fatal(v ...any)
	Log(v ...any)
	Logf(format string, v ...any)
}

func (s *Settings) Fatal(v ...any) {
	log.Fatal(v...)
}

func (s *Settings) Log(v ...any) {
	log.Print(v...)
}

func (s *Settings) Logf(format string, v ...any) {
	log.Printf(format, v...)
}
