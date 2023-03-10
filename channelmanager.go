package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/ziggie1984/lncare/config"
)

const (
	defaultTimeLockDelta uint32 = 144
)

func parseChanPoint(s string) (*lnrpc.ChannelPoint, error) {
	split := strings.Split(s, ":")
	if len(split) != 2 || len(split[0]) == 0 || len(split[1]) == 0 {
		return nil, fmt.Errorf("error bad channelpoint")
	}

	index, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to decode output index: %v", err)
	}

	txid, err := chainhash.NewHashFromStr(split[0])
	if err != nil {
		return nil, fmt.Errorf("unable to parse hex string: %v", err)
	}

	return &lnrpc.ChannelPoint{
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidBytes{
			FundingTxidBytes: txid[:],
		},
		OutputIndex: uint32(index),
	}, nil
}

// DispatchChannelManager currently evaluates htlcMax size and
// disables channels when there localbalance is lower than 2%*Channelreserve
func (lncare *lncare) DispatchChannelManager(ctx context.Context) {

	if config.Configuration.ControlHtlcMaxSize {
		go func() {
			err := lncare.htlcSizeChanger(ctx)
			if err != nil {
				log.Fatalf("htlcSizeMonitor  error: %v", err)
			}
		}()
	}

	if config.Configuration.DisableChannelLowLocal {
		go func() {
			err := lncare.channelDisabler(ctx)
			if err != nil {
				log.Fatalf("localBalanceMonitor  error: %v", err)
			}
		}()
	}

}

func (lncare *lncare) htlcSizeChanger(ctx context.Context) error {
	log.Printf("htlcSizeChanger started ...")
	var maxHtlcSizeMsat uint64
	for {
		log.Printf("Evaluating HTLC-Limits on Channels")
		err := lncare.getChannels(ctx)
		if err != nil {
			return err
		}

		for _, channel := range lncare.channels {
			chanInfo, err := lncare.getChanInfo(ctx, channel.ChanId)
			if err != nil {
				return err
			}
			nodePolicy := chanInfo.Node1Policy
			if chanInfo.Node1Pub != lncare.myInfo.IdentityPubkey {
				nodePolicy = chanInfo.Node2Policy
			}

			chanPoint, err := parseChanPoint(channel.ChannelPoint)
			if err != nil {
				return err
			}

			exponent := int64(math.Log2(float64((channel.LocalBalance - int64(channel.LocalConstraints.ChanReserveSat)) * 1000)))
			maxHtlcSizeMsat = uint64(math.Pow(2.0, float64(exponent)))

			switch {
			//Only Account for Updates which have a different Timelock or MaxHTLCMsat
			case maxHtlcSizeMsat != nodePolicy.MaxHtlcMsat || nodePolicy.TimeLockDelta != defaultTimeLockDelta:
				req := &lnrpc.PolicyUpdateRequest{
					BaseFeeMsat:   nodePolicy.FeeBaseMsat,
					TimeLockDelta: defaultTimeLockDelta,
					MaxHtlcMsat:   maxHtlcSizeMsat,
					FeeRatePpm:    uint32(nodePolicy.FeeRateMilliMsat),
				}

				req.Scope = &lnrpc.PolicyUpdateRequest_ChanPoint{
					ChanPoint: chanPoint,
				}

				resp, err := lncare.lnd.client.UpdateChannelPolicy(ctx, req)
				if err != nil {
					return err
				}

				for _, protoUpdate := range resp.FailedUpdates {
					fmt.Println(protoUpdate)
				}

				log.Printf("successfully updated chanpolicy for channel(%v): localbalance: %d htlcMax: %d sats => %d sats", channel.ChanId, channel.LocalBalance,
					nodePolicy.MaxHtlcMsat/1000, maxHtlcSizeMsat/1000)
			}
			log.Printf("Evaluating HTLC-Limits Done")
			time.Sleep(60 * time.Minute)
		}
	}
}

func (lncare *lncare) channelDisabler(ctx context.Context) error {
	log.Printf("channelDisabler started ...")
	for {
		log.Printf("Evaluating LocalBalance to disable/enable potential channels")
		err := lncare.getChannels(ctx)
		if err != nil {
			return err
		}

		for _, channel := range lncare.channels {

			chanInfo, err := lncare.getChanInfo(ctx, channel.ChanId)
			if err != nil {
				return err
			}
			nodePolicy := chanInfo.Node1Policy
			if chanInfo.Node1Pub != lncare.myInfo.IdentityPubkey {
				nodePolicy = chanInfo.Node2Policy
			}

			chanPoint, err := parseChanPoint(channel.ChannelPoint)
			if err != nil {
				return err
			}

			var action routerrpc.ChanStatusAction
			switch {
			//We do nothing in case the channel is enabled and its limits are above the channelreserve
			case uint64(channel.LocalBalance) > channel.LocalConstraints.ChanReserveSat*2 && nodePolicy.Disabled && channel.Active:
				action = routerrpc.ChanStatusAction_ENABLE
				req := &routerrpc.UpdateChanStatusRequest{
					ChanPoint: chanPoint,
					Action:    action}

				_, err = lncare.lnd.router.UpdateChanStatus(ctx, req)
				if err != nil {
					log.Printf("Error enabling channel(%v) with %s", channel.ChanId, err)
				} else {
					log.Printf("channel(%v) enabling channel", channel.ChanId)
				}

			case uint64(channel.LocalBalance) < channel.LocalConstraints.ChanReserveSat*2 && !nodePolicy.Disabled:
				action = routerrpc.ChanStatusAction_DISABLE
				log.Printf("channel(%v) disabling channel because localbalance is too low", channel.ChanId)
				req := &routerrpc.UpdateChanStatusRequest{
					ChanPoint: chanPoint,
					Action:    action}

				_, err = lncare.lnd.router.UpdateChanStatus(ctx, req)

			}

		}
		log.Printf("Evaluating LocalBalance Done")
		time.Sleep(60 * time.Second)
	}
}
