package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/jessevdk/go-flags"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/ziggie1984/lncare/config"
)

type key int

const (
	ctxKeyWaitGroup key = iota
)

type lncare struct {
	lnd    LndClient
	myInfo *lnrpc.GetInfoResponse

	channels     []*lnrpc.Channel
	channelPairs map[string][2]*lnrpc.Channel
	chanCache    map[uint64]*lnrpc.ChannelEdge
	excludeTo    map[uint64]struct{}
	excludeFrom  map[uint64]struct{}
	excludeBoth  map[uint64]struct{}
}

func newLncareInstance(ctx context.Context, lnd *LndClient) *lncare {
	myInfo, err := lnd.getMyInfo(ctx)
	if err != nil {
		log.Fatalf("Could not get my node info: %s", err)
	}
	return &lncare{
		lnd:    *lnd,
		myInfo: myInfo,
	}
}

func welcome() {
	log.Println("---- ⚡️ Running lncare ----")
	log.Println("--------------------------------------")

}

func newLndClient(ctx context.Context) (*LndClient, error) {
	conn, err := lndclient.NewBasicConn(config.Configuration.Connect, config.Configuration.TLSCert, config.Configuration.MacaroonDir, config.Configuration.Network,
		lndclient.MacFilename(config.Configuration.MacaroonFilename))
	if err != nil {
		log.Fatalf("Connection failed: %s", err)
		return &LndClient{}, err
	}
	client := lnrpc.NewLightningClient(conn)
	router := routerrpc.NewRouterClient(conn)
	return &LndClient{
		client: client,
		conn:   conn,
		router: router,
	}, nil
}

func main() {
	parser := flags.NewParser(&config.Configuration, flags.Default)
	_, err := parser.Parse()

	if err != nil {
		// This prevents and error message when using the help flags
		switch t := err.(type) {
		case *flags.Error:
			if t.Type != 5 {
				log.Fatalf("Error when parsing command line options: %s", err)
			} else {
				os.Exit(1)
			}

		default:
			log.Fatalf("Unexpected error when parsing command line option with : %s", err)
		}
	}

	welcome()
	ctx := context.Background()
	for {

		lnd, err := newLndClient(ctx)
		if err != nil {
			log.Fatalf("Failed to create lnd client: %s", err)
			return
		}

		lncare := newLncareInstance(ctx, lnd)

		if len(lncare.myInfo.Alias) > 0 {
			log.Printf("Connected to %s (%s)", lncare.myInfo.Alias, trimPubKey([]byte(lncare.myInfo.IdentityPubkey)))
		} else {
			log.Printf("Connected to %s", lncare.myInfo.IdentityPubkey)
		}

		var wg sync.WaitGroup
		ctx = context.WithValue(ctx, ctxKeyWaitGroup, &wg)
		wg.Add(1)

		lncare.DispatchChannelManager(ctx)

		wg.Wait()
		log.Println("All routines stopped. Waiting for new connection.")
	}

}
