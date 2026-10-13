package grpcserver

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/contexttoken"
	"rangoapp/internal/easystock/domain"
	commercev1 "rangoapp/internal/easystock/gen/commercev1"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type principalKey struct{}

// CommerceServer adapts the transactional service to gRPC.
type CommerceServer struct {
	commercev1.UnimplementedCommerceServiceServer
	svc *commerce.Service
}

func NewCommerce(svc *commerce.Service) *CommerceServer {
	return &CommerceServer{svc: svc}
}

func (s *CommerceServer) ConfirmSale(ctx context.Context, req *commercev1.ConfirmSaleRequest) (*commercev1.ConfirmSaleResponse, error) {
	p, err := Principal(ctx)
	if err != nil {
		return nil, err
	}
	in, err := saleInput(req)
	if err != nil {
		return nil, err
	}
	out, err := s.svc.ConfirmSale(ctx, p, in)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &commercev1.ConfirmSaleResponse{
		SaleId: out.SaleID.String(), Currency: out.Currency,
		NetAmount: out.Net, PaidAmount: out.Paid, DueAmount: out.Due,
	}
	if out.DebtID != nil {
		resp.DebtId = out.DebtID.String()
	}
	return resp, nil
}

func (s *CommerceServer) PayDebt(ctx context.Context, req *commercev1.PayDebtRequest) (*commercev1.PayDebtResponse, error) {
	p, err := Principal(ctx)
	if err != nil {
		return nil, err
	}
	debtID, err := uuid.Parse(req.GetDebtId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Identifiant de dette invalide.")
	}
	amount, err := decimal.NewFromString(req.GetAmount())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Montant invalide.")
	}
	in := commerce.PayDebtInput{
		IdempotencyKey: req.GetIdempotencyKey(), DebtID: debtID, Amount: amount, Method: req.GetMethod(),
		ExternalReference: req.GetExternalReference(),
	}
	if req.GetCashSessionId() != "" {
		id, err := uuid.Parse(req.GetCashSessionId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Session de caisse invalide.")
		}
		in.CashSessionID = &id
	}
	out, err := s.svc.PayDebt(ctx, p, in)
	if err != nil {
		return nil, toStatus(err)
	}
	return &commercev1.PayDebtResponse{DebtId: out.DebtID, DueAmount: out.Due, PaidAmount: out.Paid}, nil
}

func (s *CommerceServer) CreateRefund(ctx context.Context, req *commercev1.CreateRefundRequest) (*commercev1.CreateRefundResponse, error) {
	p, err := Principal(ctx)
	if err != nil {
		return nil, err
	}
	saleID, err := uuid.Parse(req.GetSaleId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Identifiant de vente invalide.")
	}
	in := commerce.CreateRefundInput{IdempotencyKey: req.GetIdempotencyKey(), SaleID: saleID, Reason: req.GetReason()}
	if req.GetCashSessionId() != "" {
		id, err := uuid.Parse(req.GetCashSessionId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Session de caisse invalide.")
		}
		in.CashSessionID = &id
	}
	for _, line := range req.GetLines() {
		itemID, err := uuid.Parse(line.GetSaleItemId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Ligne de vente invalide.")
		}
		qty, err := decimal.NewFromString(line.GetQuantity())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "Quantité invalide.")
		}
		in.Lines = append(in.Lines, commerce.RefundLineInput{SaleItemID: itemID, Quantity: qty, Restock: line.GetRestock()})
	}
	out, err := s.svc.CreateRefund(ctx, p, in)
	if err != nil {
		return nil, toStatus(err)
	}
	return &commercev1.CreateRefundResponse{
		RefundId: out.RefundID, Amount: out.Amount, CashAmount: out.CashAmount, DebtAmount: out.DebtAmount,
	}, nil
}

func saleInput(req *commercev1.ConfirmSaleRequest) (commerce.ConfirmSaleInput, error) {
	storeID, err := uuid.Parse(req.GetStoreId())
	if err != nil {
		return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Boutique invalide.")
	}
	in := commerce.ConfirmSaleInput{
		IdempotencyKey: req.GetIdempotencyKey(),
		StoreID:        storeID,
		Currency:       req.GetCurrency(),
		ClientDraftID:  req.GetClientDraftId(),
	}
	if req.GetCustomerId() != "" {
		id, err := uuid.Parse(req.GetCustomerId())
		if err != nil {
			return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Client invalide.")
		}
		in.CustomerID = &id
	}
	if req.GetCashSessionId() != "" {
		id, err := uuid.Parse(req.GetCashSessionId())
		if err != nil {
			return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Session de caisse invalide.")
		}
		in.CashSessionID = &id
	}
	if req.GetDiscountAmount() != "" {
		in.DiscountAmount, err = decimal.NewFromString(req.GetDiscountAmount())
		if err != nil {
			return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Remise invalide.")
		}
	}
	for _, line := range req.GetLines() {
		productID, err := uuid.Parse(line.GetProductId())
		if err != nil {
			return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Article invalide.")
		}
		qty, err := decimal.NewFromString(line.GetQuantity())
		if err != nil {
			return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Quantité invalide.")
		}
		price, err := decimal.NewFromString(line.GetExpectedUnitPrice())
		if err != nil {
			return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Prix invalide.")
		}
		in.Lines = append(in.Lines, commerce.SaleLineInput{
			ProductID: productID, UnitName: line.GetUnitName(), Quantity: qty, ExpectedUnitPrice: price,
		})
	}
	for _, payment := range req.GetPayments() {
		amount, err := decimal.NewFromString(payment.GetAmount())
		if err != nil {
			return commerce.ConfirmSaleInput{}, status.Error(codes.InvalidArgument, "Montant invalide.")
		}
		in.Payments = append(in.Payments, commerce.PaymentInput{
			Method: payment.GetMethod(), Amount: amount, ExternalReference: payment.GetExternalReference(),
		})
	}
	return in, nil
}

func Principal(ctx context.Context) (tenant.Principal, error) {
	p, ok := ctx.Value(principalKey{}).(tenant.Principal)
	if !ok {
		return tenant.Principal{}, status.Error(codes.Unauthenticated, "Contexte d'appel absent.")
	}
	return p, nil
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "Le délai de l'opération est dépassé.")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "L'opération a été annulée.")
	}
	if _, ok := status.FromError(err); ok && status.Code(err) != codes.Unknown {
		return err
	}
	if de, ok := domain.As(err); ok {
		return status.Error(codeFor(de.Code), de.Message)
	}
	return status.Error(codes.Internal, "L'opération n'a pas pu être enregistrée.")
}

func codeFor(code string) codes.Code {
	switch code {
	case domain.CodeUnauthenticated:
		return codes.Unauthenticated
	case domain.CodePermissionDenied:
		return codes.PermissionDenied
	case domain.CodeInvalid:
		return codes.InvalidArgument
	case domain.CodeNotFound:
		return codes.NotFound
	case domain.CodeAlreadyExists:
		return codes.AlreadyExists
	case domain.CodeFailedPrecondition:
		return codes.FailedPrecondition
	case domain.CodeConflict:
		return codes.Aborted
	default:
		return codes.Internal
	}
}

// AuthInterceptor rejects calls without a signed context and ignores a conflicting company metadata value.
func AuthInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == "/grpc.health.v1.Health/Check" || info.FullMethod == "/grpc.health.v1.Health/Watch" {
			return handler(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		raw := first(md.Get("x-easystock-context"))
		p, err := contexttoken.Verify(secret, raw)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "Appel interne non authentifié.")
		}
		if claimed := first(md.Get("x-company-id")); claimed != "" && claimed != p.CompanyID.String() {
			return nil, status.Error(codes.PermissionDenied, "Le contexte d'entreprise ne correspond pas à la session.")
		}
		return handler(context.WithValue(ctx, principalKey{}, p), req)
	}
}

func LoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	slog.Info("grpc", "method", info.FullMethod, "code", status.Code(err).String(), "ms", time.Since(start).Milliseconds())
	return resp, err
}

func RecoveryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("grpc panic", "stack", string(debug.Stack()))
			resp = nil
			err = status.Error(codes.Internal, "L'opération n'a pas pu être enregistrée.")
		}
	}()
	return handler(ctx, req)
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
