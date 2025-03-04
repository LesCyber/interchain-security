package keeper

import (
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint" //nolint:golint

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/interchain-security/v7/x/ccv/consumer/types"
	ccvtypes "github.com/cosmos/interchain-security/v7/x/ccv/types"
)

func (k Keeper) GetProviderInfo(ctx sdk.Context) (*types.QueryProviderInfoResponse, error) { //nolint:golint
	consumerChannelID, found := k.GetProviderChannel(ctx)
	if !found {
		return nil, ccvtypes.ErrChannelNotFound
	}
	consumerChannel, found := k.channelKeeper.GetChannel(ctx, ccvtypes.ConsumerPortID, consumerChannelID)
	if !found {
		return nil, ccvtypes.ErrChannelNotFound
	}

	// from channel get connection
	consumerConnectionID, consumerConnection, err := k.channelKeeper.GetChannelConnection(ctx, ccvtypes.ConsumerPortID, consumerChannelID)
	if err != nil {
		return nil, err
	}

	providerChannelID := consumerChannel.Counterparty.ChannelId
	providerConnection := consumerConnection.Counterparty

	consumerClientState, found := k.clientKeeper.GetClientState(ctx, consumerConnection.ClientId)
	if !found {
		return nil, ccvtypes.ErrClientNotFound
	}
	providerChainID := consumerClientState.(*ibctm.ClientState).ChainId

	resp := types.QueryProviderInfoResponse{
		Consumer: types.ChainInfo{
			ChainID:      ctx.ChainID(),
			ClientID:     consumerConnection.ClientId,
			ConnectionID: consumerConnectionID,
			ChannelID:    consumerChannelID,
		},

		Provider: types.ChainInfo{
			ChainID:      providerChainID,
			ClientID:     providerConnection.ClientId,
			ConnectionID: providerConnection.ConnectionId,
			ChannelID:    providerChannelID,
		},
	}

	return &resp, nil
}
