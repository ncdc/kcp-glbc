package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

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
							if fileExtension == ".yaml" {
								// TODO: Parse out any Traffic Objects (Ingress/Route)
								dat, err := os.ReadFile(file)
								if err != nil {
									log.Fatal(err)
								}
								fmt.Println("---")
								fmt.Println(string(dat))
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
