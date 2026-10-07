package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

// TestConcurrentLoad: viele Goroutinen rufen gleichzeitig verschachtelte
// Aufrufe (A -> B) auf, während parallel Plugins registriert und abgemeldet
// werden und die Routing-Tabelle gelesen wird. Erwartet: keine Fehler, kein
// Deadlock, alle Zähler am Ende wieder auf null.
func TestConcurrentLoad(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	var bCalls atomic.Int64

	d.Register("b", manifest("b", sdk.Capability{Object: "B", Actions: []string{"get"}}),
		handlerFunc(func(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
			bCalls.Add(1)
			time.Sleep(time.Millisecond)
			return sdk.Response{Payload: sdk.CallFromContext(ctx).TenantID}, nil
		}))
	d.Register("a", manifest("a", sdk.Capability{Object: "A", Actions: []string{"get"}}),
		handlerFunc(func(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
			return d.HandleNested(ctx, sdk.Request{Object: "B", Action: "get"})
		}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Hintergrund: Plugins kommen und gehen, Tabelle wird gelesen.
	stop := make(chan struct{})
	var churn sync.WaitGroup
	churn.Add(1)
	go func() {
		defer churn.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			name := fmt.Sprintf("churn-%d", i%4)
			_ = d.Register(name, manifest(name, sdk.Capability{Object: "Churn" + fmt.Sprint(i%4), Actions: []string{"x"}}), echoCall())
			_ = d.Routes()
			_ = d.Catalog()
			_ = d.Unregister(name)(ctx)
		}
	}()

	const workers, calls = 100, 50
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range calls {
				tenant := fmt.Sprintf("t-%d-%d", w, i)
				c := sdk.WithCall(ctx, sdk.CallContext{TenantID: tenant})
				resp, err := d.Handle(c, sdk.Request{Object: "A", Action: "get"})
				if err != nil {
					errs <- err
					return
				}
				// Jede Aufrufkette behält ihren eigenen Mandanten.
				if resp.Payload != tenant {
					errs <- fmt.Errorf("Mandant vertauscht: %v statt %s", resp.Payload, tenant)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(stop)
	churn.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	if n := bCalls.Load(); n != workers*calls {
		t.Fatalf("B-Aufrufe: %d, erwartet %d", n, workers*calls)
	}
	d.reqMu.Lock()
	open := len(d.requests)
	d.reqMu.Unlock()
	if open != 0 {
		t.Fatalf("%d Wurzelanfragen nicht aufgeräumt", open)
	}
	if err := d.Unregister("a")(ctx); err != nil {
		t.Fatalf("inflight-Zähler nicht auf null: %v", err)
	}
}

// TestSlowPluginDoesNotBlockOthers: Der Dispatcher hält während eines
// Plugin-Aufrufs keine Sperre. Ein hängendes Plugin blockiert weder andere
// Routen noch Registrierungen.
func TestSlowPluginDoesNotBlockOthers(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	started, release := make(chan struct{}), make(chan struct{})
	d.Register("slow", manifest("slow", sdk.Capability{Object: "Slow", Actions: []string{"run"}}),
		handlerFunc(func(context.Context, sdk.Request) (sdk.Response, error) {
			close(started)
			<-release
			return sdk.Response{}, nil
		}))
	d.Register("fast", manifest("fast", sdk.Capability{Object: "Fast", Actions: []string{"run"}}), echoCall())

	go d.Handle(context.Background(), sdk.Request{Object: "Slow", Action: "run"})
	<-started
	defer close(release)

	done := make(chan error, 1)
	go func() {
		if _, err := d.Handle(context.Background(), sdk.Request{Object: "Fast", Action: "run"}); err != nil {
			done <- err
			return
		}
		done <- d.Register("late", manifest("late", sdk.Capability{Object: "Late", Actions: []string{"run"}}), echoCall())
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blockiert: Aufruf/Registrierung wartet auf das langsame Plugin")
	}
}
