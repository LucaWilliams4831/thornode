package gaia

import (
	"context"
	"fmt"

	btypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	grpc "google.golang.org/grpc"
)

// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type MockBankServiceClient interface {
	// Balance queries the balance of a single coin for a single account.
	Balance(ctx context.Context, in *btypes.QueryBalanceRequest, opts ...grpc.CallOption) (*btypes.QueryBalanceResponse, error)
	// AllBalances queries the balance of all coins for a single account.
	AllBalances(ctx context.Context, in *btypes.QueryAllBalancesRequest, opts ...grpc.CallOption) (*btypes.QueryAllBalancesResponse, error)
	// TotalSupply queries the total supply of all coins.
	TotalSupply(ctx context.Context, in *btypes.QueryTotalSupplyRequest, opts ...grpc.CallOption) (*btypes.QueryTotalSupplyResponse, error)
	// SupplyOf queries the supply of a single coin.
	SupplyOf(ctx context.Context, in *btypes.QuerySupplyOfRequest, opts ...grpc.CallOption) (*btypes.QuerySupplyOfResponse, error)
	// Params queries the parameters of x/bank module.
	Params(ctx context.Context, in *btypes.QueryParamsRequest, opts ...grpc.CallOption) (*btypes.QueryParamsResponse, error)
	// DenomsMetadata queries the client metadata of a given coin denomination.
	DenomMetadata(ctx context.Context, in *btypes.QueryDenomMetadataRequest, opts ...grpc.CallOption) (*btypes.QueryDenomMetadataResponse, error)
	// DenomsMetadata queries the client metadata for all registered coin denominations.
	DenomsMetadata(ctx context.Context, in *btypes.QueryDenomsMetadataRequest, opts ...grpc.CallOption) (*btypes.QueryDenomsMetadataResponse, error)
}

type mockBankServiceClient struct{}

func NewMockBankServiceClient() MockBankServiceClient {
	return &mockBankServiceClient{}
}

func (c *mockBankServiceClient) Balance(ctx context.Context, in *btypes.QueryBalanceRequest, opts ...grpc.CallOption) (*btypes.QueryBalanceResponse, error) {
	return nil, nil
}

func (c *mockBankServiceClient) AllBalances(ctx context.Context, in *btypes.QueryAllBalancesRequest, opts ...grpc.CallOption) (*btypes.QueryAllBalancesResponse, error) {
	out := new(btypes.QueryAllBalancesResponse)
	err := unmarshalJSONToPb("./test-data/all_balances_by_address.json", out)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block by height: %s", err)
	}
	return out, nil
}

func (c *mockBankServiceClient) TotalSupply(ctx context.Context, in *btypes.QueryTotalSupplyRequest, opts ...grpc.CallOption) (*btypes.QueryTotalSupplyResponse, error) {
	return nil, nil
}

func (c *mockBankServiceClient) SupplyOf(ctx context.Context, in *btypes.QuerySupplyOfRequest, opts ...grpc.CallOption) (*btypes.QuerySupplyOfResponse, error) {
	return nil, nil
}

func (c *mockBankServiceClient) Params(ctx context.Context, in *btypes.QueryParamsRequest, opts ...grpc.CallOption) (*btypes.QueryParamsResponse, error) {
	return nil, nil
}

func (c *mockBankServiceClient) DenomMetadata(ctx context.Context, in *btypes.QueryDenomMetadataRequest, opts ...grpc.CallOption) (*btypes.QueryDenomMetadataResponse, error) {
	return nil, nil
}

func (c *mockBankServiceClient) DenomsMetadata(ctx context.Context, in *btypes.QueryDenomsMetadataRequest, opts ...grpc.CallOption) (*btypes.QueryDenomsMetadataResponse, error) {
	return nil, nil
}
