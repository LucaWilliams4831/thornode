package gaia

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	grpc "google.golang.org/grpc"
)

// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type MockTmServiceClient interface {
	// GetNodeInfo queries the current node info.
	GetNodeInfo(ctx context.Context, in *tmservice.GetNodeInfoRequest, opts ...grpc.CallOption) (*tmservice.GetNodeInfoResponse, error)
	// GetSyncing queries node syncing.
	GetSyncing(ctx context.Context, in *tmservice.GetSyncingRequest, opts ...grpc.CallOption) (*tmservice.GetSyncingResponse, error)
	// GetLatestBlock returns the latest block.
	GetLatestBlock(ctx context.Context, in *tmservice.GetLatestBlockRequest, opts ...grpc.CallOption) (*tmservice.GetLatestBlockResponse, error)
	// GetBlockByHeight queries block for given height.
	GetBlockByHeight(ctx context.Context, in *tmservice.GetBlockByHeightRequest, opts ...grpc.CallOption) (*tmservice.GetBlockByHeightResponse, error)
	// GetLatestValidatorSet queries latest validator-set.
	GetLatestValidatorSet(ctx context.Context, in *tmservice.GetLatestValidatorSetRequest, opts ...grpc.CallOption) (*tmservice.GetLatestValidatorSetResponse, error)
	// GetValidatorSetByHeight queries validator-set at a given height.
	GetValidatorSetByHeight(ctx context.Context, in *tmservice.GetValidatorSetByHeightRequest, opts ...grpc.CallOption) (*tmservice.GetValidatorSetByHeightResponse, error)
}

type mockTmServiceClient struct{}

func NewMockTmServiceClient() MockTmServiceClient {
	return &mockTmServiceClient{}
}

func (m *mockTmServiceClient) GetNodeInfo(ctx context.Context, in *tmservice.GetNodeInfoRequest, opts ...grpc.CallOption) (*tmservice.GetNodeInfoResponse, error) {
	return nil, nil
}

func (m *mockTmServiceClient) GetSyncing(ctx context.Context, in *tmservice.GetSyncingRequest, opts ...grpc.CallOption) (*tmservice.GetSyncingResponse, error) {
	return nil, nil
}

func (m *mockTmServiceClient) GetLatestBlock(ctx context.Context, in *tmservice.GetLatestBlockRequest, opts ...grpc.CallOption) (*tmservice.GetLatestBlockResponse, error) {
	out := new(tmservice.GetLatestBlockResponse)
	err := unmarshalJSONToPb("./test-data/latest_block.json", out)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block by height: %s", err)
	}
	return out, nil
}

func (m *mockTmServiceClient) GetBlockByHeight(ctx context.Context, in *tmservice.GetBlockByHeightRequest, opts ...grpc.CallOption) (*tmservice.GetBlockByHeightResponse, error) {
	out := new(tmservice.GetBlockByHeightResponse)
	err := unmarshalJSONToPb("./test-data/block_by_height.json", out)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block by height: %s", err)
	}
	return out, nil
}

func (m *mockTmServiceClient) GetLatestValidatorSet(ctx context.Context, in *tmservice.GetLatestValidatorSetRequest, opts ...grpc.CallOption) (*tmservice.GetLatestValidatorSetResponse, error) {
	return nil, nil
}

func (m *mockTmServiceClient) GetValidatorSetByHeight(ctx context.Context, in *tmservice.GetValidatorSetByHeightRequest, opts ...grpc.CallOption) (*tmservice.GetValidatorSetByHeightResponse, error) {
	return nil, nil
}
