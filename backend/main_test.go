package main

import (
	"testing"
	"time"
)

// Broker must deliver a published event to every live subscriber and stop
// delivering after unsubscribe.
func TestBrokerFanOut(t *testing.T) {
	b := newBroker()
	a := b.subscribe("split1")
	c := b.subscribe("split1")
	other := b.subscribe("split2")

	b.publish("split1", []byte("hi"))

	for _, ch := range []chan []byte{a, c} {
		select {
		case msg := <-ch:
			if string(msg) != "hi" {
				t.Fatalf("got %q, want hi", msg)
			}
		case <-time.After(time.Second):
			t.Fatal("subscriber did not receive event")
		}
	}
	select {
	case <-other:
		t.Fatal("split2 subscriber received a split1 event")
	default:
	}

	b.unsubscribe("split1", a)
	b.publish("split1", []byte("again"))
	select {
	case _, open := <-a:
		if open {
			t.Fatal("unsubscribed channel still received an event")
		}
	default:
	}
}

func TestSlugAndTokenShape(t *testing.T) {
	if got := newSlug(); len(got) != 4 {
		t.Fatalf("slug %q len %d, want 4", got, len(got))
	}
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		tok := newToken()
		if len(tok) < 20 {
			t.Fatalf("token too short: %q", tok)
		}
		if seen[tok] {
			t.Fatalf("token collision: %q", tok)
		}
		seen[tok] = true
	}
}
