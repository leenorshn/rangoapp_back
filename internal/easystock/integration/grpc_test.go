package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/contexttoken"
	commercev1 "rangoapp/internal/easystock/gen/commercev1"
	"rangoapp/internal/easystock/grpcserver"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const contextSecret = "test-secret-test-secret-test-secret"

func startCommerce(t *testing.T, svc *commerce.Service) commercev1.CommerceServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(1<<20),
		grpc.MaxSendMsgSize(1<<20),
		grpc.ChainUnaryInterceptor(
			grpcserver.RecoveryInterceptor,
			grpcserver.LoggingInterceptor,
			grpcserver.AuthInterceptor(contextSecret),
		),
	)
	commercev1.RegisterCommerceServiceServer(server, grpcserver.NewCommerce(svc))
	go server.Serve(lis)
	t.Cleanup(server.GracefulStop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return commercev1.NewCommerceServiceClient(conn)
}

func TestGRPCAuthDeadlineAndReplay(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	client := startCommerce(t, commerce.New(app))
	ctx := context.Background()
	_, err := client.ConfirmSale(ctx, &commercev1.ConfirmSaleRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated code = %s", status.Code(err))
	}
	token, err := contexttoken.Sign(contextSecret, w.p, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	authed := metadata.AppendToOutgoingContext(ctx, "x-easystock-context", token, "x-company-id", uuid.NewString())
	_, err = client.ConfirmSale(authed, &commercev1.ConfirmSaleRequest{IdempotencyKey: "x", StoreId: w.store.String(), Currency: "USD"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("forged company code = %s", status.Code(err))
	}
	req := &commercev1.ConfirmSaleRequest{
		IdempotencyKey: "grpc-sale",
		StoreId:        w.store.String(),
		CustomerId:     w.customer.String(),
		Currency:       "USD",
		Lines: []*commercev1.SaleLine{{
			ProductId: w.product.String(), UnitName: "piece", Quantity: "1", ExpectedUnitPrice: "10",
		}},
		Payments: []*commercev1.Payment{{Method: "external", Amount: "10"}},
	}
	call := metadata.AppendToOutgoingContext(ctx, "x-easystock-context", token)
	first, err := client.ConfirmSale(call, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.ConfirmSale(call, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.GetSaleId() == "" || first.GetSaleId() != second.GetSaleId() {
		t.Fatalf("replay ids %s %s", first.GetSaleId(), second.GetSaleId())
	}
	canceled, cancel := context.WithCancel(call)
	cancel()
	_, err = client.ConfirmSale(canceled, req)
	if status.Code(err) != codes.Canceled && status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("canceled code = %s %v", status.Code(err), err)
	}
}
