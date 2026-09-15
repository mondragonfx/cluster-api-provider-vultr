package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/vultr/govultr/v3"
)

type fakeLBService struct {
	govultr.LoadBalancerService

	lb      *govultr.LoadBalancer
	getCode int
	updates []*govultr.LoadBalancerReq
}

func (f *fakeLBService) Get(_ context.Context, _ string) (*govultr.LoadBalancer, *http.Response, error) {
	if f.lb == nil {
		return nil, &http.Response{StatusCode: f.getCode}, errors.New(`{"error":"not found","status":404}`)
	}
	return f.lb, &http.Response{StatusCode: http.StatusOK}, nil
}

func (f *fakeLBService) Update(_ context.Context, _ string, req *govultr.LoadBalancerReq) error {
	f.updates = append(f.updates, req)
	f.lb.Instances = req.Instances
	return nil
}

func newLBTestService(t *testing.T, lb *fakeLBService) *Service {
	t.Helper()
	svc, _ := newTestService(t, &fakeBareMetalService{})
	svc.scope.LoadBalancers = lb
	return svc
}

func TestEnsureLoadBalancerMember(t *testing.T) {
	t.Run("not active yet", func(t *testing.T) {
		lb := &fakeLBService{lb: &govultr.LoadBalancer{Status: "pending"}}
		svc := newLBTestService(t, lb)
		member, err := svc.EnsureLoadBalancerMember("lb", "srv")
		if err != nil || member || len(lb.updates) != 0 {
			t.Fatalf("member=%v err=%v updates=%d", member, err, len(lb.updates))
		}
	})
	t.Run("appends once and keeps existing members", func(t *testing.T) {
		lb := &fakeLBService{lb: &govultr.LoadBalancer{Status: testActive, Instances: []string{"other"}}}
		svc := newLBTestService(t, lb)
		for i := 0; i < 2; i++ {
			member, err := svc.EnsureLoadBalancerMember("lb", "srv")
			if err != nil || !member {
				t.Fatalf("iteration %d: member=%v err=%v", i, member, err)
			}
		}
		if len(lb.updates) != 1 || len(lb.updates[0].Instances) != 2 || lb.updates[0].Instances[1] != "srv" {
			t.Fatalf("unexpected updates: %+v", lb.updates)
		}
	})
	t.Run("empty lb id", func(t *testing.T) {
		svc := newLBTestService(t, &fakeLBService{})
		if _, err := svc.EnsureLoadBalancerMember("", "srv"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRemoveLoadBalancerMember(t *testing.T) {
	t.Run("removes only the member", func(t *testing.T) {
		lb := &fakeLBService{lb: &govultr.LoadBalancer{Status: testActive, Instances: []string{"a", "srv", "b"}}}
		svc := newLBTestService(t, lb)
		if err := svc.RemoveLoadBalancerMember("lb", "srv"); err != nil {
			t.Fatal(err)
		}
		if len(lb.updates) != 1 || len(lb.updates[0].Instances) != 2 {
			t.Fatalf("unexpected updates: %+v", lb.updates)
		}
	})
	t.Run("not a member is a no-op", func(t *testing.T) {
		lb := &fakeLBService{lb: &govultr.LoadBalancer{Status: testActive, Instances: []string{"a"}}}
		svc := newLBTestService(t, lb)
		if err := svc.RemoveLoadBalancerMember("lb", "srv"); err != nil || len(lb.updates) != 0 {
			t.Fatalf("err=%v updates=%d", err, len(lb.updates))
		}
	})
	t.Run("missing load balancer is not an error", func(t *testing.T) {
		svc := newLBTestService(t, &fakeLBService{getCode: http.StatusNotFound})
		if err := svc.RemoveLoadBalancerMember("lb", "srv"); err != nil {
			t.Fatal(err)
		}
	})
}
