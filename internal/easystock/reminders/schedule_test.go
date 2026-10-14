package reminders

import (
	"testing"
	"time"
)

func TestDuePlansUsesCompanyTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Africa/Lubumbashi")
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 3, 10, 6, 0, 0, 0, time.UTC) // 08:00 local
	now := time.Date(2026, 3, 10, 6, 5, 0, 0, time.UTC)
	plans := DuePlans(now, loc, expires, "Beni", true)
	var sms int
	for _, plan := range plans {
		if plan.Channel == "sms" {
			sms++
			if plan.Slot != "morning" {
				t.Fatalf("slot %s", plan.Slot)
			}
			if plan.SendAt.UTC().Hour() != 6 {
				t.Fatalf("send at %s, want 06:00 UTC", plan.SendAt.UTC())
			}
		}
	}
	if sms != 1 {
		t.Fatalf("sms %d", sms)
	}
}

func TestDuePlansCapsTwoSMS(t *testing.T) {
	loc, err := time.LoadLocation("Africa/Lubumbashi")
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 3, 10, 4, 0, 0, 0, time.UTC) // 06:00 local, before 08:00
	morning := time.Date(2026, 3, 10, 6, 10, 0, 0, time.UTC)
	evening := time.Date(2026, 3, 10, 16, 10, 0, 0, time.UTC)
	if n := smsCount(DuePlans(morning, loc, expires, "Beni", true)); n != 1 {
		t.Fatalf("morning sms %d", n)
	}
	if n := smsCount(DuePlans(evening, loc, expires, "Beni", true)); n != 1 {
		t.Fatalf("evening sms %d", n)
	}
}

func TestDuePlansSkipsStaleSlots(t *testing.T) {
	loc, err := time.LoadLocation("Africa/Lubumbashi")
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 3, 1, 6, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC) // 14:00 local, outside both windows
	plans := DuePlans(now, loc, expires, "Beni", true)
	if len(plans) != 0 {
		t.Fatalf("replayed %d plans", len(plans))
	}
}

func TestExpiryAfterEveningWaits(t *testing.T) {
	loc, err := time.LoadLocation("Africa/Lubumbashi")
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 3, 10, 19, 30, 0, 0, time.UTC) // 21:30 local
	now := expires.Add(10 * time.Minute)
	if len(DuePlans(now, loc, expires, "Beni", true)) != 0 {
		t.Fatal("sent an SMS outside the allowed hours")
	}
	nextMorning := time.Date(2026, 3, 11, 6, 5, 0, 0, time.UTC)
	plans := DuePlans(nextMorning, loc, expires, "Beni", true)
	if smsCount(plans) != 1 || plans[0].Slot != "morning" {
		t.Fatalf("%+v", plans)
	}
}

func smsCount(plans []Plan) int {
	n := 0
	for _, plan := range plans {
		if plan.Channel == "sms" {
			n++
		}
	}
	return n
}
