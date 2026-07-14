package realtime

import (
	"errors"
	"sync"
	"testing"
)

func TestHubRoutesUserAdminAndGlobalWakeups(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	userA := mustSubscribe(t, hub, "user-a", false)
	defer userA.Close()
	adminB := mustSubscribe(t, hub, "user-b", true)
	defer adminB.Close()
	userC := mustSubscribe(t, hub, "user-c", false)
	defer userC.Close()

	hub.PublishUser("user-a")
	expectWakeup(t, userA.Events())
	expectNoWakeup(t, adminB.Events())
	expectNoWakeup(t, userC.Events())

	hub.PublishAdmin()
	expectNoWakeup(t, userA.Events())
	expectWakeup(t, adminB.Events())
	expectNoWakeup(t, userC.Events())

	hub.PublishAll()
	expectWakeup(t, userA.Events())
	expectWakeup(t, adminB.Events())
	expectWakeup(t, userC.Events())
}

func TestHubCoalescesPendingWakeups(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	subscription := mustSubscribe(t, hub, "user-a", true)
	defer subscription.Close()

	hub.PublishAll()
	hub.PublishAdmin()
	hub.PublishUser("user-a")

	expectWakeup(t, subscription.Events())
	expectNoWakeup(t, subscription.Events())
}

func TestSubscriptionCloseAndHubCloseAreIdempotent(t *testing.T) {
	hub := NewHub()
	subscription := mustSubscribe(t, hub, "user-a", false)

	subscription.Close()
	subscription.Close()
	expectClosed(t, subscription.Events())

	hub.Close()
	hub.Close()
	if _, err := hub.Subscribe("user-b", false); !errors.Is(err, ErrHubClosed) {
		t.Fatalf("Subscribe() error = %v, want ErrHubClosed", err)
	}
	hub.PublishAll()
}

func TestHubCloseClosesActiveSubscriptions(t *testing.T) {
	hub := NewHub()
	subscription := mustSubscribe(t, hub, "user-a", false)

	hub.Close()
	expectClosed(t, subscription.Events())
	subscription.Close()
}

func TestHubConcurrentPublishUnsubscribeAndClose(t *testing.T) {
	hub := NewHub()
	subscriptions := make([]*Subscription, 32)
	for index := range subscriptions {
		subscriptions[index] = mustSubscribe(t, hub, "user-a", index%2 == 0)
	}

	var workers sync.WaitGroup
	for _, subscription := range subscriptions {
		workers.Add(2)
		go func() {
			defer workers.Done()
			for index := 0; index < 100; index++ {
				hub.PublishUser("user-a")
				hub.PublishAdmin()
				hub.PublishAll()
			}
		}()
		go func(subscription *Subscription) {
			defer workers.Done()
			subscription.Close()
		}(subscription)
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		hub.Close()
	}()
	workers.Wait()

	hub.Close()
	for _, subscription := range subscriptions {
		subscription.Close()
	}
}

func TestHubRejectsEmptySubscriptionUserID(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	for _, userID := range []string{"", " ", " user-a", "user-a "} {
		if _, err := hub.Subscribe(userID, false); !errors.Is(err, ErrUserIDRequired) {
			t.Fatalf("Subscribe(%q) error = %v, want ErrUserIDRequired", userID, err)
		}
	}
}

func mustSubscribe(t *testing.T, hub *Hub, userID string, admin bool) *Subscription {
	t.Helper()
	subscription, err := hub.Subscribe(userID, admin)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	return subscription
}

func expectWakeup(t *testing.T, events <-chan struct{}) {
	t.Helper()
	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("event channel closed before wakeup")
		}
	default:
		t.Fatal("expected wakeup")
	}
}

func expectNoWakeup(t *testing.T, events <-chan struct{}) {
	t.Helper()
	select {
	case _, ok := <-events:
		if !ok {
			t.Fatal("event channel unexpectedly closed")
		}
		t.Fatal("unexpected wakeup")
	default:
	}
}

func expectClosed(t *testing.T, events <-chan struct{}) {
	t.Helper()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected closed event channel")
		}
	default:
		t.Fatal("expected event channel to be closed")
	}
}
