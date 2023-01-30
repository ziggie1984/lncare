package main

import (
	"context"

	"github.com/lightningnetwork/lnd/lnrpc"
)

func (lncare *lncare) getChannels(ctx context.Context) error {
	// ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(params.TimeoutRoute))
	// defer cancel()
	channels, err := lncare.lnd.client.ListChannels(ctx, &lnrpc.ListChannelsRequest{ActiveOnly: false, PublicOnly: true})
	if err != nil {
		return err
	}
	lncare.channels = channels.Channels
	return nil
}

func (lncare *lncare) getChanInfo(ctx context.Context, chanID uint64) (*lnrpc.ChannelEdge, error) {
	c, err := lncare.lnd.client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{ChanId: chanID})
	if err != nil {
		return nil, err
	}
	return c, nil
}
