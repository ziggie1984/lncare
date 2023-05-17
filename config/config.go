package config

import (
	"log"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"
)

// Configuration defines the config for lncare
var Configuration = struct {
	Config                       string `short:"f" long:"config" description:"config file path"`
	Connect                      string `short:"c" long:"connect" description:"connect to lnd using host:port" toml:"connect"`
	TLSCert                      string `short:"t" long:"tlscert" description:"path to tls.cert to connect" required:"false" toml:"tlscert"`
	MacaroonDir                  string `long:"macaroon-dir" description:"path to the macaroon directory" required:"false" toml:"macaroon_dir"`
	MacaroonFilename             string `long:"macaroon-filename" description:"macaroon filename" toml:"macaroon_filename"`
	Network                      string `short:"n" long:"network" description:"bitcoin network to use" toml:"network"`
	DisableChannelLowLocal       bool   `long:"disable-channel-low-local" description:"disables a channel when the localbalance reaches the channelreserve" toml:"disable_channels_low_local"`
	ControlHtlcMaxSize           bool   `long:"control-htlc-max-size" description:"changes the htlcMaxSize according to the localbalance" toml:"disable_channels_low_local"`
	AvoidForcCloseByReconnecting bool   `long:"avoid-force-close-by-reconnecting" description:"avoids unilateral closes by reconnecting in the grace period (only works for lnd peers)" toml:"avoid_force_close_by_reconnecting"`
}{}

func preflightChecks() error {
	if Configuration.Connect == "" {
		Configuration.Connect = "127.0.0.1:10009"
	}
	if Configuration.MacaroonFilename == "" {
		Configuration.MacaroonFilename = "admin.macaroon"
	}
	if Configuration.Network == "" {
		Configuration.Network = "mainnet"
	}

	return nil

}

// LoadConfig the config from the the command-line argument
func loadConfig() {
	flags.NewParser(&Configuration, flags.None).Parse()

	if Configuration.Config == "" {
		return
	}
	if strings.Contains(Configuration.Config, ".toml") {
		_, err := toml.DecodeFile(Configuration.Config, &Configuration)

		if err != nil {
			log.Fatalf("Error opening config file %s: %s", Configuration.Config, err.Error())
		}
	}
}

func init() {
	loadConfig()

	err := preflightChecks()
	if err != nil {
		log.Fatalf("Failed preflightChecks with %s", err)
	}

}
