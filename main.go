package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/aeytom/bdemo/app"
	"github.com/aeytom/bdemo/simplesearch"
	"github.com/mattn/go-mastodon"
)

var (
	tags = []string{
		"Demonstration",
		"Mahnwache",
		"Versammlung",
		"Berlin",
		"Polizei",
		"Friedrichshain",
		"Kreuzberg",
		"Pankow",
		"Charlottenburg",
		"Wilmersdorf",
		"Spandau",
		"Steglitz",
		"Zehlendorf",
		"Tempelhof",
		"Schöneberg",
		"Neukölln",
		"Treptow",
		"Köpenick",
		"Marzahn",
		"Hellersdorf",
		"Lichtenberg",
		"Reinickendorf",
	}
)

func main() {
	var err error

	settings := app.LoadConfig()
	mc := settings.Mastodon.GetClient(settings)
	if err := settings.Mastodon.CompileTags(tags); err != nil {
		settings.Fatal(err)
	}

	ss := simplesearch.SimpleSearchConfig{
		UserAgent: settings.Mastodon.UserAgent,
	}
	defer settings.CloseDatabase()

	for doff := 0; doff < 2; doff++ {
		// get data for today and tomorrow
		tomorrow := time.Now().AddDate(0, 0, doff)
		url := settings.SimpleSearch.Base + settings.SimpleSearch.Feed + tomorrow.Format("02.01.2006")
		settings.Log(url)

		var sj *gabs.Container
		if sj, err = ss.FetchJson(url); err != nil {
			log.Fatal(err)
		}

		for _, i := range sj.S("index").Children() {
			fmt.Printf("%#v", i.Data())
			if !settings.StoreJson(i) {
				settings.Log("… not stored")
			}
		}
	}

	sendUpcoming(settings, mc)
	replyNotifications(mc, settings)
}

func replyNotifications(mc *mastodon.Client, settings *app.Settings) {
	pg := &mastodon.Pagination{
		MaxID:   "",
		SinceID: "",
		MinID:   "",
		Limit:   40,
	}
	if nl, err := mc.GetNotifications(context.Background(), pg); err != nil {
		settings.Log(err)
	} else {
		for i, n := range nl {
			settings.Log(i, " ", n.ID, " ", n.Type, " ", n.Account.Acct)
			if n.Account.Bot {
				continue
			}

			switch n.Type {
			// case "favourite":
			// 	sendReply(settings, mc, n, "Danke für ⭐")
			case "follow":
				sendReply(settings, mc, n, "Vielen Dank für das Interesse. 🤗")
			// case "reblog":
			// 	sendReply(settings, mc, n, "Vielen Dank für die Unterstützung. 🤗")
			case "mention":
				doFavourite(settings, mc, n)
			}
		}
	}
	if err := mc.ClearNotifications(context.Background()); err != nil {
		settings.Log(err)
	}
}

func doFavourite(settings *app.Settings, mc *mastodon.Client, note *mastodon.Notification) {
	settings.Log(note)
	if s, err := mc.Favourite(context.Background(), note.Status.ID); err != nil {
		settings.Log("doFavorite ", err)
	} else {
		settings.Log("doFavourite ", s.Account)
	}
}

func sendReply(settings *app.Settings, mc *mastodon.Client, note *mastodon.Notification, text string) {

	toot := &mastodon.Toot{
		Status:     "@" + note.Account.Acct + " " + text,
		Sensitive:  false,
		Visibility: "direct", // "unlisted"
		Language:   "de",
	}
	if note.Status != nil && note.Status.ID != "" {
		toot.InReplyToID = note.Status.ID
	}
	if _, err := mc.PostStatus(context.Background(), toot); err != nil {
		settings.Log(err)
	} else if err = mc.DismissNotification(context.Background(), note.ID); err != nil {
		settings.Log(err)
	}
}

func sendUpcoming(settings *app.Settings, mc *mastodon.Client) {
	// exclude events without start time
	for entry := settings.GetUnsent(); entry != nil; entry = settings.GetUnsent() {

		if settings.IsDuplicate(entry) {
			settings.MarkSent(entry)
			continue
		}
		if entry.Von == "00:00" || entry.Von == "" {
			settings.MarkSent(entry)
			continue
		}
		if entry.Start.Add(time.Hour).Before(time.Now()) {
			settings.MarkSent(entry)
			continue
		}
		scheduledAt := entry.Start.Add(-6 * time.Hour)
		link := regexp.MustCompile("//www.berlin.de/").ReplaceAllString(settings.SimpleSearch.Base, "//berlin.de/") + settings.SimpleSearch.Detail + fmt.Sprintf("%d", entry.Id)
		text := entry.Thema
		if entry.StrasseNr != "" {
			text += "\n\nOrt: " + entry.StrasseNr
			if entry.Plz != "" {
				text += ", " + entry.Plz + " Berlin"
			}
		} else if entry.Aufzugsstrecke != "" {
			text += "\n\n" + entry.Aufzugsstrecke
		}
		footer := "\nBeginn: " + entry.Start.Format("2.1. 15:04") + " Uhr\n\n" + settings.SimpleSearch.Title + "\n" + link
		status := settings.Mastodon.Hashtag(text) + footer
		length := len(status)
		if length > 500 {
			status = settings.Mastodon.Hashtag(text[:len(text)-(length-501)]) + "…" + footer
		}
		toot := &mastodon.Toot{
			Status:      status,
			Sensitive:   false,
			Visibility:  "public",
			Language:    "de",
			ScheduledAt: &scheduledAt,
		}
		if _, err := mc.PostStatus(context.Background(), toot); err != nil {
			settings.MarkError(entry, err)
			settings.Log(err)
			continue
		} else {
			settings.MarkSent(entry)
			settings.Log("… sent ", entry.Id)
		}
	}
}
