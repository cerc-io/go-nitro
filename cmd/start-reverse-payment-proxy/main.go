package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/statechannels/go-nitro/cmd/utils"
	"github.com/statechannels/go-nitro/internal/logging"
	"github.com/statechannels/go-nitro/reverseproxy"
	"github.com/urfave/cli/v2"
)

const (
	NITRO_ENDPOINT     = "nitroendpoint"
	PROXY_ADDRESS      = "proxyaddress"
	DESTINATION_URL    = "destinationurl"
	COST_PER_BYTE      = "costperbyte"
	FILTER_RPC_METHODS = "filterrpcmethods"
)

func main() {
	var rProxy *reverseproxy.ReversePaymentProxy
	app := &cli.App{
		Name:  "start-reverse-payment-proxy",
		Usage: "Runs an HTTP payment proxy that charges for HTTP requests",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    PROXY_ADDRESS,
				Usage:   "Specifies the TCP address for the proxy to listen on for requests. This should be in the form 'host:port'",
				Value:   "localhost:5511",
				Aliases: []string{"p"},
			},
			&cli.StringFlag{
				Name:    NITRO_ENDPOINT,
				Usage:   "Specifies the endpoint of the Nitro RPC server to connect to. This should be in the form 'host:port/api/v1'",
				Value:   "localhost:4007/api/v1",
				Aliases: []string{"n"},
			},
			&cli.StringFlag{
				Name:    DESTINATION_URL,
				Usage:   "Specifies the destination URL to forward requests to. It should be a fully qualified URL, including the protocol (e.g. http://localhost:8081)",
				Value:   "http://localhost:8081",
				Aliases: []string{"d"},
			},
			&cli.Uint64Flag{
				Name:    COST_PER_BYTE,
				Usage:   "Specifies the amount of wei that the proxy should charge per byte of the response body",
				Value:   1,
				Aliases: []string{"c"},
			},
			&cli.BoolFlag{
				Name:    FILTER_RPC_METHODS,
				Usage:   "Flag to enable/disable filtering RPC methods that require payment",
				Value:   false,
				Aliases: []string{"r"},
			},
		},
		Action: func(c *cli.Context) error {
			proxyEndpoint := c.String(PROXY_ADDRESS)
			nitroEndpoint := c.String(NITRO_ENDPOINT)

			logging.SetupDefaultLogger(os.Stdout, slog.LevelDebug)

			rProxy = reverseproxy.NewReversePaymentProxy(
				proxyEndpoint,
				nitroEndpoint,
				c.String(DESTINATION_URL),
				c.Uint64(COST_PER_BYTE),
				c.Bool(FILTER_RPC_METHODS),
			)

			return rProxy.Start()
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
	utils.WaitForKillSignal()
	if rProxy != nil {
		err := rProxy.Stop()
		if err != nil {
			log.Fatal(err)
		}
	}
}
