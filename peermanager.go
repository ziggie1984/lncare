package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/ziggie1984/lncare/config"
)

const (
	frequency int = 10 //10 minutes
)

// DispatchChannelManager currently evaluates htlcMax size and
// disables channels when there localbalance is lower than 2%*Channelreserve
func (lncare *lncare) DispatchPeerManager(ctx context.Context) {

	if config.Configuration.AvoidForcCloseByReconnecting {
		go func() {
			err := lncare.reconnectPeers(ctx)
			if err != nil {
				log.Fatalf("htlcSizeMonitor  error: %v", err)
			}
		}()
	}
}

func (lncare *lncare) reconnectPeers(ctx context.Context) error {
	log.Printf("reconnectPeers started ...")
	for {
		log.Printf("Checking for HTLCs in the grace period and triggering a reconnect")
		err := lncare.getChannels(ctx)
		if err != nil {
			return err
		}

		nodeinfo, err := lncare.lnd.getMyInfo(ctx)
		if err != nil {
			return err
		}

		lncare.disabledChannels = make(map[uint64]Channel)

		for _, channel := range lncare.channels {

			//We check that the peer is a LND node by checking if they have the AMP featue set
			// Now immediatly reconnect
			remoteNodeInfo, err := lncare.lnd.getNodeInfo(ctx, channel.RemotePubkey)
			if err != nil {
				// we break if we cannot fetch node infos
				continue
			}
			// exit early if the peer is not an lnd node.
			if _, ok := remoteNodeInfo.Node.Features[uint32(lnwire.AMPOptional)]; !ok {
				continue
			}

			for _, htlc := range channel.PendingHtlcs {
				if nodeinfo.BlockHeight >= htlc.ExpirationHeight-lncfg.DefaultFinalCltvRejectDelta {
					lncare.disabledChannels[channel.ChanId] = Channel{channel, true}
					log.Printf("Channel has an HTLC %s which is in the grace period we should reconnet "+
						"and also disable it: Blockheight %d, Expiration %d\n", hex.EncodeToString(htlc.HashLock), nodeinfo.BlockHeight, htlc.ExpirationHeight)

					// We are not going to reconnect the peer to exploit unintended behaviour of the lnd code, where
					// during replay of Incoming HTLC packages are failed back no matter the outgoing HTLC is still not resolved.
					log.Printf("Disconnecting peer with pubkey: %v and alias:%v\n", channel.RemotePubkey, channel.AliasScids)
					_, err := lncare.lnd.client.DisconnectPeer(ctx, &lnrpc.DisconnectPeerRequest{PubKey: channel.RemotePubkey})
					if err != nil {
						log.Printf("Error unable to disconnect peer with: %v", err)
					}

					var host string
					for _, addr := range remoteNodeInfo.Node.Addresses {
						if validateIPv4(addr.Addr) {
							host = addr.Addr
							break
						}

						if host == "" {
							host = addr.Addr
						}
					}
					// This should never happen
					if host == "" {
						break
					}
					log.Printf("Connecting peer with pubkey: %v and alias:%v\n", channel.RemotePubkey, channel.AliasScids)
					//We ignore the error and just break out of this loop continueing with the next peer
					// Lnd should eventually reconnect this channel.
					_, err = lncare.lnd.client.ConnectPeer(ctx, &lnrpc.ConnectPeerRequest{
						Addr:    &lnrpc.LightningAddress{Pubkey: channel.RemotePubkey, Host: host},
						Timeout: 10,
					})
					if err != nil {
						log.Printf("Error unable to connect peer with: %v", err)

					}
					// We break out of the HTLC loop one expired HTLC is enough to make the decision.
					break
				}
			}

		}

		log.Printf("Evaluating reconnectPeers Done\n")
		log.Printf("ReconnectPeers Sleeping for %v minutes", frequency)
		time.Sleep(time.Duration(frequency) * time.Minute)
	}
}

// validateIPv4 checks if a string is a valid IPv4 address
func validateIPv4(ip string) bool {
	// Regular expression pattern for IPv4 address
	ipv4Pattern := `^(\d{1,3}\.){3}\d{1,3}`

	match, err := regexp.MatchString(ipv4Pattern, ip)
	if err != nil {
		// Handle any error that occurred during pattern matching
		fmt.Println("Error analysing IPv4 string:", err)
		return false
	}

	return match
}
