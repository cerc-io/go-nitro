package main

import (
	"crypto/tls"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/statechannels/go-nitro/bridge"
	"github.com/statechannels/go-nitro/cmd/utils"
	"github.com/statechannels/go-nitro/internal/logging"
	"github.com/statechannels/go-nitro/internal/rpc"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

const (
	CONFIG = "config"

	L1_CHAIN_URL = "l1chainurl"
	L2_CHAIN_URL = "l2chainurl"

	L1_CHAIN_START_BLOCK = "l1chainstartblock"
	L2_CHAIN_START_BLOCK = "l2chainstartblock"

	CHAIN_PK         = "chainpk"
	STATE_CHANNEL_PK = "statechannelpk"

	NA_ADDRESS         = "naaddress"
	VPA_ADDRESS        = "vpaaddress"
	CA_ADDRESS         = "caaddress"
	BRIDGE_ADDRESS     = "bridgeaddress"
	ASSET_MAP_FILEPATH = "assetmapfilepath"

	DURABLE_STORE_DIR           = "nodel1durablestorefolder"
	NODEL2_DURABLE_STORE_FOLDER = "nodel2durablestorefolder"

	BRIDGE_PUBLIC_IP     = "bridgepublicip"
	NODEL1_EXT_MULTIADDR = "nodel1ExtMultiAddr"
	NODEL2_EXT_MULTIADDR = "nodel2ExtMultiAddr"

	NODEL1_MSG_PORT = "nodel1msgport"
	NODEL2_MSG_PORT = "nodel2msgport"

	RPC_PORT = "rpcport"

	TLS_CERT_FILEPATH = "tlscertfilepath"
	TLS_KEY_FILEPATH  = "tlskeyfilepath"
)

func main() {
	var l1chainurl, l2chainurl, chainpk, statechannelpk, naaddress, vpaaddress, caaddress, bridgeaddress, durableStoreDir, bridgepublicip, nodel1ExtMultiAddr, nodel2ExtMultiAddr string
	var nodel1msgport, nodel2msgport, rpcport int
	var l1chainstartblock, l2chainstartblock uint64

	var tlscertfilepath, tlskeyfilepath string

	var assetsmapfilepath string

	// urfave default precedence for flag value sources (highest to lowest):
	// 1. Command line flag value
	// 2. Environment variable (if specified)
	// 3. Configuration file (if specified)
	// 4. Default defined on the flag

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  CONFIG,
			Usage: "Load config options from `config.toml`",
		},
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        STATE_CHANNEL_PK,
			Usage:       "Specifies the private key used by the nitro node.",
			Destination: &statechannelpk,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        L1_CHAIN_URL,
			Usage:       "Specifies the chain URL of L1",
			Destination: &l1chainurl,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        L2_CHAIN_URL,
			Usage:       "Specifies the chain URL of L2",
			Destination: &l2chainurl,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        CHAIN_PK,
			Usage:       "Specifies the chain private key of bridge",
			Destination: &chainpk,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        NA_ADDRESS,
			Usage:       "Specifies the nitro adjudicator contract address",
			Destination: &naaddress,
			EnvVars:     []string{"NA_ADDRESS"},
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        CA_ADDRESS,
			Usage:       "Specifies the consensus app contract address",
			Destination: &caaddress,
			EnvVars:     []string{"CA_ADDRESS"},
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        VPA_ADDRESS,
			Usage:       "Specifies the virtual payment app contract address",
			Destination: &vpaaddress,
			EnvVars:     []string{"VPA_ADDRESS"},
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        BRIDGE_ADDRESS,
			Usage:       "Specifies the bridge contract address",
			Destination: &bridgeaddress,
			EnvVars:     []string{"BRIDGE_ADDRESS"},
		}),
		altsrc.NewPathFlag(&cli.PathFlag{
			Name:        ASSET_MAP_FILEPATH,
			Usage:       "Filepath to the map of asset address on L1 to asset address of L2",
			Destination: &assetsmapfilepath,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        DURABLE_STORE_DIR,
			Usage:       "Specifies the durable store location of nodes",
			Destination: &durableStoreDir,
			Value:       "./data/bridge-store",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        BRIDGE_PUBLIC_IP,
			Usage:       "Specifies the ip address of node L1 for message service",
			Destination: &bridgepublicip,
			Value:       "127.0.0.1",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        NODEL1_EXT_MULTIADDR,
			Usage:       "Additional external multiaddr to advertise for node L1",
			Value:       "",
			Destination: &nodel1ExtMultiAddr,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        NODEL2_EXT_MULTIADDR,
			Usage:       "Additional external multiaddr to advertise for node L2",
			Value:       "",
			Destination: &nodel2ExtMultiAddr,
		}),
		altsrc.NewIntFlag(&cli.IntFlag{
			Name:        NODEL1_MSG_PORT,
			Usage:       "Specifies the message port of nodeL1 for the message service.",
			Value:       3005,
			Destination: &nodel1msgport,
		}),
		altsrc.NewIntFlag(&cli.IntFlag{
			Name:        NODEL2_MSG_PORT,
			Usage:       "Specifies the message port of nodeL2 for the message service.",
			Value:       3006,
			Destination: &nodel2msgport,
		}),
		altsrc.NewIntFlag(&cli.IntFlag{
			Name:        RPC_PORT,
			Usage:       "Specifies the tcp port for the rpc server.",
			Value:       4007,
			Destination: &rpcport,
		}),
		altsrc.NewUint64Flag(&cli.Uint64Flag{
			Name:        L1_CHAIN_START_BLOCK,
			Usage:       "Specifies the block number to start looking for nitro adjudicator events of nodeL1",
			Value:       0,
			Destination: &l1chainstartblock,
		}),
		altsrc.NewUint64Flag(&cli.Uint64Flag{
			Name:        L2_CHAIN_START_BLOCK,
			Usage:       "Specifies the block number to start looking for nitro adjudicator events of nodeL1",
			Value:       0,
			Destination: &l2chainstartblock,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        TLS_CERT_FILEPATH,
			Usage:       "Filepath to the TLS certificate. If not specified, TLS will not be used with the RPC transport.",
			Value:       "./tls/statechannels.org.pem",
			Destination: &tlscertfilepath,
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:        TLS_KEY_FILEPATH,
			Usage:       "Filepath to the TLS private key. If not specified, TLS will not be used with the RPC transport.",
			Value:       "./tls/statechannels.org_key.pem",
			Destination: &tlskeyfilepath,
		}),
	}

	app := &cli.App{
		Name:   "bridge",
		Usage:  "Nitro as a service. State channel node with RPC server.",
		Flags:  flags,
		Before: altsrc.InitInputSourceWithContext(flags, altsrc.NewTomlSourceFromFlagFunc(CONFIG)),
		Action: func(cCtx *cli.Context) error {
			chainpk = utils.TrimHexPrefix(chainpk)
			statechannelpk = utils.TrimHexPrefix(statechannelpk)

			// Variable to hold the deserialized data
			var assets bridge.L1ToL2AssetConfig

			if assetsmapfilepath != "" {
				tomlFile, err := os.Open(assetsmapfilepath)
				if err != nil {
					return err
				}
				defer tomlFile.Close()

				byteValue, err := io.ReadAll(tomlFile)
				if err != nil {
					return err
				}

				// Deserialize toml file data into the struct
				err = toml.Unmarshal(byteValue, &assets)
				if err != nil {
					return err
				}
			}

			bridgeConfig := bridge.BridgeConfig{
				L1ChainUrl:         l1chainurl,
				L2ChainUrl:         l2chainurl,
				L1ChainStartBlock:  l1chainstartblock,
				L2ChainStartBlock:  l2chainstartblock,
				ChainPK:            chainpk,
				StateChannelPK:     statechannelpk,
				NaAddress:          naaddress,
				VpaAddress:         vpaaddress,
				CaAddress:          caaddress,
				BridgeAddress:      bridgeaddress,
				DurableStoreDir:    durableStoreDir,
				BridgePublicIp:     bridgepublicip,
				NodeL1ExtMultiAddr: nodel1ExtMultiAddr,
				NodeL2ExtMultiAddr: nodel2ExtMultiAddr,
				NodeL1MsgPort:      nodel1msgport,
				NodeL2MsgPort:      nodel2msgport,
				Assets:             assets.Assets,
			}

			logging.SetupDefaultLogger(os.Stdout, slog.LevelDebug)
			bridge := bridge.New()

			bridgeNodeL1Multiaddress, bridgeNodeL2Multiaddress, err := bridge.Start(bridgeConfig)
			if err != nil {
				log.Fatal(err)
			}

			var cert tls.Certificate

			if tlscertfilepath != "" && tlskeyfilepath != "" {
				cert, err = tls.LoadX509KeyPair(tlscertfilepath, tlskeyfilepath)
				if err != nil {
					panic(err)
				}
			}

			rpcServer, err := rpc.InitializeBridgeRpcServer(bridge, rpcport, false, &cert)
			if err != nil {
				return err
			}

			slog.Info("Bridge nodes multiaddresses", "l1 node multiaddress", bridgeNodeL1Multiaddress, "l2 node multiaddress", bridgeNodeL2Multiaddress)
			utils.WaitForKillSignal()

			err = rpcServer.Close()
			if err != nil {
				return err
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
