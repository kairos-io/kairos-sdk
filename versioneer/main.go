package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kairos-io/kairos-sdk/versioneer/pkg/versioneer"
	"github.com/urfave/cli/v2"
)

var (
	flavorFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "flavor",
		Value: "",
		Usage: "the OS flavor (e.g. opensuse)",
	}

	flavorReleaseFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "flavor-release",
		Value: "",
		Usage: "the OS flavor release (e.g. leap-15.5)",
	}

	variantFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "variant",
		Value: "",
		Usage: "the Kairos variant (core, standard)",
	}

	modelFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "model",
		Value: "",
		Usage: "the model for which the OS was built (e.g. rpi4)",
	}

	archFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "arch",
		Value: "",
		Usage: "the architecture of the OS",
	}

	versionFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "version",
		Value: "",
		Usage: "the Kairos version (e.g. v2.4.2)",
	}

	softwareVersionFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "software-version",
		Value: "",
		Usage: "the software version (e.g. k3sv1.28.2+k3s1)",
	}

	registryAndOrgFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "registry-and-org",
		Value: "",
		Usage: "the container registry and org (e.g. \"quay.io/kairos\")",
	}
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:  "container-artifact-name",
				Usage: "generates an artifact name for Kairos OCI images",
				Flags: []cli.Flag{
					flavorFlag, flavorReleaseFlag, variantFlag, modelFlag, archFlag,
					versionFlag, softwareVersionFlag, registryAndOrgFlag,
				},
				Action: func(cCtx *cli.Context) error {
					a := versioneer.Artifact{
						Flavor:          cCtx.String(flavorFlag.Name),
						FlavorRelease:   cCtx.String(flavorReleaseFlag.Name),
						Variant:         cCtx.String(variantFlag.Name),
						Model:           cCtx.String(modelFlag.Name),
						Arch:            cCtx.String(archFlag.Name),
						Version:         cCtx.String(versionFlag.Name),
						SoftwareVersion: cCtx.String(softwareVersionFlag.Name),
					}

					result, err := a.ContainerName(cCtx.String(registryAndOrgFlag.Name))
					if err != nil {
						return err
					}
					fmt.Println(result)

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
