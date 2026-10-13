package grpcserver

import (
	"context"

	subscriptionv1 "rangoapp/internal/easystock/gen/subscriptionv1"
	"rangoapp/internal/easystock/subscription"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SubscriptionServer exposes plan state and local payment confirmation.
type SubscriptionServer struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	svc *subscription.Service
}

func NewSubscription(svc *subscription.Service) *SubscriptionServer {
	return &SubscriptionServer{svc: svc}
}

func (s *SubscriptionServer) GetSubscription(ctx context.Context, _ *subscriptionv1.GetSubscriptionRequest) (*subscriptionv1.GetSubscriptionResponse, error) {
	p, err := Principal(ctx)
	if err != nil {
		return nil, err
	}
	view, err := s.svc.Get(ctx, p)
	if err != nil {
		return nil, toStatus(err)
	}
	return &subscriptionv1.GetSubscriptionResponse{
		PlanCode: view.PlanCode, Status: view.Status, Source: view.Source,
	}, nil
}

func (s *SubscriptionServer) ConfirmLocalPayment(ctx context.Context, req *subscriptionv1.ConfirmLocalPaymentRequest) (*subscriptionv1.ConfirmLocalPaymentResponse, error) {
	p, err := Principal(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetPaymentId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Identifiant de paiement invalide.")
	}
	if err := s.svc.ConfirmLocalPayment(ctx, p, id); err != nil {
		return nil, toStatus(err)
	}
	view, err := s.svc.Get(ctx, p)
	if err != nil {
		return nil, toStatus(err)
	}
	return &subscriptionv1.ConfirmLocalPaymentResponse{Status: view.Status, Source: view.Source}, nil
}
