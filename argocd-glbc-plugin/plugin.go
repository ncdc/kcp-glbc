package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/urfave/cli/v2"
	yaml "gopkg.in/yaml.v3"
)

func main() {
	app := &cli.App{
		Name: "argocd-glbc-plugin",
		Commands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "argocd-glbc-plugin generate <path>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "url",
						Usage:    "the transform endpoint url",
						Aliases:  []string{"u"},
						Required: true,
					},
					&cli.StringFlag{
						Name:     "resolve",
						Usage:    "force the host to resolve to the specified address (format: 'host:port')",
						Aliases:  []string{"r"},
						Value:    "",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "path",
						Usage:    "the path where the context is",
						Aliases:  []string{"p"},
						Value:    ".",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "token",
						Usage:    "the argocd token",
						Aliases:  []string{"t"},
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					path := cCtx.String("path")
					url := cCtx.String("url")
					resolve := cCtx.String("resolve")

					// TODO: Sanity check path is not trying to break outside current dir

					err := generate(path, url, resolve)
					if err != nil {
						log.Fatal(err)
					}
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generate(path, url, resolve string) error {

	err := filepath.Walk(path,
		func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fileExtension := filepath.Ext(file)
			if fileExtension == ".yaml" || fileExtension == ".yml" {

				resources, err := os.ReadFile(file)
				if err != nil {
					log.Fatal(err)
				}

				dec := yaml.NewDecoder(bytes.NewReader(resources))

				// a file can contain several yaml documents
				for {
					var value map[string]interface{}
					err := dec.Decode(&value)
					if err == io.EOF {
						break
					}
					if err != nil {
						return err
					}

					// we are only interested in Ingress and Route resources
					// avoid unmarshalling anything else
					if kind, ok := value["kind"]; !ok {
						// should we skip instead??
						return fmt.Errorf("found document without kind")
					} else {

						if kind == "Ingress" || kind == "Route" {
							// Send the resource to the glbc transform endpoint
							client := resty.NewWithClient(client(resolve))
							resp, err := client.R().SetBody(value).Post(url)
							if err != nil {
								return err
							}
							if !resp.IsSuccess() {
								return fmt.Errorf("transform endpoint returned '%s'", resp.Status())
							}
							// print the transformed resource
							fmt.Println("---")
							fmt.Println(string(resp.Body()))
						} else {
							// print the resource as-is
							fmt.Println("---")
							out, err := yaml.Marshal(value)
							if err != nil {
								return fmt.Errorf("error serializing to json: '%s'", err)
							}
							fmt.Println(string(out))
						}
					}
				}
			}

			// TODO: Fetch liveState from ArgoCD ManagedResources API, parsing out any Traffic Objects

			// TODO: Fetch Cluster information from ArgoCD API

			// TODO: Fetch any Applications that are part of the same multi-cluster deployment

			// TODO: Pass Cluster information, multi-cluster Applications, and Traffic Objects targetState and liveState to GLBC transform endpoint

			// TODO: Output transformed version of resources

			return nil
		})

	return err
}

// This is a temporary solution to implement a --resolve option for the plugin
// like curl's. This is required because the glbc ingress that exposes the transform
// endpoint doesn't resolve in the local setup. This is only necesary while glbc runs
// within kcp.
func client(resolve string) *http.Client {

	dialer := &net.Dialer{Timeout: 2 * time.Second, KeepAlive: 2 * time.Second}

	tr := &http.Transport{
		Dial: dialer.Dial,
	}

	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if resolve != "" {
			addr = resolve
		}
		return dialer.DialContext(ctx, network, addr)
	}

	client := http.Client{Transport: tr}
	return &client
}
