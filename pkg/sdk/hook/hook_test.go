package hook

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

type services struct {
	resp sdk.Response
	err  error
	last string
}

func (s *services) Call(_ context.Context, object, action string, _ any) (sdk.Response, error) {
	s.last = object + "." + action
	return s.resp, s.err
}

func TestCallWithoutDispatcher(t *testing.T) {
	s := &services{err: sdk.ErrUnimplemented}
	r, err := Call(context.Background(), s, "ledger.posting", PhaseModify, map[string]any{"a": 1})
	if err != nil || r.Data.(map[string]any)["a"] != 1 || s.last != "Hook.Call" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestResult(t *testing.T) {
	s := &services{resp: sdk.Response{Payload: map[string]any{
		"data": map[string]any{"text": "neu"},
		"messages": []any{
			map[string]any{"type": "W", "id": "W-1", "text": "Hinweis"},
			map[string]any{"type": "E", "id": "E-1", "text": "Konto fehlt", "field": "account", "source": "tax/TaxCheck"},
		},
		"subscribers": 2,
	}}}
	r, err := Call(context.Background(), s, "ledger.posting", PhaseCheck, nil)
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Text string }
	if err := r.DecodeData(&data); err != nil || data.Text != "neu" {
		t.Fatalf("Daten: %+v %v", data, err)
	}
	if !r.HasErrors() || len(r.Filter(TypeWarning)) != 1 || !errors.Is(r.Err(), sdk.ErrInvalidArgument) ||
		!strings.Contains(r.Err().Error(), "E-1 Konto fehlt (tax/TaxCheck)") {
		t.Fatalf("Result: %+v / %v", r, r.Err())
	}
	if (Result{}).Err() != nil {
		t.Fatal("ohne Fehler kein error")
	}
}

func TestFunc(t *testing.T) {
	h := Func(func(_ context.Context, req Request) (Response, error) {
		var in struct{ N int }
		if err := req.DecodeData(&in); err != nil {
			return Response{}, err
		}
		return Reply(map[string]any{"n": in.N + 1}, Info("S-1", req.Hook+"/"+req.Action)), nil
	})
	resp, err := h(context.Background(), sdk.Request{Object: "X", Action: CallbackAction,
		Payload: map[string]any{"hook": "ledger.posting", "action": "modify", "data": map[string]any{"n": 1}}})
	if err != nil {
		t.Fatal(err)
	}
	out := resp.Payload.(Response)
	if out.ReturnData.(map[string]any)["n"] != 2 || out.Messages[0].Text != "ledger.posting/modify" {
		t.Fatalf("%+v", out)
	}
}
