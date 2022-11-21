package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/go-resty/resty/v2"
	"github.com/urfave/cli/v2"
	yaml "gopkg.in/yaml.v3"
)

const TransformEndpoint = "http://kcp-glbc-transform.kcp-glbc.svc.cluster.local/transform"

func main() {
	app := &cli.App{
		Name: "argocd-glbc-plugin",
		Commands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "argocd-glbc-plugin generate <path>",
				Action: func(cCtx *cli.Context) error {
					path := cCtx.Args().First()
					if len(path) < 1 {
						return cli.Exit("Must specify a path", 1)
					}
					// TODO: Sanity check path is not trying to break outside current dir
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
											client := resty.New()
											resp, err := client.R().SetBody(value).Post(TransformEndpoint)
											if err != nil {
												return err
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
