package reminders

import "time"

// RelevanceWindow is how long a slot stays sendable after its local instant.
// A worker that was down longer than this does not replay the missed slot.
const RelevanceWindow = 2 * time.Hour

// Plan is one notification to insert. SMS plans are already capped at two per local day.
type Plan struct {
	Slot      string
	Channel   string
	LocalDate time.Time
	SendAt    time.Time
	ExpiresAt time.Time
	Title     string
	Body      string
}

// DuePlans returns the slots whose send instant is inside the relevance window.
// Preventive reminders are daytime (09:00). After expiry, SMS is only at 08:00 and 18:00,
// except an expiry that falls between those hours, which sends once at that instant and
// still counts toward the two SMS of the day. An expiry after 18:00 waits until the next morning.
func DuePlans(now time.Time, loc *time.Location, expires time.Time, company string, smsEnabled bool) []Plan {
	if loc == nil {
		loc = time.UTC
	}
	now = now.In(loc)
	expires = expires.In(loc)
	var plans []Plan
	add := func(slot string, at time.Time, channels []string, title, body string) {
		if !relevant(at, now) {
			return
		}
		for _, channel := range channels {
			if channel == "sms" && !smsEnabled {
				continue
			}
			plans = append(plans, Plan{
				Slot: slot, Channel: channel, LocalDate: dateOnly(at), SendAt: at.UTC(),
				ExpiresAt: expires.UTC(), Title: title, Body: body,
			})
		}
	}
	j3 := atClock(expires.AddDate(0, 0, -3), 9, 0, loc)
	j1 := atClock(expires.AddDate(0, 0, -1), 9, 0, loc)
	if now.Before(expires) {
		add("j3", j3, []string{"push", "in_app"}, "Abonnement", "EasyStock: l'abonnement de "+company+" expire le "+expires.Format("02/01")+".")
		add("j1", j1, []string{"sms", "push", "in_app"}, "Abonnement", "EasyStock: abonnement "+company+" expire demain. Renouvelez dans l'application.")
		return capSMS(plans, 2)
	}
	day := dateOnly(now)
	expDay := dateOnly(expires)
	if day.Before(expDay) {
		return nil
	}
	morning := atClock(day, 8, 0, loc)
	evening := atClock(day, 18, 0, loc)
	late := sameDay(day, expires) && expires.After(evening)
	if late {
		return nil
	}
	expiryBody := "EasyStock: l'abonnement de " + company + " a expiré. Renouvelez pour continuer."
	morningBody := "EasyStock: rappel, abonnement " + company + " expiré. Renouvelez aujourd'hui."
	eveningBody := "EasyStock: abonnement " + company + " toujours expiré. Ouvrez l'application pour payer."
	if !expires.After(morning) {
		body := morningBody
		if sameDay(day, expires) || (sameDay(day, expires.AddDate(0, 0, 1)) && expires.After(atClock(expDay, 18, 0, loc))) {
			body = expiryBody
		}
		if sameDay(day, expires) {
			body = expiryBody
		}
		add("morning", morning, []string{"sms", "push", "in_app"}, "Abonnement expiré", body)
	} else if sameDay(day, expires) {
		add("expiry", expires, []string{"sms", "push", "in_app"}, "Abonnement expiré", expiryBody)
	}
	add("evening", evening, []string{"sms", "push", "in_app"}, "Abonnement expiré", eveningBody)
	return capSMS(plans, 2)
}

func relevant(sendAt, now time.Time) bool {
	return !now.Before(sendAt) && now.Before(sendAt.Add(RelevanceWindow))
}

func atClock(day time.Time, hour, minute int, loc *time.Location) time.Time {
	y, m, d := day.In(loc).Date()
	return time.Date(y, m, d, hour, minute, 0, 0, loc)
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func capSMS(plans []Plan, max int) []Plan {
	seen := map[string]int{}
	var out []Plan
	for _, plan := range plans {
		if plan.Channel == "sms" {
			key := plan.LocalDate.Format("2006-01-02")
			if seen[key] >= max {
				continue
			}
			seen[key]++
		}
		out = append(out, plan)
	}
	return out
}
