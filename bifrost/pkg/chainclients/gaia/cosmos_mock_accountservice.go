package gaia

import (
	"context"
	"fmt"

	atypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	grpc "google.golang.org/grpc"
)

// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type MockAccountServiceClient interface {
	// Accounts returns all the existing accounts
	//
	// Since: cosmos-sdk 0.43
	Accounts(ctx context.Context, in *atypes.QueryAccountsRequest, opts ...grpc.CallOption) (*atypes.QueryAccountsResponse, error)
	// Account returns account details based on address.
	Account(ctx context.Context, in *atypes.QueryAccountRequest, opts ...grpc.CallOption) (*atypes.QueryAccountResponse, error)
	// Params queries all parameters.
	Params(ctx context.Context, in *atypes.QueryParamsRequest, opts ...grpc.CallOption) (*atypes.QueryParamsResponse, error)
}

type mockAccountServiceClient struct{}

func NewMockAccountServiceClient() MockAccountServiceClient {
	return &mockAccountServiceClient{}
}

func (c *mockAccountServiceClient) Account(ctx context.Context, in *atypes.QueryAccountRequest, opts ...grpc.CallOption) (*atypes.QueryAccountResponse, error) {
	out := new(atypes.QueryAccountResponse)
	err := unmarshalJSONToPb("./test-data/account_by_address.json", out)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block by height: %s", err)
	}
	return out, nil
}

func (c *mockAccountServiceClient) Accounts(ctx context.Context, in *atypes.QueryAccountsRequest, opts ...grpc.CallOption) (*atypes.QueryAccountsResponse, error) {
	return nil, nil
}

func (c *mockAccountServiceClient) Params(ctx context.Context, in *atypes.QueryParamsRequest, opts ...grpc.CallOption) (*atypes.QueryParamsResponse, error) {
	return nil, nil
}
